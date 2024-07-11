package generate

import (
	"context"
	"fmt"
	"github.com/speakeasy-api/speakeasy/internal/codesamples"

	charm_internal "github.com/speakeasy-api/speakeasy/internal/charm"
	"github.com/speakeasy-api/speakeasy/internal/charm/styles"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/model"
	"github.com/speakeasy-api/speakeasy/internal/model/flag"
)

type codeSamplesFlags struct {
	Schema     string   `json:"schema"`
	Header     string   `json:"header"`
	Token      string   `json:"token"`
	Langs      []string `json:"langs"`
	ConfigPath string   `json:"config-path"`
	Out        string   `json:"out"`
	Style      string   `json:"style"`
}

var codeSamplesCmd = &model.ExecutableCommand[codeSamplesFlags]{
	Usage: "codeSamples",
	Short: "Creates an overlay for a given spec containing x-codeSamples extensions for the given languages.",
	Run:   runCodeSamples,
	Flags: []flag.Flag{
		flag.StringFlag{
			Name:                       "schema",
			Shorthand:                  "s",
			Description:                "the schema to generate code samples for",
			Required:                   true,
			AutocompleteFileExtensions: charm_internal.OpenAPIFileExtensions,
		},
		headerFlag,
		tokenFlag,
		flag.StringSliceFlag{
			Name:        "langs",
			Shorthand:   "l",
			Description: "the languages to generate code samples for",
			Required:    true,
		},
		flag.StringFlag{
			Name:         "config-path",
			Description:  "the path to the directory containing the gen.yaml file(s) to use",
			DefaultValue: ".",
		},
		flag.StringFlag{
			Name:        "out",
			Description: "write directly to a file instead of stdout",
		},
		flag.EnumFlag{
			Name:          "style",
			Description:   "the codeSamples style to generate, usually based on where the code samples will be used",
			DefaultValue:  "standard",
			AllowedValues: []string{"standard", "readme"},
		},
	},
}

func runCodeSamples(ctx context.Context, flags codeSamplesFlags) error {
	var style codesamples.CodeSamplesStyle
	switch flags.Style {
	case "standard":
		style = codesamples.Default
	case "readme":
		style = codesamples.ReadMe
	}

	_, err := codesamples.GenerateOverlay(ctx, flags.Schema, flags.Header, flags.Token, flags.ConfigPath, flags.Out, flags.Langs, false, style)

	if err == nil {
		locationString := "Overlay file written to stdout"
		if flags.Out != "" {
			locationString = fmt.Sprintf("Overlay file written to %s", flags.Out)
		}
		log.From(ctx).Println(styles.RenderSuccessMessage("Code samples generated successfully", locationString, "To apply the overlay, use the `overlay apply` command"))
	}

	return err
}
