package testcmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/speakeasy-api/openapi-generation/v2/pkg/generate"
	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/speakeasy-api/speakeasy-core/auth"
	"github.com/speakeasy-api/speakeasy/internal/charm/styles"
	"github.com/speakeasy-api/speakeasy/internal/env"
	"github.com/speakeasy-api/speakeasy/internal/links"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/utils"
	"github.com/speakeasy-api/speakeasy/internal/workflowTracking"
)

// Runner is a struct that contains the options and internal state for the test
// command.
type Runner struct {
	// When enabled, skips starting the target testing mock API server before
	// running tests.
	disableMockserver bool

	// Directory of the discovered workflow configuration project directory.
	// This directory will be joined with workflow target output directory
	// in the workflow configuration, if set.
	projectDir string

	// When enabled, outputs verbose output from the generator.
	verboseOutput bool

	// Discovered workflow configuration.
	workflow *workflow.Workflow

	// Name of the workflow target to run testing against. Defaults to all
	// targets when an empty string.
	workflowTarget string

	// Enhanced CLI visualization tracker for the workflow.
	workflowTracker *workflowTracking.WorkflowStep

	testReportURLs []string
}

// NewRunner creates a new Runner with the given options.
func NewRunner(ctx context.Context, opts ...RunnerOpt) *Runner {
	// Defaults
	r := &Runner{
		workflowTracker: workflowTracking.NewWorkflowStep("Workflow Testing", log.From(ctx), nil),
	}

	for _, opt := range opts {
		opt(r)
	}

	return r
}

// Run loads the workflow, loads the generator, and runs target testing.
func (r *Runner) Run(ctx context.Context) error {
	if err := r.checkAccountType(ctx); err != nil {
		return err
	}

	if err := r.loadWorkflow(); err != nil {
		return err
	}

	if err := r.runWorkflowTargetTesting(ctx); err != nil {
		return err
	}

	return nil
}

// RunWithVisualization calls Run with enhanced CLI visualizations. The full
// logs are captured and only displayed if there is an error.
func (r *Runner) RunWithVisualization(ctx context.Context) error {
	logger := log.From(ctx)

	// Swallow but retain the logs to be displayed later, upon failure
	logCaptureBuffer := new(bytes.Buffer)
	logCapture := logger.WithWriter(logCaptureBuffer)
	updatesChannel := make(chan workflowTracking.UpdateMsg)

	r.workflowTracker = workflowTracking.NewWorkflowStep("Workflow Testing", logCapture, updatesChannel)

	var runErr error

	runFnCli := func() error {
		runCtx := log.With(ctx, logCapture)
		runErr = r.Run(runCtx)

		r.workflowTracker.Finalize(runErr == nil)

		return runErr
	}

	err := r.workflowTracker.RunWithVisualization(runFnCli, updatesChannel)

	if err != nil {
		logger.Errorf("Workflow testing failed: %s", err)
	}

	// Display error logs if the workflow failed
	if runErr != nil {
		logger.Errorf("Workflow testing failed: %s\n", runErr)

		output := strings.TrimSpace(logCaptureBuffer.String())

		logger.PrintlnUnstyled(styles.MakeSection("Workflow testing run logs", output, styles.Colors.Grey))
	}

	if len(r.testReportURLs) > 0 {
		msg := "View your test report here"
		if len(r.testReportURLs) > 1 {
			msg = "View your test reports here"
		}
		shortenedURLs := make([]string, 0, len(r.testReportURLs))
		for _, url := range r.testReportURLs {
			shortenedURLs = append(shortenedURLs, links.Shorten(ctx, url))
		}
		if runErr != nil {
			logger.Println("\n\n" + styles.RenderErrorMessage(msg, lipgloss.Center, shortenedURLs...))
		} else {
			logger.Println("\n\n" + styles.RenderSuccessMessage(msg, shortenedURLs...))
		}
	}

	return errors.Join(err, runErr)
}

