package cmd

import (
	"context"
	"github.com/speakeasy-api/speakeasy-client-sdk-go/v3/pkg/models/shared"
	charminternal "github.com/speakeasy-api/speakeasy/internal/charm"
	"github.com/speakeasy-api/speakeasy/internal/model"
	"github.com/speakeasy-api/speakeasy/internal/model/flag"
	"github.com/speakeasy-api/speakeasy/internal/suggest"
)

var suggestCmd = &model.CommandGroup{
	Usage:          "suggest",
	Short:          "Automatically improve your OpenAPI document with an LLM",
	InteractiveMsg: "What would you like to improve?",
	Commands:       []model.Command{suggestOperationIDsCmd},
}

type suggestOperationIDsFlags struct {
	Schema  string `json:"schema"`
	Out     string `json:"out"`
	Overlay bool   `json:"overlay"`
	Style   string `json:"style"`
}

var suggestOperationIDsCmd = &model.ExecutableCommand[suggestOperationIDsFlags]{
	Usage:        "operation-ids",
	Short:        "Get suggestions to improve your OpenAPI document's operation IDs",
	Run:          runSuggest,
	RequiresAuth: true,
	Flags: []flag.Flag{
		flag.StringFlag{
			Name:                       "schema",
			Shorthand:                  "s",
			Description:                "the schema to transform",
			Required:                   true,
			AutocompleteFileExtensions: charminternal.OpenAPIFileExtensions,
		},
		flag.StringFlag{
			Name:        "out",
			Shorthand:   "o",
			Description: "write the suggestion to the specified path",
			Required:    true,
		},
		flag.BooleanFlag{
			Name:         "overlay",
			Description:  "write the suggestion as an overlay to --out, instead of the full document (default: true)",
			DefaultValue: true,
		},
		flag.EnumFlag{
			Name:          "style",
			Description:   "the style of suggestion to provide",
			AllowedValues: []string{"standardize", "resource", "flatten"},
			DefaultValue:  "resource",
		},
	},
}

func runSuggest(ctx context.Context, flags suggestOperationIDsFlags) error {
	style := shared.StyleResource
	depthStyle := shared.DepthStyleNested
	switch flags.Style {
	case "standardize":
		style = shared.StyleStandardize
		depthStyle = shared.DepthStyleOriginal
	case "flatten":
		style = shared.StyleStandardize
		depthStyle = shared.DepthStyleFlat
	}

	return suggest.Suggest(ctx, flags.Schema, flags.Out, flags.Overlay, style, depthStyle)
}
