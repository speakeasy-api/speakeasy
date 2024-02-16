package cmd

import (
	"context"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/model"
	"github.com/speakeasy-api/speakeasy/internal/model/flag"
	"github.com/speakeasy-api/speakeasy/internal/overlay"
	"os"
)

var (
	overlayFlag = flag.StringFlag{
		Name:        "overlay",
		Shorthand:   "o",
		Description: "the overlay file to use",
		Required:    true,
	}
)

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
	Schemas []string `json:"schemas"`
}

var overlayCompareCmd = &model.ExecutableCommand[overlayCompareFlags]{
	Usage: "compare",
	Short: "Given two specs, output an overlay that describes the differences between them",
	Run:   runCompare,
	Flags: []flag.Flag{
		flag.StringSliceFlag{
			Name:        "schemas",
			Shorthand:   "s",
			Description: "two schemas to compare and generate overlay from",
			Required:    true,
		},
	},
}

type overlayApplyFlags struct {
	Overlay string `json:"overlay"`
	Schema  string `json:"schema"`
}

var overlayApplyCmd = &model.ExecutableCommand[overlayApplyFlags]{
	Usage: "apply",
	Short: "Given an overlay, construct a new specification by extending a specification and applying the overlay, and output it to stdout.",
	Run:   runApply,
	Flags: []flag.Flag{
		overlayFlag,
		flag.StringFlag{
			Name:        "schema",
			Shorthand:   "s",
			Description: "the schema to extend",
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
	return overlay.Compare(flags.Schemas)
}

func runApply(ctx context.Context, flags overlayApplyFlags) error {
	return overlay.Apply(flags.Schema, flags.Overlay, os.Stdout)
}
