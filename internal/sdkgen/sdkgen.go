package sdkgen

import (
	"context"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/speakeasy-api/speakeasy/internal/schema"

	"github.com/fatih/color"
	changelog "github.com/speakeasy-api/openapi-generation/v2"
	"github.com/speakeasy-api/openapi-generation/v2/pkg/filetracking"
	"github.com/speakeasy-api/openapi-generation/v2/pkg/generate"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/utils"
	"go.uber.org/zap"
)

func Generate(ctx context.Context, customerID, workspaceID, lang, schemaPath, header, token, outDir, genVersion, installationURL string, debug, autoYes, published, outputTests bool, repo, repoSubDir string) error {
	if !generate.CheckLanguageSupported(lang) {
		return fmt.Errorf("language not supported: %s", lang)
	}

	fmt.Printf("Generating SDK for %s...\n", lang)

	if strings.TrimSpace(outDir) == "." {
		wd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current working directory: %w", err)
		}

		outDir = wd
	}

	isRemote, schema, err := schema.GetSchemaContents(schemaPath, header, token)
	if err != nil {
		return fmt.Errorf("failed to get schema contents: %w", err)
	}

	if err := filetracking.CleanDir(outDir, autoYes); err != nil {
		return fmt.Errorf("failed to clean out dir %s: %w", outDir, err)
	}

	l := log.NewLogger(schemaPath)

	runLocation := os.Getenv("SPEAKEASY_RUN_LOCATION")
	if runLocation == "" {
		runLocation = "cli"
	}

	opts := []generate.GeneratorOptions{
		generate.WithLogger(l),
		generate.WithCustomerID(customerID),
		generate.WithWorkspaceID(workspaceID),
		generate.WithFileFuncs(func(filename string, data []byte, perm os.FileMode) error {
			dir := path.Dir(filename)

			if _, err := os.Stat(dir); os.IsNotExist(err) {
				err := os.MkdirAll(dir, 0o755)
				if err != nil {
					return err
				}
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
		return err
	}

	if errs := g.Generate(context.Background(), schema, schemaPath, lang, outDir, isRemote); len(errs) > 0 {
		for _, err := range errs {
			l.Error("", zap.Error(err))
		}

		return fmt.Errorf("failed to generate SDKs for %s ✖", lang)
	}

	docs := color.New(color.FgGreen, color.Underline)

	sdkDocsLink := "https://www.speakeasyapi.dev/docs/customize-sdks"

	fmt.Printf("Generating SDK for %s... %s.\n", lang, utils.Green("done ✓"))
	docs.Printf("For more docs on customising the SDK check out: %s\n", sdkDocsLink)

	return nil
}

func ValidateConfig(ctx context.Context, outDir string) error {
	l := log.NewLogger("gen.yaml") // TODO if we want to associate annotations with this file we need to get the actual path

	opts := []generate.GeneratorOptions{
		generate.WithLogger(l),
		generate.WithFileFuncs(func(filename string, data []byte, perm os.FileMode) error {
			dir := path.Dir(filename)

			if _, err := os.Stat(dir); os.IsNotExist(err) {
				err := os.MkdirAll(dir, 0o755)
				if err != nil {
					return err
				}
			}

			// TODO Using 0755 here rather than perm is temporary until an upstream change to
			// easytemplate can be made to add better support for file permissions.
			return os.WriteFile(filename, data, 0o755)
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
