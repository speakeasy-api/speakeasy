package cmd

import (
	"context"
	"fmt"

	"github.com/speakeasy-api/openapi-generation/v2/pkg/generate"
	"github.com/speakeasy-api/speakeasy/internal/model"
	"github.com/speakeasy-api/speakeasy/internal/model/flag"
	"github.com/speakeasy-api/speakeasy/internal/utils"
)

type RegisterCustomCodeFlags struct {
	Target  string `json:"target"`
	OutDir  string `json:"out-dir"`
	List    bool   `json:"list"`
	Resolve bool   `json:"resolve"`
}

var registerCustomCodeCmd = &model.ExecutableCommand[RegisterCustomCodeFlags]{
	Usage:  "registercustomcode",
	Short:  "Register custom code with the OpenAPI generation system.",
	Long:   `Register custom code with the OpenAPI generation system.`,
	Run:    registerCustomCode,
	Flags:  []flag.Flag{
		flag.StringFlag{
			Name:        "target",
			Shorthand:   "t",
			Description: "target to run. specify 'all' to run all targets",
		},
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

func registerCustomCode(ctx context.Context, flags RegisterCustomCodeFlags) error {
	outDir := flags.OutDir
	if outDir == "" {
		outDir = "."
	}

	// Load workflow to get target and schemaPath
	wf, _, err := utils.GetWorkflowAndDir()
	if err != nil {
		return fmt.Errorf("failed to load workflow: %w", err)
	}

	// Get the target from the command flag or use the single available target
	var target string
	var schemaPath string
	
	if flags.Target != "" {
		target = flags.Target
	} else if len(wf.Targets) == 1 {
		// If no target specified but there's exactly one target, use it
		for tid := range wf.Targets {
			target = tid
			break
		}
	}

	if target == "" {
		return fmt.Errorf("no target specified and no targets found in workflow")
	}

	// Get the target configuration
	targetConfig, exists := wf.Targets[target]
	if !exists {
		return fmt.Errorf("target '%s' not found in workflow", target)
	}

	// Get the schema path from the target's source
	source, sourcePath, err := wf.GetTargetSource(target)
	if err != nil {
		return fmt.Errorf("failed to get target source: %w", err)
	}

	if source != nil {
		// Source is defined in workflow, use the source inputs
		for _, input := range source.Inputs {
			if input.Location != "" {
				schemaPath = string(input.Location)
				break
			}
		}
	} else if sourcePath != "" {
		// Direct source path specified
		schemaPath = sourcePath
	} else {
		// Use the target source as the schema path
		schemaPath = targetConfig.Source
	}

	if schemaPath == "" {
		return fmt.Errorf("could not determine schema path for target '%s'", target)
	}

	// Create generator instance
	g, err := generate.New()
	if err != nil {
		return err
	}

	// If --list flag is provided, call ListCustomCodePatch
	if flags.List {
		return g.ListCustomCodePatch(outDir)
	}

	// Call the registercustomcode functionality
	return g.RegisterCustomCode(outDir, targetConfig.Target, schemaPath)
}