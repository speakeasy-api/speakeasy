package sdkgen

import (
	"context"
	stderrors "errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/charmbracelet/lipgloss"
	"github.com/speakeasy-api/speakeasy-core/auth"
	"github.com/speakeasy-api/speakeasy-core/openapi"

	config "github.com/speakeasy-api/sdk-gen-config"
	"github.com/speakeasy-api/speakeasy-client-sdk-go/v3/pkg/models/shared"
	"github.com/speakeasy-api/speakeasy-core/access"
	"github.com/speakeasy-api/speakeasy-core/events"
	"github.com/speakeasy-api/speakeasy/internal/charm/styles"
	"github.com/speakeasy-api/speakeasy/internal/env"
	"github.com/speakeasy-api/speakeasy/internal/fs"
	"github.com/speakeasy-api/speakeasy/internal/git"
	"github.com/speakeasy-api/speakeasy/internal/patches"
	"github.com/speakeasy-api/speakeasy/internal/workflowTracking"

	changelog "github.com/speakeasy-api/openapi-generation/v2"
	"github.com/speakeasy-api/openapi-generation/v2/pkg/generate"
	"github.com/speakeasy-api/openapi-generation/v2/pkg/merge"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/utils"
	"github.com/speakeasy-api/speakeasy/prompts"
	"go.uber.org/zap"
)

// PromptForCustomCode is a function variable that can be replaced in tests.
// It defaults to the real prompt implementation.
var PromptForCustomCode = prompts.PromptForCustomCode

type GenerationAccess struct {
	AccessAllowed bool
	Message       string
	Level         *shared.Level
}

