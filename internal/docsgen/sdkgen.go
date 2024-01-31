package docsgen

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/speakeasy-api/speakeasy/internal/schema"

	changelog "github.com/speakeasy-api/openapi-generation/v2"
	"github.com/speakeasy-api/openapi-generation/v2/pkg/generate"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/utils"
	"go.uber.org/zap"
)

var SupportSDKDocsLanguages map[string]bool = map[string]bool{
	"go":         true,
	"python":     true,
	"typescript": true,
	"csharp":     true,
	"unity":      true,
	"java":       true,
	"curl":       true,
}

func GenerateContent(ctx context.Context, inputLangs []string, customerID, schemaPath, header, token, outDir, repo, repoSubDir string, debug, autoYes, compile bool) error {
	var langs []string
	hasCurl := false
	for _, lang := range inputLangs {
		if lang == "curl" {
			hasCurl = true
		} else {
			if _, ok := SupportSDKDocsLanguages[lang]; !ok {
				return fmt.Errorf("language %s is not supported in SDK docs", lang)
			}

			langs = append(langs, lang)
		}
	}

	logger := log.From(ctx)

	logger.Infof("Generating SDK Docs for langs %s...\n", strings.Join(langs, ", "))

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

	logger = logger.WithAssociatedFile(schemaPath)

	if hasCurl {
		langs = append(langs, "curl")
	}

	opts := []generate.GeneratorOptions{
		generate.WithLogger(logger),
		generate.WithCustomerID(customerID),
		generate.WithFileFuncs(func(filename string, data []byte, perm os.FileMode) error {
			if err := utils.CreateDirectory(filename); err != nil {
				return err
			}

			return os.WriteFile(filename, data, perm)
		}, os.ReadFile),
		generate.WithRunLocation("cli"),
		generate.WithGenVersion(strings.TrimPrefix(changelog.GetLatestVersion(), "v")),
		generate.WithRepoDetails(repo, repoSubDir),
		generate.WithAllowRemoteReferences(),
		generate.WithSDKDocLanguages(langs...),
		generate.WithCleanDir(),
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

	if errs := g.Generate(context.Background(), schema, schemaPath, "docs", outDir, isRemote, false); len(errs) > 0 {
		for _, err := range errs {
			logger.Error("", zap.Error(err))
		}

		return fmt.Errorf("failed to generate SDKs Docs for %s ✖", strings.Join(langs, ", "))
	}

	logger.Successf("Generated SDK(s) for %s... ✓", strings.Join(langs, ", "))

	return nil
}
