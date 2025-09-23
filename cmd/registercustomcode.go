package cmd

import (
	"context"

	"github.com/speakeasy-api/openapi-generation/v2/pkg/generate"
	"github.com/speakeasy-api/speakeasy/internal/model"
	"github.com/speakeasy-api/speakeasy/internal/model/flag"
)

type RegisterCustomCodeFlags struct {
	OutDir  string `json:"out-dir"`
	List    bool   `json:"list"`
	Resolve bool   `json:"resolve"`
}

var registerCustomCodeCmd = &model.ExecutableCommand[RegisterCustomCodeFlags]{
	Usage: "registercustomcode",
	Short: "Register custom code with the OpenAPI generation system.",
	Long:  `Register custom code with the OpenAPI generation system.`,
	Run:   registerCustomCode,
	Flags: []flag.Flag{
		flag.StringFlag{
			Name:        "out-dir",
			Shorthand:   "o",
			Description: "output directory for the registercustomcode command",
		},
		flag.BooleanFlag{
			Name:        "list",
			Shorthand:   "l",
			Description: "list custom code patches",
		},
		flag.BooleanFlag{
			Name:        "resolve",
			Shorthand:   "r",
			Description: "resolve custom code conflicts",
		},
	},
}

func registerCustomCode(_ context.Context, flags RegisterCustomCodeFlags) error {
	outDir := flags.OutDir
	if outDir == "" {
		outDir = "."
	}

	// Create generator options
	generatorOpts := []generate.GeneratorOptions{}
	
	// Create generator instance
	g, err := generate.New(generatorOpts...)
	if err != nil {
		return err
	}

	// If --list flag is provided, call ListCustomCodePatches
	if flags.List {
		return g.ListCustomCodePatches(outDir)
	}

	// If --resolve flag is provided, call ResolveCustomCodeConflicts
	if flags.Resolve {
		return g.ResolveCustomCodeConflicts(outDir)
	}

	// Call the registercustomcode functionality
	return g.RegisterCustomCode(outDir)
}