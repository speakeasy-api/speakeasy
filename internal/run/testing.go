package run

import (
	"context"
	"fmt"

	"github.com/speakeasy-api/openapi-generation/v2/pkg/generate"
	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/speakeasy-api/speakeasy/internal/env"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/workflowTracking"
)

// Prepares and returns the target testing generator instance.
func (w Workflow) prepareTestingGenerator(ctx context.Context) (*generate.Generator, error) {
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
	if w.Verbose {
		generatorOpts = append(generatorOpts, generate.WithVerboseOutput(true))
	}

	generator, err := generate.New(generatorOpts...)

	if err != nil {
		return nil, fmt.Errorf("Unable to prepare testing instance: %w", err)
	}

	return generator, nil
}

// Runs target testing for the given target. Returns an error if the generator
// setup or testing commands failed.
func (w Workflow) runTesting(ctx context.Context, workflowTargetName string, target workflow.Target, testingStep *workflowTracking.WorkflowStep, outputDir string) error {
	logger := log.From(ctx)
	logListener := make(chan log.Msg)

	go testingStep.ListenForSubsteps(logListener)

	testingLogger := logger.WithListener(logListener)
	testingCtx := log.With(ctx, testingLogger)

	generator, err := w.prepareTestingGenerator(testingCtx)

	if err != nil {
		return err
	}

	if err := generator.RunTargetTesting(testingCtx, target.Target, outputDir); err != nil {
		return fmt.Errorf("error running workflow target %s (%s) testing: %w", workflowTargetName, target.Target, err)
	}

	testingStep.SucceedWorkflow()

	return nil
}

// Returns true if target configuration enables testing.
func targetEnablesTesting(target workflow.Target) bool {
	// Testing should only run if the target explicitly has testing enabled.
	if target.Testing == nil || target.Testing.Enabled == nil {
		return false
	}

	return *target.Testing.Enabled
}
