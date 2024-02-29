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
	Commands: []model.Command{overlayCompareCmd, overlayValidateCmd, overlayApplyCmd, overlayCodeSamplesCmd},
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
	Out     string `json:"out"`
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
		flag.StringFlag{
			Name:        "out",
			Description: "write directly to a file instead of stdout",
		},
	},
}

type overlayCodeSamplesFlags struct {
	Schema     string   `json:"schema"`
	Header     string   `json:"header"`
	Token      string   `json:"token"`
	Langs      []string `json:"langs"`
	ConfigPath string   `json:"config-path"`
	Out        string   `json:"out"`
}

var overlayCodeSamplesCmd = &model.ExecutableCommand[overlayCodeSamplesFlags]{
	Usage: "codeSamples",
	Short: "Creates an overlay for a given spec containing x-codeSamples extensions for the given languages.",
	Run:   runCodeSamples,
	Flags: []flag.Flag{
		flag.StringFlag{
			Name:        "schema",
			Shorthand:   "s",
			Description: "the schema to generate code samples for",
			Required:    true,
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
			Name:        "config-path",
			Description: "the path to the directory containing the gen.yaml file(s) to use",
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
	return overlay.Compare(flags.Schemas)
}

func runApply(ctx context.Context, flags overlayApplyFlags) error {
	out := os.Stdout
	if flags.Out != "" {
		file, err := os.Create(flags.Out)
		if err != nil {
			return err
		}
		defer file.Close()
		out = file
	}

	return overlay.Apply(flags.Schema, flags.Overlay, out)
}

func runCodeSamples(ctx context.Context, flags overlayCodeSamplesFlags) error {
	return overlay.CodeSamples(ctx, flags.Schema, flags.Header, flags.Token, flags.ConfigPath, flags.Out, flags.Langs)
}
