package docsgen

import (
	"context"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/speakeasy-api/speakeasy/internal/schema"

	changelog "github.com/speakeasy-api/openapi-generation/v2"
	"github.com/speakeasy-api/openapi-generation/v2/pkg/filetracking"
	"github.com/speakeasy-api/openapi-generation/v2/pkg/generate"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/utils"
	"go.uber.org/zap"
)

var supportSDKDocsLanguages map[string]bool = map[string]bool{
	"go":         true,
	"python":     true,
	"typescript": true,
	"csharp":     true,
	"unity":      true,
}

func GenerateContent(ctx context.Context, langs []string, customerID, schemaPath, header, token, outDir, repo, repoSubDir string, debug, autoYes, compile bool) error {
	for _, lang := range langs {
		if _, ok := supportSDKDocsLanguages[lang]; !ok {
			return fmt.Errorf("language %s is not supported in SDK docs", lang)
		}
	}

	fmt.Printf("Generating SDK Docs for langs %s...\n", strings.Join(langs, ", "))

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

	opts := []generate.GeneratorOptions{
		generate.WithLogger(l),
		generate.WithCustomerID(customerID),
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
		generate.WithGenVersion(strings.TrimPrefix(changelog.GetLatestVersion(), "v")),
		generate.WithRepoDetails(repo, repoSubDir),
		generate.WithAllowRemoteReferences(),
		// We will add optional curl support when it is complete.
		generate.WithSDKDocLanguages(false, langs...),
	}

	if compile {
		opts = append(opts, generate.WithSinglePageWrapping())
	}

	if debug {
		opts = append(opts, generate.WithDebuggingEnabled())
	}

	g, err := generate.New(opts...)
	if err != nil {
		return err
	}

	if errs := g.Generate(context.Background(), schema, schemaPath, "docs", outDir, isRemote); len(errs) > 0 {
		for _, err := range errs {
			l.Error("", zap.Error(err))
		}

		return fmt.Errorf("failed to generate SDKs Docs for %s ✖", strings.Join(langs, ", "))
	}

	fmt.Printf("Generated SDK for %s... %s\n", strings.Join(langs, ", "), utils.Green("done ✓"))

	return nil
}
