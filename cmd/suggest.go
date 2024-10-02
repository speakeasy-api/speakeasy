package cmd

import (
	"context"
	"fmt"
	"github.com/speakeasy-api/speakeasy-core/suggestions"
	charminternal "github.com/speakeasy-api/speakeasy/internal/charm"
	"github.com/speakeasy-api/speakeasy/internal/model"
	"github.com/speakeasy-api/speakeasy/internal/model/flag"
	"github.com/speakeasy-api/speakeasy/internal/suggest"
	"github.com/speakeasy-api/speakeasy/internal/utils"
	"os"
)

const suggestLong = `
# Suggest 

Automatically optimise your OpenAPI document for SDK generation with an LLM powered suggestions
`

var suggestCmd = &model.CommandGroup{
	Usage:          "suggest",
	Short:          "Automatically improve your OpenAPI document with an LLM",
	Long:           utils.RenderMarkdown(suggestLong),
	InteractiveMsg: "What would you like to improve?",
	Commands:       []model.Command{suggestOperationIDsCmd, suggestErrorTypesCmd},
}

type suggestFlags struct {
	Schema  string `json:"schema"`
	Out     string `json:"out"`
	Overlay bool   `json:"overlay"`
}

var suggestFlagDefs = []flag.Flag{
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
}

var suggestOperationIDsCmd = &model.ExecutableCommand[suggestFlags]{
	Usage:        "operation-ids",
	Short:        "Automatically improve your SDK's method names",
	Run:          runSuggestOperationIDs,
	RequiresAuth: true,
	Flags:        suggestFlagDefs,
}

var suggestErrorTypesCmd = &model.ExecutableCommand[suggestFlags]{
	Usage:        "error-types",
	Short:        "Automatically improve your SDK's error handling ergonomics",
	Run:          runSuggestErrorTypes,
	RequiresAuth: true,
	Flags:        suggestFlagDefs,
}

func runSuggestOperationIDs(ctx context.Context, flags suggestFlags) error {
	return runSuggest(ctx, flags, suggestions.ModificationTypeMethodName)
}

func runSuggestErrorTypes(ctx context.Context, flags suggestFlags) error {
	return runSuggest(ctx, flags, suggestions.ModificationTypeErrorNames)
}

func runSuggest(ctx context.Context, flags suggestFlags, modificationType string) error {
	yamlOut := utils.HasYAMLExt(flags.Out)
	if flags.Overlay && !yamlOut {
		return fmt.Errorf("output path must be a YAML or YML file when generating an overlay. Set --overlay=false to write an updated spec")
	}

	outFile, err := os.Create(flags.Out)
	if err != nil {
		return err
	}
	defer outFile.Close()

	return suggest.SuggestAndWrite(ctx, modificationType, flags.Schema, flags.Overlay, yamlOut, outFile)
}
