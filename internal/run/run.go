package run

import (
	"bufio"
	"bytes"
	"context"
	stdErrors "errors"
	"fmt"
	"github.com/speakeasy-api/speakeasy/internal/download"
	"gopkg.in/yaml.v3"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/speakeasy-api/openapi-generation/v2/pkg/generate"
	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/speakeasy-api/speakeasy-core/errors"
	"github.com/speakeasy-api/speakeasy-core/events"
	"github.com/speakeasy-api/speakeasy/internal/ask"
	"github.com/speakeasy-api/speakeasy/internal/charm/styles"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/overlay"
	"github.com/speakeasy-api/speakeasy/internal/transform"
	"github.com/speakeasy-api/speakeasy/internal/utils"
	"github.com/speakeasy-api/speakeasy/internal/workflowTracking"
)

const minimumViableOverlayPath = "valid-overlay.yaml"

const speakeasySelf = "speakeasy-self"

func ParseSourcesAndTargets() ([]string, []string, error) {
	wf, _, err := utils.GetWorkflowAndDir()
	if err != nil {
		return nil, nil, err
	}

	if err := wf.Validate(generate.GetSupportedLanguages()); err != nil {
		return nil, nil, err
	}

	targets := []string{}
	for targetID := range wf.Targets {
		targets = append(targets, targetID)
	}
	slices.Sort(targets)

	sources := []string{}
	for sourceID := range wf.Sources {
		sources = append(sources, sourceID)
	}
	slices.Sort(sources)

	return sources, targets, nil
}

func (w *Workflow) GetWorkflowFile() *workflow.Workflow {
	return &w.workflow
}

func (w *Workflow) RunWithVisualization(ctx context.Context) error {
	var err, runErr error

	logger := log.From(ctx)
	var logs bytes.Buffer
	warnings := make([]string, 0)

	logCapture := logger.WithWriter(&logs).WithWarnCapture(&warnings) // Swallow but retain the logs to be displayed later, upon failure
	updatesChannel := make(chan workflowTracking.UpdateMsg)
	w.RootStep = workflowTracking.NewWorkflowStep("Workflow", logCapture, updatesChannel)

	runFnCli := func() error {
		runCtx := log.With(ctx, logCapture)
		err = w.Run(runCtx)

		w.RootStep.Finalize(err == nil)

		if err != nil {
			runErr = err
			return err
		}

		return nil
	}

	err = w.RootStep.RunWithVisualization(runFnCli, updatesChannel)

	if err != nil {
		logger.Errorf("Workflow failed with error: %s", err)
	}

	w.criticalWarns = getCriticalWarnings(warnings)

	// Display error logs if the workflow failed
	if runErr != nil {
		logger.Errorf("Workflow failed with error: %s\n", runErr)

		output := strings.TrimSpace(logs.String())

		var lintErr *LintingError
		if errors.As(runErr, &lintErr) {
			output += fmt.Sprintf("\nRun `speakeasy lint openapi -s %s` to lint the OpenAPI document in isolation for ease of debugging.", lintErr.Document)
		}

		logger.PrintlnUnstyled(styles.MakeSection("Workflow run logs", output, styles.Colors.Grey))

		filteredLogs := filterLogs(ctx, &logs)
		if !w.FromQuickstart {
			ask.OfferChatSessionOnError(ctx, filteredLogs)
		}
	} else if len(w.criticalWarns) > 0 { // Display warning logs if the workflow succeeded with critical warnings
		s := strings.Join(w.criticalWarns, "\n")
		logger.PrintlnUnstyled(styles.MakeSection("Critical warnings found", strings.TrimSpace(s), styles.Colors.Yellow))
	}

	return stdErrors.Join(err, runErr)
}

func (w *Workflow) PrintSuccessSummary(ctx context.Context) {
	// Display success message if the workflow succeeded
	w.printSourceSuccessMessage(ctx)
	w.printTargetSuccessMessage(ctx)
	_ = w.printGenerationOverview(ctx)
}

func (w *Workflow) Run(ctx context.Context) error {
	startTime := time.Now()
	err := w.RunInner(ctx)
	w.duration = time.Since(startTime)

	enrichTelemetryWithCompletedWorkflow(ctx, w)

	return err
}

