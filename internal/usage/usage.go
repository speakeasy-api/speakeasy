package usage

import (
	"fmt"
	"github.com/speakeasy-api/speakeasy/internal/schema"
	"os"

	"github.com/speakeasy-api/openapi-generation/v2/pkg/generate"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/utils"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

func OutputUsage(cmd *cobra.Command, file, header, token, out string, debug bool) error {
	ctx := cmd.Context()

	l := log.NewLogger(file)

	fmt.Printf("Generating CSV for %s...\n", file)

	isRemote, schema, err := schema.GetSchemaContents(file, header, token)
	if err != nil {
		return fmt.Errorf("failed to get schema contents: %w", err)
	}

	opts := []generate.GeneratorOptions{
		generate.WithFileFuncs(func(outFileName string, data []byte, mode os.FileMode) error {
			err := utils.CreateDirectory(outFileName)
			if err != nil {
				return err
			}
			return os.WriteFile(outFileName, data, 0644)
		}, os.ReadFile),
		generate.WithLogger(l),
		generate.WithRunLocation("cli"),
	}

	if debug {
		opts = append(opts, generate.WithDebuggingEnabled())
	}

	g, err := generate.New(opts...)
	if err != nil {
		return err
	}

	if errs := g.GenerateCSV(ctx, schema, file, out, isRemote); len(errs) > 0 {
		for _, err := range errs {
			l.Error("", zap.Error(err))
		}

		return fmt.Errorf("failed to generate CSV for %s ✖", file)
	}

	fmt.Printf("Generating CSV for %s... %s\n", file, utils.Green("done ✓"))
	return nil
}