// Checks the account type to ensure testing is enabled.
func (r *Runner) checkAccountType(ctx context.Context) error {
	accountType := auth.GetAccountTypeFromContext(ctx)

	if accountType == nil {
		return fmt.Errorf("Account type not found. Ensure you are logged in via the `speakeasy auth login` command or SPEAKEASY_API_KEY environment variable.")
	}

	if !CheckTestingAccountType(*accountType) {
		return fmt.Errorf("Testing is not supported on the %s account tier. Contact %s for more information.", *accountType, styles.RenderSupportEmail())
	}

	return nil
}

// Discovers the workflow configuration to set the runner project directory and
// workflow.
func (r *Runner) loadWorkflow() error {
	wf, projectDir, err := utils.GetWorkflowAndDir()

	if err != nil || wf == nil {
		return fmt.Errorf("Unable to load workflow configuration (workflow.yaml): %w", err)
	}

	r.projectDir = projectDir
	r.workflow = wf

	return nil
}

// Prepares and returns the generator instance.
func (r *Runner) prepareGenerator(ctx context.Context) (*generate.Generator, error) {
	logger := log.From(ctx)
	runLocation := env.SpeakeasyRunLocation()

	if runLocation == "" {
		runLocation = "cli"
	}

	generatorOpts := []generate.GeneratorOptions{
		generate.WithDontWrite(),
		generate.WithLogger(logger.WithFormatter(log.PrefixedFormatter)),
		generate.WithRunLocation(runLocation),
	}

	// The generator verbose output option, regardless of given value, also
	// resets the logger in the generator, so only set when enabled. Otherwise,
	// output can interleave/format incorrectly.
	if r.verboseOutput {
		generatorOpts = append(generatorOpts, generate.WithVerboseOutput(true))
	}

	generator, err := generate.New(generatorOpts...)

	if err != nil {
		return nil, fmt.Errorf("Unable to prepare testing instance: %w", err)
	}

	return generator, nil
}

// Runs testing for all given workflow targets.
func (r *Runner) runWorkflowTargetTesting(ctx context.Context) error {
	if r.workflowTarget == "" {
		return r.runAllWorkflowTargetsTesting(ctx)
	}

	workflowTarget, ok := r.workflow.Targets[r.workflowTarget]

	if !ok {
		return fmt.Errorf("Workflow target %s not found in configuration.", r.workflowTarget)
	}

	return r.runSingleWorkflowTargetTesting(ctx, r.workflowTarget, workflowTarget)
}

// Runs testing for all workflow targets.
func (r *Runner) runAllWorkflowTargetsTesting(ctx context.Context) error {
	for workflowTargetName, workflowTarget := range r.workflow.Targets {
		if err := r.runSingleWorkflowTargetTesting(ctx, workflowTargetName, workflowTarget); err != nil {
			return err
		}
	}

	return nil
}

// Runs testing for a single workflow target. The output directory is set to
// the project directory or joined with the target output directory, if set in
// the workflow configuration.
func (r *Runner) runSingleWorkflowTargetTesting(ctx context.Context, workflowTargetName string, workflowTarget workflow.Target) error {
	logger := log.From(ctx)
	outputDir := r.projectDir

	if workflowTarget.Output != nil && *workflowTarget.Output != "" {
		outputDir = filepath.Join(r.projectDir, *workflowTarget.Output)
	}

	targetTracker := r.workflowTracker.NewSubstep(fmt.Sprintf("Target: %s", workflowTargetName))

	logger.Infof("Running target %s (%s) testing...\n", workflowTargetName, workflowTarget.Target)

	logListener := make(chan log.Msg)
	testingTracker := targetTracker.NewSubstep(fmt.Sprintf("Testing %s Target", utils.CapitalizeFirst(workflowTarget.Target)))
	go testingTracker.ListenForSubsteps(logListener)

	testingLogger := logger.WithListener(logListener)
	testingCtx := log.With(ctx, testingLogger)

	generator, err := r.prepareGenerator(testingCtx)

	if err != nil {
		return err
	}

	testReportURL, err := ExecuteTargetTesting(testingCtx, generator, workflowTarget, workflowTargetName, outputDir)

	if testReportURL != "" {
		r.testReportURLs = append(r.testReportURLs, testReportURL)
	}

	if err != nil {
		return fmt.Errorf("error running workflow target %s (%s) testing: %w", workflowTargetName, workflowTarget.Target, err)
	}

	targetTracker.SucceedWorkflow()

	return nil
}
