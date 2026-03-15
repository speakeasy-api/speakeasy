package patches

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/speakeasy-api/speakeasy/internal/model"
	"github.com/speakeasy-api/speakeasy/internal/model/flag"
	"github.com/speakeasy-api/speakeasy/internal/run"
)

type captureFlags struct {
	Dir         string `json:"dir"`
	Target      string `json:"target"`
	SkipCompile bool   `json:"skip-compile"`
	AutoYes     bool   `json:"auto-yes"`
	Verbose     bool   `json:"verbose"`
}

var captureCmd = &model.ExecutableCommand[captureFlags]{
	Usage: "capture",
	Short: "Capture current customized SDK files into committed patch files",
	Run:   runCapture,
	Flags: []flag.Flag{
		flag.StringFlag{
			Name:         "dir",
			Shorthand:    "d",
			Description:  "project directory containing workflow.yaml and gen.yaml",
			DefaultValue: ".",
		},
		flag.StringFlag{
			Name:         "target",
			Shorthand:    "t",
			Description:  "target to capture. specify 'all' to run all targets",
			DefaultValue: "all",
		},
		flag.BooleanFlag{
			Name:        "skip-compile",
			Description: "skip compilation when capturing patch files",
		},
		flag.BooleanFlag{
			Name:        "auto-yes",
			Shorthand:   "y",
			Description: "auto confirm all prompts",
		},
		flag.BooleanFlag{
			Name:        "verbose",
			Description: "verbose logging",
		},
	},
}

func runCapture(ctx context.Context, flags captureFlags) error {
	absDir, err := filepath.Abs(flags.Dir)
	if err != nil {
		return fmt.Errorf("failed to resolve directory: %w", err)
	}

	originalWD, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}
	defer func() {
		_ = os.Chdir(originalWD)
	}()

	if err := os.Chdir(absDir); err != nil {
		return fmt.Errorf("failed to change directory to %s: %w", absDir, err)
	}

	workflow, err := run.NewWorkflow(
		ctx,
		run.WithTarget(flags.Target),
		run.WithShouldCompile(!flags.SkipCompile),
		run.WithSkipVersioning(true),
		run.WithAutoYes(flags.AutoYes),
		run.WithVerbose(flags.Verbose),
		run.WithAllowPrompts(false),
		run.WithPatchCapture(true),
	)
	if err != nil {
		return err
	}

	if err := workflow.Run(ctx); err != nil {
		return err
	}

	workflow.PrintSuccessSummary(ctx)
	return nil
}
