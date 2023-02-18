package sdkgen

import (
	"context"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/speakeasy-api/openapi-generation/v2/pkg/filetracking"
	"github.com/speakeasy-api/openapi-generation/v2/pkg/generate"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/utils"
)

func Generate(ctx context.Context, customerID, lang, schemaPath, outDir, genVersion string, debug bool, autoYes bool) error {
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

	schema, err := os.ReadFile(schemaPath)
	if err != nil {
		return fmt.Errorf("failed to read schema file %s: %w", schemaPath, err)
	}

	if err := filetracking.CleanDir(outDir, autoYes); err != nil {
		return fmt.Errorf("failed to clean out dir %s: %w", outDir, err)
	}

	l := log.Logger()

	opts := []generate.GeneratorOptions{
		generate.WithLogger(l),
		generate.WithCustomerID(customerID),
		generate.WithFileFuncs(func(filename string, data []byte, perm os.FileMode) error {
			dir := path.Dir(filename)

			if _, err := os.Stat(dir); os.IsNotExist(err) {
				err := os.MkdirAll(dir, os.ModePerm)
				if err != nil {
					return err
				}
			}

			return os.WriteFile(filename, data, os.ModePerm)
		}, func() func(filename string) ([]byte, error) {
			return func(filename string) ([]byte, error) {
				filePath := path.Join(outDir, filename)

				if _, err := os.Stat(filePath); err != nil {
					return nil, err
				}

				return os.ReadFile(filePath)
			}
		}()),
		generate.WithRunLocation("cli"),
		generate.WithGenVersion(genVersion),
	}

	if debug {
		opts = append(opts, generate.WithDebuggingEnabled())
	}

	g, err := generate.New(opts...)
	if err != nil {
		return err
	}

	if errs := g.Generate(context.Background(), schema, lang, outDir); len(errs) > 0 {
		for _, err := range errs {
			l.Error(err.Error())
		}

		return fmt.Errorf("Failed to generate SDKs for %s ✖", lang)
	}

	fmt.Printf("Generating SDK for %s... %s\n", lang, utils.Green("done ✓"))

	return nil
}

func ValidateConfig(ctx context.Context, outDir string) error {
	l := log.Logger()

	opts := []generate.GeneratorOptions{
		generate.WithLogger(l),
		generate.WithFileFuncs(func(filename string, data []byte, perm os.FileMode) error {
			dir := path.Dir(filename)

			if _, err := os.Stat(dir); os.IsNotExist(err) {
				err := os.MkdirAll(dir, os.ModePerm)
				if err != nil {
					return err
				}
			}

			return os.WriteFile(filename, data, os.ModePerm)
		}, func() func(filename string) ([]byte, error) {
			return func(filename string) ([]byte, error) {
				filePath := path.Join(outDir, filename)

				if _, err := os.Stat(filePath); err != nil {
					return nil, err
				}

				return os.ReadFile(filePath)
			}
		}()),
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
