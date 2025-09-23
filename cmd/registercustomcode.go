package cmd

import (
	"context"

	"github.com/speakeasy-api/openapi-generation/v2/pkg/generate"
	"github.com/speakeasy-api/speakeasy/internal/model"
	"github.com/speakeasy-api/speakeasy/internal/model/flag"
)

type RegisterCustomCodeFlags struct {
	OutDir string `json:"out-dir"`
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
	},
}

func registerCustomCode(ctx context.Context, flags RegisterCustomCodeFlags) error {
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

	// Call the registercustomcode functionality
	return g.RegisterCustomCode(ctx, outDir)
}