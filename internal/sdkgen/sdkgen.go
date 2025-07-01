package sdkgen

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/charmbracelet/lipgloss"
	"github.com/speakeasy-api/speakeasy-core/auth"
	"github.com/speakeasy-api/speakeasy-core/openapi"

	config "github.com/speakeasy-api/sdk-gen-config"
	gen_config "github.com/speakeasy-api/sdk-gen-config"
	"github.com/speakeasy-api/speakeasy-client-sdk-go/v3/pkg/models/shared"
	"github.com/speakeasy-api/speakeasy-core/access"
	"github.com/speakeasy-api/speakeasy-core/events"
	"github.com/speakeasy-api/speakeasy/internal/charm/styles"
	"github.com/speakeasy-api/speakeasy/internal/env"

	changelog "github.com/speakeasy-api/openapi-generation/v2"
	"github.com/speakeasy-api/openapi-generation/v2/pkg/generate"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/utils"
	"go.uber.org/zap"
)

type GenerationAccess struct {
	AccessAllowed bool
	Message       string
	Level         *shared.Level
}

type CancellableGeneration struct {
	CancellationMutex  sync.Mutex         // protects both CancellableContext and CancelGeneration (exposed by w.CancelGeneration())
	CancellableContext context.Context    // the context that can be cancelled to stop generation
	CancelGeneration   context.CancelFunc // the function to call to cancel generation
}

type StreamableGeneration struct {
	OnProgressUpdate func(generate.ProgressUpdate) // the callback function called on each progress update
	GenSteps         bool                          // whether to receive an update before each generation step starts
	FileStatus       bool                          // whether to receive updates on each file status change
	LogListener      chan log.Msg                  // the channel to listen for log messages (Debug only)
}

type GenerateOptions struct {
	CustomerID      string
	WorkspaceID     string
	Language        string
	SchemaPath      string
	Header          string
	Token           string
	OutDir          string
	CLIVersion      string
	InstallationURL string
	Debug           bool
	AutoYes         bool
	Published       bool
	OutputTests     bool
	Repo            string
	RepoSubDir      string
	Verbose         bool
	Compile         bool
	TargetName      string
	SkipVersioning  bool

	CancellableGeneration *CancellableGeneration
	StreamableGeneration  *StreamableGeneration
}

