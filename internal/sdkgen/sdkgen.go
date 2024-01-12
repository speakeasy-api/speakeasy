package sdkgen

import (
	"context"
	"fmt"
	"github.com/speakeasy-api/speakeasy/internal/styles"
	"os"
	"strings"

	"github.com/speakeasy-api/speakeasy/internal/schema"

	changelog "github.com/speakeasy-api/openapi-generation/v2"
	"github.com/speakeasy-api/openapi-generation/v2/pkg/generate"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/utils"
	"go.uber.org/zap"
)

func Generate(ctx context.Context, customerID, workspaceID, lang, schemaPath, header, token, outDir, genVersion, installationURL string, debug, autoYes, published, outputTests bool, repo, repoSubDir string, compile bool) error {
	if !generate.CheckLanguageSupported(lang) {
		return fmt.Errorf("language not supported: %s", lang)
	}

	logger := log.From(ctx).WithAssociatedFile(schemaPath)

	logger.Infof("Generating SDK for %s...\n", lang)

	if strings.TrimSpace(outDir) == "." {
		wd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current working directory: %w", err)
		}

		outDir = wd
	}

	isRemote, schema, err := schema.GetSchemaContents(ctx, schemaPath, header, token)
	if err != nil {
		return fmt.Errorf("failed to get schema contents: %w", err)
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
		generate.WithCleanDir(),
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
		return err
	}

	if errs := g.Generate(context.Background(), schema, schemaPath, lang, outDir, isRemote, compile); len(errs) > 0 {
		for _, err := range errs {
			logger.Error("", zap.Error(err))
		}

		return fmt.Errorf("failed to generate SDKs for %s ✖", lang)
	}

	sdkDocsLink := "https://www.speakeasyapi.dev/docs/customize-sdks"

	logger.Successf("\nSDK for %s generated successfully ✓", lang)
	logger.WithStyle(styles.HeavilyEmphasized).Printf("For docs on customising the SDK check out: %s", sdkDocsLink)

	return nil
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

	if _, err := g.LoadConfig(outDir, true); err != nil {
		return err
	}

	return nil
}
