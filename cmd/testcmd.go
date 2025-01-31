package cmd

import (
	"context"
	"fmt"

	"github.com/speakeasy-api/speakeasy/internal/model"
	"github.com/speakeasy-api/speakeasy/internal/model/flag"
	"github.com/speakeasy-api/speakeasy/internal/run"
	"github.com/speakeasy-api/speakeasy/internal/testcmd"
	"github.com/speakeasy-api/speakeasy/internal/utils"
)

// testCmd is the command for running target tests.
var testCmd = &model.ExecutableCommand[testCmdFlags]{
	Usage:            "test",
	Short:            "For each workflow target, starts the mock API server and runs testing.",
	Run:              testCmdRun,
	RunInteractive:   testCmdRunInteractive,
	RequiresAuth:     true,
	UsesWorkflowFile: true,
	Flags: []flag.Flag{
		flag.BooleanFlag{
			Name:        "disable-mockserver",
			Description: "Skips starting the target testing mock API server before running tests.",
		},
		flag.BooleanFlag{
			Name:        "pinned",
			Description: "Run using the current CLI version instead of the version specified in the workflow file.",
			Hidden:      true,
		},
		flag.StringFlag{
			Name:        "target",
			Shorthand:   "t",
			Description: "Specify a single workflow target to run testing. Defaults to all targets. Use 'all' to explicitly run all targets.",
		},
		flag.BooleanFlag{
			Name:        "verbose",
			Description: "Verbose output.",
		},
	},
}

// testCmdFlags stores the testCmd flags values.
type testCmdFlags struct {
	DisableMockserver bool   `json:"disable-mockserver"`
	Pinned            bool   `json:"pinned"`
	Target            string `json:"target"`
	Verbose           bool   `json:"verbose"`
}

// Non-interactive command logic for testCmd.
func testCmdRun(ctx context.Context, flags testCmdFlags) error {
	runnerOpts, err := testCmdRunnerOpts(flags)
	if err != nil {
		return err
	}
	runner := testcmd.NewRunner(ctx, runnerOpts...)

	return runner.Run(ctx)
}

// Interactive command logic for testCmd.
func testCmdRunInteractive(ctx context.Context, flags testCmdFlags) error {
	runnerOpts, err := testCmdRunnerOpts(flags)
	if err != nil {
		return err
	}
	runner := testcmd.NewRunner(ctx, runnerOpts...)

	if flags.Verbose {
		return runner.Run(ctx)
	}

	return runner.RunWithVisualization(ctx)
}

// Returns the test command runner options based on the flags.
func testCmdRunnerOpts(flags testCmdFlags) ([]testcmd.RunnerOpt, error) {
	wf, _, err := utils.GetWorkflowAndDir()
	if err != nil {
		return nil, err
	}

	_, targets, err := run.ParseSourcesAndTargets()
	if err != nil {
		return nil, err
	}

	target := ""

	if flags.Target == "" {
		if len(wf.Targets) == 1 {
			target = targets[0]
		} else if len(wf.Targets) == 0 {
			return nil, fmt.Errorf("No targets found in workflow configuration.")
		} else {
			target, err = askForTarget("What target would you like to test?", "You may choose an individual target or 'all'.", "Let's choose a target to run tests againsts.", targets, true)
			if err != nil {
				return nil, err
			}
		}
	} else {
		target = flags.Target
	}

	if target == "all" && len(targets) == 1 {
		target = targets[0]
	}

	runnerOpts := []testcmd.RunnerOpt{
		testcmd.WithWorkflowTarget(target),
	}

	if flags.DisableMockserver {
		runnerOpts = append(runnerOpts, testcmd.WithDisableMockserver())
	}

	if flags.Verbose {
		runnerOpts = append(runnerOpts, testcmd.WithVerboseOutput())
	}

	return runnerOpts, nil
}
