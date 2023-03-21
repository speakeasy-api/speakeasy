package usage

import (
	"fmt"
	"github.com/speakeasy-api/openapi-generation/v2/pkg/generate"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/utils"
	"github.com/spf13/cobra"
	"os"
)

func OutputUsage(cmd *cobra.Command, file, out string, debug bool) error {
	ctx := cmd.Context()

	l := log.Logger()

	fmt.Printf("Generating CSV for %s...\n", file)

	schema, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("failed to read schema file %s: %w", file, err)
	}

	opts := []generate.GeneratorOptions{
		generate.WithFileFuncs(func(outFileName string, data []byte, mode os.FileMode) error {
			err := utils.CreateDirectory(outFileName)
			if err != nil {
				return err
			}
			return os.WriteFile(outFileName, data, os.ModePerm)
		}, os.ReadFile),
		generate.WithLogger(l),
		generate.WithRunLocation("cli"),
	}

	g, err := generate.New(opts...)
	if err != nil {
		return err
	}

	if debug {
		opts = append(opts, generate.WithDebuggingEnabled())
	}

	if errs := g.GenerateCSV(ctx, schema, out); len(errs) > 0 {
		for _, err := range errs {
			l.Error(err.Error())
		}

		return fmt.Errorf("failed to generate CSV for %s ✖", file)
	}

	fmt.Printf("Generating CSV for %s... %s\n", file, utils.Green("done ✓"))
	return nil
}