type CancellableGeneration struct {
	CancellationMutex  sync.Mutex         // protects both CancellableContext and CancelGeneration (exposed by w.CancelGeneration())
	CancellableContext context.Context    //nolint:containedctx // Intentional: enables cancellation of long-running generation
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
	AllowPrompts    bool

	CancellableGeneration *CancellableGeneration
	StreamableGeneration  *StreamableGeneration
	ReleaseNotes          string

	// WorkflowStep enables running prompts through the workflow visualizer.
	// If nil, prompts will run directly without visualizer integration.
	WorkflowStep *workflowTracking.WorkflowStep
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
		}, stderrors.New("generation access blocked")
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
		generate.WithChangelogReleaseNotes(opts.ReleaseNotes),
	}

	if opts.Verbose {
		generatorOpts = append(generatorOpts, generate.WithVerboseOutput(true))
	}

	if opts.Debug {
		generatorOpts = append(generatorOpts, generate.WithDebuggingEnabled())
	}

	// Enable outputting of internal tests for internal speakeasy use cases
	if opts.OutputTests {
		generatorOpts = append(generatorOpts, generate.WithOutputTests())
	}
	if opts.SkipVersioning {
		generatorOpts = append(generatorOpts, generate.WithSkipVersioning(opts.SkipVersioning))
	}

	// Track the current generation step for error reporting
	failedStepMessage := ""
	trackProgress := func(update generate.ProgressUpdate) {
		if update.Step != nil {
			failedStepMessage = update.Step.Message
		}
		// Forward to user-provided callback if present
		if opts.StreamableGeneration != nil && opts.StreamableGeneration.OnProgressUpdate != nil {
			opts.StreamableGeneration.OnProgressUpdate(update)
		}
	}

	// Enable step tracking by default for error reporting
	genSteps := true
	fileStatus := false
	if opts.StreamableGeneration != nil {
		genSteps = opts.StreamableGeneration.GenSteps || true
		fileStatus = opts.StreamableGeneration.FileStatus
	}
	generatorOpts = append(
		generatorOpts,
		generate.WithProgressUpdates(
			opts.TargetName,
			trackProgress,
			genSteps,
			fileStatus,
		),
	)

	// Try to open a git repository for the Round-Trip Engineering (3-way merge) feature.
	// If a git repository exists, inject Git and FileSystem adapters for the persistentEdits feature.
	repo, repoErr := git.NewLocalRepository(opts.OutDir)
	if repoErr == nil && !repo.IsNil() {
		wrappedRepo := patches.WrapGitRepository(repo)

		// Use opts.OutDir as baseDir for GitAdapter. This allows translation between
		// generation-relative paths (e.g., "sdk.go") and git-root-relative paths (e.g., "go-sdk/sdk.go").
		// opts.OutDir is relative to cwd which should be at or inside the git root.
		gitAdapter := patches.NewGitAdapter(wrappedRepo, opts.OutDir)

		generatorOpts = append(generatorOpts,
			generate.WithGit(gitAdapter),
			generate.WithFileSystem(fs.NewFileSystem(opts.OutDir)),
		)

		// Pre-generation: detect file moves/deletions, prompt if needed, and update lockfile
		// Use WorkflowStep for prompts if available (to pause visualizer), otherwise use direct prompts
		// Pass nil promptFunc if prompts are not allowed (e.g., console output mode, CI)
		var promptFunc patches.PromptFunc
		if opts.AllowPrompts {
			promptFunc = PromptForCustomCode
			if opts.WorkflowStep != nil {
				promptFunc = func(summary string) (prompts.CustomCodeChoice, error) {
					return prompts.PromptForCustomCodeWithStep(summary, opts.WorkflowStep)
				}
			}
		}
		if err := patches.PrepareForGeneration(opts.OutDir, opts.AutoYes, promptFunc, logger.Warnf); err != nil {
			logger.Warnf("Error preparing for generation: %v", err)
		}
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
				return fmt.Errorf("generation was aborted for %s ✖", opts.Language)
			}
		} else {
			errs = g.Generate(ctx, schema, opts.SchemaPath, opts.Language, opts.OutDir, isRemote, opts.Compile)
		}

		if len(errs) > 0 {
			for _, err := range errs {
				// Check if it's a ConflictsError - render pretty conflict message
				var conflictErr *merge.ConflictsError
				if stderrors.As(err, &conflictErr) {
					renderConflictsError(logger, conflictErr)
					// Don't log the generic error for conflicts - we rendered a nice message
					continue
				}
				logger.Error("", zap.Error(err))
			}

			if failedStepMessage != "" {
				return fmt.Errorf("generation failed for %q during step %q", opts.Language, failedStepMessage)
			}

			return fmt.Errorf("failed to generate %q", opts.Language)
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

	cliEvent := events.GetTelemetryEventFromContext(ctx)
	if cliEvent != nil && cliEvent.ExecutionID != "" {
		// Get org and workspace slugs from context
		orgSlug := auth.GetOrgSlugFromContext(ctx)
		workspaceSlug := auth.GetWorkspaceSlugFromContext(ctx)

		if orgSlug != "" && workspaceSlug != "" {
			logger.Successf("speakeasy repro %s_%s_%s", orgSlug, workspaceSlug, cliEvent.ExecutionID)
		}
	}

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
		if cfg, err := config.Load(outDir); err == nil && cfg.LockFile != nil {
			return &cfg.LockFile.ID
		}
	}

	return nil
}

// renderConflictsError renders a git-status style error message for merge conflicts.
func renderConflictsError(logger log.Logger, conflictErr *merge.ConflictsError) {
	// Build file list with "both modified:" prefix like git status
	var fileLines strings.Builder
	for _, file := range conflictErr.Files {
		fileLines.WriteString(fmt.Sprintf("    both modified:   %s\n", file))
	}

	// Render instructional error box
	msg := styles.RenderInstructionalError(
		"Merge Conflicts Detected",
		fmt.Sprintf("%d file(s) have conflicts that must be resolved manually:\n%s", len(conflictErr.Files), fileLines.String()),
		"To resolve:\n"+
			"1. Open each file and resolve the conflict markers (<<<<<<, ======, >>>>>>)\n"+
			"2. Remove the conflict markers after choosing the correct code\n"+
			"3. Run: speakeasy run --skip-versioning",
	)
	logger.Println("\n" + msg)

	// Emit GitHub Actions annotations for each conflicting file
	for _, file := range conflictErr.Files {
		logger.Printf("::error file=%s::Merge conflict detected - manual resolution required", file)
	}
}
