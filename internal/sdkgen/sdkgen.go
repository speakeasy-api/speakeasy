package sdkgen

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	gen_config "github.com/speakeasy-api/sdk-gen-config"
	"github.com/speakeasy-api/speakeasy-client-sdk-go/v3/pkg/models/shared"
	"github.com/speakeasy-api/speakeasy-core/access"
	"github.com/speakeasy-api/speakeasy-core/events"
	"github.com/speakeasy-api/speakeasy/internal/charm/styles"
	"github.com/speakeasy-api/speakeasy/internal/schema"

	changelog "github.com/speakeasy-api/openapi-generation/v2"
	"github.com/speakeasy-api/openapi-generation/v2/pkg/generate"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/utils"
	"go.uber.org/zap"
)

type GenerationAccess struct {
	AccessAllowed bool
	Message       string
}

func Generate(ctx context.Context, customerID, workspaceID, lang, schemaPath, header, token, outDir, genVersion, installationURL string, debug, autoYes, published, outputTests bool, repo, repoSubDir string, compile bool) (*GenerationAccess, error) {
	if !generate.CheckLanguageSupported(lang) {
		return nil, fmt.Errorf("language not supported: %s", lang)
	}

	ctx = events.SetTargetInContext(ctx, outDir)

	generationAccess, message, _ := access.HasGenerationAccess(ctx, &access.GenerationAccessArgs{
		GenLockID:  getGenLockID(outDir),
		TargetType: &lang,
	})

	logger := log.From(ctx).WithAssociatedFile(schemaPath)

	logger.Infof("Generating SDK for %s...\n", lang)

	if strings.TrimSpace(outDir) == "." {
		wd, err := os.Getwd()
		if err != nil {
			return &GenerationAccess{
				AccessAllowed: generationAccess,
				Message:       message,
			}, fmt.Errorf("failed to get current working directory: %w", err)
		}

		outDir = wd
	}

	isRemote, schema, err := schema.GetSchemaContents(ctx, schemaPath, header, token)
	if err != nil {
		return &GenerationAccess{
			AccessAllowed: generationAccess,
			Message:       message,
		}, fmt.Errorf("failed to get schema contents: %w", err)
	}

	runLocation := os.Getenv("SPEAKEASY_RUN_LOCATION")
	if runLocation == "" {
		runLocation = "cli"
	}

	opts := []generate.GeneratorOptions{
		generate.WithLogger(logger.WithFormatter(log.PrefixedFormatter)),
		generate.WithCustomerID(customerID),
		generate.WithWorkspaceID(workspaceID),
		generate.WithFileFuncs(func(filename string, data []byte, perm os.FileMode) error {
			if err := utils.CreateDirectory(filename); err != nil {
				return err
			}

			return os.WriteFile(filename, data, perm)
		}, os.ReadFile),
		generate.WithRunLocation(runLocation),
		generate.WithGenVersion(strings.TrimPrefix(changelog.GetLatestVersion(), "v")),
		generate.WithInstallationURL(installationURL),
		generate.WithPublished(published),
		generate.WithRepoDetails(repo, repoSubDir),
		generate.WithAllowRemoteReferences(),
	}

	if debug {
		opts = append(opts, generate.WithDebuggingEnabled())
	}

	// Enable outputting of internal tests for internal speakeasy use cases
	if outputTests {
		opts = append(opts, generate.WithOutputTests())
	}

	g, err := generate.New(opts...)
	if err != nil {
		return &GenerationAccess{
			AccessAllowed: generationAccess,
			Message:       message,
		}, err
	}

	err = events.Telemetry(ctx, shared.InteractionTypeTargetGenerate, func(ctx context.Context, event *shared.CliEvent) error {
		if errs := g.Generate(ctx, schema, schemaPath, lang, outDir, isRemote, compile); len(errs) > 0 {
			for _, err := range errs {
				logger.Error("", zap.Error(err))
			}

			return fmt.Errorf("failed to generate SDKs for %s ✖", lang)
		}
		return nil
	})
	if err != nil {
		return &GenerationAccess{
			AccessAllowed: generationAccess,
			Message:       message,
		}, err
	}

	sdkDocsLink := "https://www.speakeasyapi.dev/docs/customize-sdks"

	logger.Successf("\nSDK for %s generated successfully ✓", lang)
	logger.WithStyle(styles.HeavilyEmphasized).Printf("For docs on customising the SDK check out: %s", sdkDocsLink)

	if !generationAccess {
		warningDate := time.Date(2024, time.March, 18, 0, 0, 0, 0, time.UTC)
		daysToLimit := int(math.Round(warningDate.Sub(time.Now().Truncate(24*time.Hour)).Hours() / 24))
		msg := styles.RenderInfoMessage(
			"🚀 Time to Upgrade 🚀\n",
			message,
			fmt.Sprintf("Please reach out to the Speakeasy team in the next %d days to ensure continued access.", daysToLimit),
			"\nhttps://calendly.com/d/5dm-wvm-2mx/chat-with-speakeasy-team",
		)
		logger.Println("\n\n" + msg)
	}

	return &GenerationAccess{
		AccessAllowed: generationAccess,
		Message:       message,
	}, nil
}

func ValidateConfig(ctx context.Context, outDir string) error {
	l := log.From(ctx).WithAssociatedFile("gen.yaml") // TODO if we want to associate annotations with this file we need to get the actual path

	opts := []generate.GeneratorOptions{
		generate.WithLogger(l),
		generate.WithFileFuncs(func(filename string, data []byte, perm os.FileMode) error {
			if err := utils.CreateDirectory(filename); err != nil {
				return err
			}

			return os.WriteFile(filename, data, perm)
		}, os.ReadFile),
		generate.WithRunLocation("cli"),
	}

	g, err := generate.New(opts...)
	if err != nil {
		return err
	}

	if _, err := g.LoadConfig(ctx, outDir, true); err != nil {
		return err
	}

	return nil
}

func getGenLockID(outDir string) *string {
	if utils.FileExists(filepath.Join(utils.SanitizeFilePath(outDir), ".speakeasy/gen.lock")) || utils.FileExists(filepath.Join(utils.SanitizeFilePath(outDir), ".gen/gen.lock")) {
		if cfg, err := gen_config.Load(outDir); err == nil && cfg.LockFile != nil {
			return &cfg.LockFile.ID
		}
	}

	return nil
}