func Generate(ctx context.Context, opts GenerateOptions) (*GenerationAccess, error) {
	if !generate.CheckTargetNameSupported(opts.Language) {
		return nil, fmt.Errorf("language not supported: %s", opts.Language)
	}

	ctx = events.SetTargetInContext(ctx, opts.OutDir)

	logger := log.From(ctx).WithAssociatedFile(opts.SchemaPath)

	generationAccess, level, message, _ := access.HasGenerationAccess(ctx, &access.GenerationAccessArgs{
		GenLockID:  GetGenLockID(opts.OutDir),
		TargetType: &opts.Language,
	})

	if !generationAccess && level != nil && *level == shared.LevelBlocked {
		msg := styles.RenderErrorMessage(
			"Upgrade Required\n",
			lipgloss.Center,
			strings.Split(message, "\n")...,
		)
		logger.Println("\n\n" + msg)
		return &GenerationAccess{
			AccessAllowed: generationAccess,
			Message:       message,
			Level:         level,
		}, errors.New("generation access blocked")
	}

	logger.Infof("Generating SDK for %s...\n", opts.Language)

	if strings.TrimSpace(opts.OutDir) == "." {
		wd, err := os.Getwd()
		if err != nil {
			return &GenerationAccess{
				AccessAllowed: generationAccess,
				Message:       message,
				Level:         level,
			}, fmt.Errorf("failed to get current working directory: %w", err)
		}

		opts.OutDir = wd
	}

	isRemote, schema, err := openapi.GetSchemaContents(ctx, opts.SchemaPath, opts.Header, opts.Token)
	if err != nil {
		return &GenerationAccess{
			AccessAllowed: generationAccess,
			Message:       message,
			Level:         level,
		}, fmt.Errorf("failed to get schema contents: %w", err)
	}

	runLocation := env.SpeakeasyRunLocation()
	if runLocation == "" {
		runLocation = "cli"
	}

	workspaceUri := auth.GetWorkspaceBaseURL(ctx)

	generatorOpts := []generate.GeneratorOptions{
		generate.WithLogger(logger.WithFormatter(log.PrefixedFormatter)),
		generate.WithCustomerID(opts.CustomerID),
		generate.WithWorkspaceID(opts.WorkspaceID),
		// We need the workspace uri in the generator to render a link to the
		// workspace onboarding steps in the readme when it is not yet setup fully
		generate.WithWorkspaceUri(workspaceUri),
		generate.WithRunLocation(runLocation),
		generate.WithGenVersion(strings.TrimPrefix(changelog.GetLatestVersion(), "v")),
		generate.WithInstallationURL(opts.InstallationURL),
		generate.WithPublished(opts.Published),
		generate.WithRepoDetails(opts.Repo, opts.RepoSubDir),
		generate.WithCLIVersion(opts.CLIVersion),
		generate.WithForceGeneration(),
	}

	if opts.Verbose {
		generatorOpts = append(generatorOpts, generate.WithVerboseOutput(true))
	}

	// Always enable debug file writing to help with troubleshooting format errors
	generatorOpts = append(generatorOpts, generate.WithDebuggingEnabled())

	// Enable outputting of internal tests for internal speakeasy use cases
	if opts.OutputTests {
		generatorOpts = append(generatorOpts, generate.WithOutputTests())
	}
	if opts.SkipVersioning {
		generatorOpts = append(generatorOpts, generate.WithSkipVersioning(opts.SkipVersioning))
	}

	if opts.StreamableGeneration != nil {
		generatorOpts = append(
			generatorOpts,
			generate.WithProgressUpdates(
				opts.TargetName,
				opts.StreamableGeneration.OnProgressUpdate,
				opts.StreamableGeneration.GenSteps,
				opts.StreamableGeneration.FileStatus,
			),
		)
	}

	g, err := generate.New(generatorOpts...)
	if err != nil {
		return &GenerationAccess{
			AccessAllowed: generationAccess,
			Message:       message,
			Level:         level,
		}, err
	}

	err = events.Telemetry(ctx, shared.InteractionTypeTargetGenerate, func(ctx context.Context, event *shared.CliEvent) error {
		event.GenerateTargetName = &opts.TargetName

		var errs []error
		if opts.CancellableGeneration != nil && opts.CancellableGeneration.CancellableContext != nil {
			cancelCtx := opts.CancellableGeneration.CancellableContext

			var cancelled bool
			cancelled, errs = g.GenerateWithCancel(cancelCtx, schema, opts.SchemaPath, opts.Language, opts.OutDir, isRemote, opts.Compile)
			if cancelled {
				return fmt.Errorf("Generation was aborted for %s ✖", opts.Language)
			}
		} else {
			errs = g.Generate(ctx, schema, opts.SchemaPath, opts.Language, opts.OutDir, isRemote, opts.Compile)
		}

		if len(errs) > 0 {
			for _, err := range errs {
				logger.Error("", zap.Error(err))
			}

			return fmt.Errorf("failed to generate SDKs for %s ✖", opts.Language)
		}

		return nil
	})
	if err != nil {
		return &GenerationAccess{
			AccessAllowed: generationAccess,
			Message:       message,
			Level:         level,
		}, err
	}

	sdkDocsLink := "https://www.speakeasy.com/docs/customize-sdks"

	logger.Successf("\nSDK for %s generated successfully ✓", opts.Language)
	logger.WithStyle(styles.HeavilyEmphasized).Printf("For docs on customising the SDK check out: %s", sdkDocsLink)

	if !generationAccess {
		msg := styles.RenderInfoMessage(
			"🚀 Time to Upgrade 🚀\n",
			strings.Split(message, "\n")...,
		)
		logger.Println("\n\n" + msg)
	}

	return &GenerationAccess{
		AccessAllowed: generationAccess,
		Message:       message,
	}, nil
}

func ValidateConfig(ctx context.Context, outDir string) error {
	path := "gen.yaml"

	res, err := config.FindConfigFile(outDir, nil)
	if err == nil {
		path = res.Path
	}

	l := log.From(ctx).WithAssociatedFile(path)

	opts := []generate.GeneratorOptions{
		generate.WithLogger(l),
		generate.WithDontWrite(),
		generate.WithRunLocation("cli"),
	}

	g, err := generate.New(opts...)
	if err != nil {
		return err
	}

	if _, err := g.LoadConfig(ctx, outDir, generate.GetSupportedTargetNames()...); err != nil {
		return err
	}

	return nil
}

func GetGenLockID(outDir string) *string {
	if utils.FileExists(filepath.Join(utils.SanitizeFilePath(outDir), ".speakeasy/gen.lock")) || utils.FileExists(filepath.Join(utils.SanitizeFilePath(outDir), ".gen/gen.lock")) {
		if cfg, err := gen_config.Load(outDir); err == nil && cfg.LockFile != nil {
			return &cfg.LockFile.ID
		}
	}

	return nil
}