func (w *Workflow) RunInner(ctx context.Context) error {
	if w.Source != "" && w.Target != "" {
		return fmt.Errorf("cannot specify both a target and a source")
	}

	if w.Target == "all" {
		if w.SetVersion != "" && len(w.workflow.Targets) > 1 {
			return fmt.Errorf("cannot manually apply a version when more than one target is specified ")
		}

		for t := range w.workflow.Targets {
			sourceRes, targetRes, err := w.runTarget(ctx, t)
			if err != nil {
				return err
			}

			w.SourceResults[sourceRes.Source] = sourceRes
			w.TargetResults[t] = targetRes
		}
	} else if w.Source == "all" {
		for id := range w.workflow.Sources {
			_, sourceRes, err := w.RunSource(ctx, w.RootStep, id, "")
			if err != nil {
				return err
			}

			w.SourceResults[sourceRes.Source] = sourceRes
		}
	} else if w.Target != "" {
		if _, ok := w.workflow.Targets[w.Target]; !ok {
			return fmt.Errorf("target %s not found", w.Target)
		}

		sourceRes, targetRes, err := w.runTarget(ctx, w.Target)
		if err != nil {
			return err
		}

		w.SourceResults[sourceRes.Source] = sourceRes
		w.TargetResults[w.Target] = targetRes
	} else if w.Source != "" {
		if _, ok := w.workflow.Sources[w.Source]; !ok {
			return fmt.Errorf("source %s not found", w.Source)
		}

		_, sourceRes, err := w.RunSource(ctx, w.RootStep, w.Source, "")
		if err != nil {
			return err
		}

		w.SourceResults[sourceRes.Source] = sourceRes
	}

	if !w.SkipCleanup {
		w.RootStep.NewSubstep("Cleaning Up")
		w.Cleanup()
	}

	if err := workflow.SaveLockfile(w.ProjectDir, w.lockfile); err != nil {
		return err
	}
	return nil
}

func (w *Workflow) Cleanup() {
	os.RemoveAll(workflow.GetTempDir())
}

func (w *Workflow) printGenerationOverview(ctx context.Context) error {
	logger := log.From(ctx)

	t, err := getTarget(w.Target)
	if err != nil {
		return err
	}
	workingDirectory, err := os.Getwd()
	if err != nil {
		return err
	}
	tOut := workingDirectory
	if t.Output != nil && *t.Output != "" && *t.Output != "." {
		tOut = *t.Output
	}
	if w.Target == "all" {
		tOut = "the paths specified in workflow.yaml"
	}

	additionalLines := []string{
		fmt.Sprintf("⏲ Generated in %.1f Seconds", w.duration.Seconds()),
		"✎ Output written to " + tOut,
	}

	if w.FromQuickstart {
		additionalLines = append(additionalLines, "Regenerate your target with `speakeasy run`!")
		additionalLines = append(additionalLines, "Review all targets with `speakeasy status`.")
	}

	if t.CodeSamples != nil {
		additionalLines = append(additionalLines, fmt.Sprintf("Code samples overlay file written to %s", t.CodeSamples.Output))
	}

	if len(w.criticalWarns) > 0 {
		additionalLines = append(additionalLines, "⚠ Critical warnings found. Please review the logs above.")
	}

	msg := styles.RenderSuccessMessage(
		fmt.Sprintf("%s", "Generation Summary"),
		additionalLines...,
	)
	logger.Println(msg)

	if len(w.OperationsRemoved) > 0 && w.FromQuickstart {
		lines := []string{
			"To fix validation issues use `speakeasy validate openapi`.",
			"The generated SDK omits the following operations:",
		}
		lines = append(lines, groupInvalidOperations(w.OperationsRemoved)...)

		msg := styles.RenderInstructionalError(
			"⚠ Validation issues detected in provided OpenAPI spec",
			lines...,
		)
		logger.Println(msg + "\n\n")
	}

	if w.generationAccess != nil && !w.generationAccess.AccessAllowed {
		msg := styles.RenderInfoMessage(
			"🚀 Time to Upgrade 🚀\n",
			strings.Split(w.generationAccess.Message, "\n")...,
		)
		logger.Println("\n\n" + msg)
	}

	return nil
}

