package cmd

import (
	"context"
	charm_internal "github.com/speakeasy-api/speakeasy/internal/charm"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/model"
	"github.com/speakeasy-api/speakeasy/internal/model/flag"
	"github.com/speakeasy-api/speakeasy/internal/overlay"
	"github.com/speakeasy-api/speakeasy/internal/utils"
	"os"
)

var overlayFlag = flag.StringFlag{
	Name:        "overlay",
	Shorthand:   "o",
	Description: "the overlay file to use",
	Required:    true,
}

var overlayCmd = &model.CommandGroup{
	Usage:    "overlay",
	Short:    "Work with OpenAPI Overlays",
	Commands: []model.Command{overlayCompareCmd, overlayValidateCmd, overlayApplyCmd},
}

type overlayValidateFlags struct {
	Overlay string `json:"overlay"`
}

var overlayValidateCmd = &model.ExecutableCommand[overlayValidateFlags]{
	Usage: "validate",
	Short: "Given an overlay, validate it according to the OpenAPI Overlay specification",
	Run:   runValidateOverlay,
	Flags: []flag.Flag{overlayFlag},
}

type overlayCompareFlags struct {
	Before string `json:"before"`
    After  string `json:"after"`
}

var overlayCompareCmd = &model.ExecutableCommand[overlayCompareFlags]{
	Usage: "compare",
	Short: "Given two specs (before and after), output an overlay that describes the differences between them",
	Run:   runCompare,
	Flags: []flag.Flag{
		flag.StringFlag{
			Name:        "before",
			Shorthand:   "b",
			Description: "the before schema to compare",
			Required:    true,
		},
		flag.StringFlag{
			Name:        "after",
			Shorthand:   "a",
			Description: "the after schema to compare",
			Required:    true,
		},
	},
}

type overlayApplyFlags struct {
	Overlay string `json:"overlay"`
	Schema  string `json:"schema"`
	Strict  bool `json:"strict"`
	Out     string `json:"out"`
}

var overlayApplyCmd = &model.ExecutableCommand[overlayApplyFlags]{
	Usage: "apply",
	Short: "Given an overlay, construct a new specification by extending a specification and applying the overlay, and output it to stdout.",
	Run:   runApply,
	Flags: []flag.Flag{
		overlayFlag,
		flag.StringFlag{
			Name:                       "schema",
			Shorthand:                  "s",
			Description:                "the schema to extend",
			AutocompleteFileExtensions: charm_internal.OpenAPIFileExtensions,
		},
		flag.BooleanFlag{
			Name:                       "strict",
			Description:                "fail if the overlay has any action target expressions which match no nodes, and produce warnings if any overlay actions do nothing",
		},
		flag.StringFlag{
			Name:        "out",
			Description: "write directly to a file instead of stdout",
		},
	},
}

func runValidateOverlay(ctx context.Context, flags overlayValidateFlags) error {
	if err := overlay.Validate(flags.Overlay); err != nil {
		return err
	}

	log.From(ctx).Successf("Overlay file %q is valid.", flags.Overlay)
	return nil
}

func runCompare(ctx context.Context, flags overlayCompareFlags) error {
	schemas := []string{flags.Before, flags.After}
	return overlay.Compare(schemas, os.Stdout)
}

func runApply(ctx context.Context, flags overlayApplyFlags) error {
	out := os.Stdout
	yamlOut := true

	if flags.Out != "" {
		file, err := os.Create(flags.Out)
		if err != nil {
			return err
		}
		defer file.Close()
		out = file

		yamlOut = utils.HasYAMLExt(flags.Out)
	}

	return overlay.Apply(flags.Schema, flags.Overlay, yamlOut, out, flags.Strict, len(flags.Out) > 0 && flags.Strict)
}
