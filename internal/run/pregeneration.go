package run

import (
	"context"
	"fmt"

	"github.com/speakeasy-api/speakeasy/internal/env"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/patches"
	"github.com/speakeasy-api/speakeasy/internal/workflowTracking"
	"github.com/speakeasy-api/speakeasy/prompts"
)

func (w *Workflow) prepareTargetGeneration(ctx context.Context, target, outDir string, step *workflowTracking.WorkflowStep) (patches.PreparationResult, error) {
	logger := log.From(ctx)

	if w.ChangesetUpgrade {
		return patches.PreparationResult{}, nil
	}

	var promptFunc patches.PromptFunc
	if w.AllowPrompts {
		promptFunc = prompts.PromptForCustomCode
		if step != nil {
			promptFunc = func(summary string) (prompts.CustomCodeChoice, error) {
				return prompts.PromptForCustomCodeWithStep(summary, step)
			}
		}
	}

	result, err := patches.PrepareForGeneration(outDir, w.AutoYes, w.PatchCapture, w.ChangesetCapture, promptFunc, logger.Warnf)
	if err == nil {
		return result, nil
	}

	captureErr, ok := patches.IsCaptureRequired(err)
	if !ok {
		return result, err
	}
	if w.PatchCapture {
		return result, nil
	}
	if w.ChangesetCapture {
		return result, nil
	}
	if w.AutoYes {
		if err := w.runRequiredCapture(ctx, target, captureErr.Mode); err != nil {
			return result, err
		}
		return patches.PreparationResult{}, nil
	}
	if !w.AllowPrompts || env.IsCI() {
		return result, captureErr
	}

	shouldCapture := false
	if step != nil {
		shouldCapture, err = prompts.PromptForPatchCaptureWithStep(captureErr.Summary, step)
	} else {
		shouldCapture, err = prompts.PromptForPatchCapture(captureErr.Summary)
	}
	if err != nil {
		return result, err
	}
	if !shouldCapture {
		return result, captureErr
	}

	if err := w.runRequiredCapture(ctx, target, captureErr.Mode); err != nil {
		return result, err
	}
	return patches.PreparationResult{}, nil
}

func (w *Workflow) runRequiredCapture(ctx context.Context, target string, mode patches.CaptureMode) error {
	switch mode {
	case patches.CaptureModeChangeset:
		return w.runChangesetCapture(ctx, target)
	case "", patches.CaptureModePatchFiles:
		return w.runPatchCapture(ctx, target)
	default:
		return fmt.Errorf("unsupported capture mode %q", mode)
	}
}

func (w *Workflow) runPatchCapture(ctx context.Context, target string) error {
	if err := w.recoverHistoricalPristineForTarget(ctx, target); err != nil {
		log.From(ctx).Warnf("Unable to recover historical pristine baseline for patch capture: %v", err)
	}

	captureWorkflow, err := NewWorkflow(
		ctx,
		WithTarget(target),
		WithDebug(w.Debug),
		WithShouldCompile(w.ShouldCompile),
		WithVerbose(w.Verbose),
		WithFrozenWorkflowLock(w.FrozenWorkflowLock),
		WithSkipVersioning(true),
		WithSkipTesting(true),
		WithAutoYes(true),
		WithAllowPrompts(false),
		WithPatchCapture(true),
		WithRepo(w.Repo),
		WithRepoSubDirs(w.RepoSubDirs),
		WithInstallationURLs(w.InstallationURLs),
		WithSourceLocation(w.SourceLocation),
	)
	if err != nil {
		return fmt.Errorf("initializing patch capture workflow: %w", err)
	}

	if err := captureWorkflow.Run(ctx); err != nil {
		return fmt.Errorf("capturing generated-file edits before generation: %w", err)
	}

	return nil
}

func (w *Workflow) runChangesetCapture(ctx context.Context, target string) error {
	if err := w.recoverHistoricalPristineForTarget(ctx, target); err != nil {
		log.From(ctx).Warnf("Unable to recover historical pristine baseline for changeset capture: %v", err)
	}

	captureWorkflow, err := NewWorkflow(
		ctx,
		WithTarget(target),
		WithDebug(w.Debug),
		WithShouldCompile(w.ShouldCompile),
		WithVerbose(w.Verbose),
		WithFrozenWorkflowLock(w.FrozenWorkflowLock),
		WithSkipVersioning(true),
		WithSkipTesting(true),
		WithAutoYes(true),
		WithAllowPrompts(false),
		WithChangesetCapture(true),
		WithRepo(w.Repo),
		WithRepoSubDirs(w.RepoSubDirs),
		WithInstallationURLs(w.InstallationURLs),
		WithSourceLocation(w.SourceLocation),
	)
	if err != nil {
		return fmt.Errorf("initializing changeset capture workflow: %w", err)
	}

	if err := captureWorkflow.Run(ctx); err != nil {
		return fmt.Errorf("capturing generated-file edits into the current branch changeset: %w", err)
	}

	return nil
}
