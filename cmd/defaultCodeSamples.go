package cmd

import (
	"context"
	"fmt"

	"github.com/speakeasy-api/speakeasy/internal/defaultcodesamples"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/model"
	"github.com/speakeasy-api/speakeasy/internal/model/flag"
)

var defaultCodeSamplesCmd = &model.ExecutableCommand[defaultcodesamples.DefaultCodeSamplesFlags]{
	Hidden:  true,
	Usage:   "default-code-samples",
	Aliases: []string{""},
	Short:   "Utility for generating default code samples",
	Long:    `The "default-code-samples" command allows you to generate code samples which use native language primitives for a given OpenAPI document.`,
	Run:     defaultCodeSamples,
	Flags: []flag.Flag{
		flag.StringFlag{
			Name:        "schema",
			Shorthand:   "s",
			Description: "Path to the OpenAPI schema",
			Required:    true,
		},
		flag.StringFlag{
			Name:        "language",
			Shorthand:   "l",
			Description: "Language to generate code samples for",
			Required:    true,
		},
		flag.StringFlag{
			Name:        "out",
			Shorthand:   "o",
			Description: "Output directory for generated code samples",
			Required:    true,
		},
	},
}

// You can test this command locally with the following command:
// cd internal/defaultcodesamples/ && pnpm i && pnpm run build && cd - && go run main.go default-code-samples -s ./integration/resources/spec.yaml  -l node -o /tmp/overlay.yaml
func defaultCodeSamples(ctx context.Context, flags defaultcodesamples.DefaultCodeSamplesFlags) error {
	err := defaultcodesamples.DefaultCodeSamples(ctx, flags)
	if err != nil {
		err = fmt.Errorf("failed to generate default code samples: %w", err)
		log.From(ctx).Error(err.Error())
		return err
	}

	log.From(ctx).Successf("Successfully generated default code samples for %s output to %s", flags.SchemaPath, flags.Out)

	return nil
}