func (w *Workflow) retryWithMinimumViableSpec(ctx context.Context, parentStep *workflowTracking.WorkflowStep, sourceID, targetID string, viableOperations []string) (string, *SourceResult, error) {
	subStep := parentStep.NewSubstep("Retrying with minimum viable document")
	source := w.workflow.Sources[sourceID]
	baseLocation := source.Inputs[0].Location
	workingDir, err := os.Getwd()
	if err != nil {
		return "", nil, err
	}

	// This is intended to only be used from quickstart, we must assume a singular input document
	if len(source.Inputs)+len(source.Overlays) > 1 {
		return "", nil, errors.New("multiple inputs are not supported for minimum viable spec")
	}

	tempOmitted := fmt.Sprintf("ommitted_%s%s", randStringBytes(10), filepath.Ext(baseLocation))
	tempBase := fmt.Sprintf("downloaded_%s%s", randStringBytes(10), filepath.Ext(baseLocation))

	if source.Inputs[0].IsRemote() {
		outResolved, err := download.ResolveRemoteDocument(ctx, source.Inputs[0], tempBase)
		if err != nil {
			return "", nil, err
		}

		baseLocation = outResolved
	}

	file, err := os.Create(filepath.Join(workingDir, tempOmitted))
	if err != nil {
		return "", nil, err
	}
	defer file.Close()

	failedRetry := false
	defer func() {
		os.Remove(filepath.Join(workingDir, tempOmitted))
		os.Remove(filepath.Join(workingDir, tempBase))
		if failedRetry {
			source.Overlays = []workflow.Overlay{}
			w.workflow.Sources[sourceID] = source
			os.Remove(filepath.Join(workingDir, minimumViableOverlayPath))
		}
	}()

	if err := transform.FilterOperations(ctx, source.Inputs[0].Location, viableOperations, true, file); err != nil {
		failedRetry = true
		return "", nil, err
	}

	overlayFile, err := os.Create(filepath.Join(workingDir, minimumViableOverlayPath))
	if err != nil {
		return "", nil, err
	}
	defer overlayFile.Close()

	if err := overlay.Compare([]string{
		baseLocation,
		tempOmitted,
	}, overlayFile); err != nil {
		failedRetry = true
		return "", nil, err
	}

	source.Overlays = []workflow.Overlay{{Document: &workflow.Document{Location: minimumViableOverlayPath}}}
	w.workflow.Sources[sourceID] = source

	sourcePath, sourceRes, err := w.RunSource(ctx, subStep, sourceID, targetID)
	if err != nil {
		failedRetry = true
		return "", nil, err
	}

	return sourcePath, sourceRes, err
}

func filterLogs(ctx context.Context, logBuffer *bytes.Buffer) string {
	logger := log.From(ctx)
	var filteredLogs strings.Builder
	scanner := bufio.NewScanner(logBuffer)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "ERROR") || strings.Contains(line, "WARN") {
			filteredLogs.WriteString(line + "\n")
		}
	}
	if err := scanner.Err(); err != nil {
		logger.Errorf("Failed to format question: %s", err)
	}

	return filteredLogs.String()
}

func groupInvalidOperations(input []string) []string {
	var result []string
	for _, op := range input[0:7] {
		joined := styles.DimmedItalic.Render(fmt.Sprintf("- %s", op))
		result = append(result, joined)
	}

	if len(input) > 7 {
		result = append(result, styles.DimmedItalic.Render(fmt.Sprintf("- ... see %s", minimumViableOverlayPath)))
	}

	return result
}

func enrichTelemetryWithCompletedWorkflow(ctx context.Context, w *Workflow) {
	cliEvent := events.GetTelemetryEventFromContext(ctx)
	if cliEvent != nil {
		mermaid, _ := w.RootStep.ToMermaidDiagram()
		cliEvent.MermaidDiagram = &mermaid
		lastStep := w.RootStep.LastStepToString()
		cliEvent.LastStep = &lastStep
		if w.lockfile != nil {
			lockFileBytes, _ := yaml.Marshal(w.lockfile)
			lockFileString := string(lockFileBytes)
			cliEvent.WorkflowLockPostRaw = &lockFileString
		}
		if w.lockfileOld != nil {
			lockFileOldBytes, _ := yaml.Marshal(w.lockfileOld)
			lockFileOldString := string(lockFileOldBytes)
			cliEvent.WorkflowLockPreRaw = &lockFileOldString
		}
	}
}
