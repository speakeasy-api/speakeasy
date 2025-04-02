package run

import (
	"bufio"
	"bytes"
	"context"
	stdErrors "errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/samber/lo"
	"gopkg.in/yaml.v3"

	"github.com/speakeasy-api/openapi-generation/v2/pkg/generate"
	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/speakeasy-api/speakeasy-core/errors"
	"github.com/speakeasy-api/speakeasy-core/events"
	"github.com/speakeasy-api/speakeasy/internal/charm/styles"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/utils"
	"github.com/speakeasy-api/speakeasy/internal/workflowTracking"
)

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
	logger := log.From(ctx)
	var logs bytes.Buffer
	warnings := make([]string, 0)

	if w.BoostrapTests {
		msg := styles.MakeBoxed(styles.MakeBold(fmt.Sprintf("🚀 %s 🚀", styles.Info.Render("Bootstrapping Tests"))), styles.Colors.Green, lipgloss.Center)
		logger.Println(msg)
	}

	logCapture := logger.WithWriter(&logs).WithWarnCapture(&warnings) // Swallow but retain the logs to be displayed later, upon failure
	updatesChannel := make(chan workflowTracking.UpdateMsg)
	w.RootStep = workflowTracking.NewWorkflowStep("Workflow", logCapture, updatesChannel)

	var runErr error
	runFnCli := func() error {
		runCtx := log.With(ctx, logCapture)
		errInner := w.Run(runCtx)

		w.RootStep.Finalize(errInner == nil)

		if errInner != nil {
			runErr = errInner
			return errInner
		}

		return nil
	}

	err := w.RootStep.RunWithVisualization(runFnCli, updatesChannel)
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
			output += styles.DimmedItalic.Render("\nRun `speakeasy lint openapi -s %s` to lint the OpenAPI document in isolation for ease of debugging.", lintErr.Document)
		}

		logger.PrintlnUnstyled(styles.MakeSection("Workflow run logs", output, styles.Colors.Grey))
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
	w.Error = err
	w.Duration = time.Since(startTime)

	enrichTelemetryWithCompletedWorkflow(ctx, w)

	return err
}

func (w *Workflow) RunInner(ctx context.Context) error {
	if w.Source != "" && w.Target != "" {
		return fmt.Errorf("cannot specify both a target and a source")
	}

	sourceIDs := []string{w.Source}
	if w.Source == "all" {
		sourceIDs = lo.Keys(w.workflow.Sources)
	}
	targetIDs := []string{w.Target}
	if w.Target == "all" {
		targetIDs = lo.Keys(w.workflow.Targets)
	}

	if w.SetVersion != "" && len(targetIDs) > 1 {
		return fmt.Errorf("cannot manually apply a version when more than one target is specified ")
	}

	for _, sourceID := range sourceIDs {
		if sourceID == "" {
			continue
		}
		if _, ok := w.workflow.Sources[sourceID]; !ok {
			return fmt.Errorf("source '%s' not found", sourceID)
		}
		_, _, err := w.RunSource(ctx, w.RootStep, sourceID, "")
		if err != nil {
			return err
		}
	}

	for _, targetID := range targetIDs {
		if targetID == "" {
			continue
		}
		if _, ok := w.workflow.Targets[targetID]; !ok {
			return fmt.Errorf("target '%s' not found", targetID)
		}
		_, _, err := w.runTarget(ctx, targetID)
		if err != nil {
			return err
		}
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
		fmt.Sprintf("⏲ Generated in %.1f Seconds", w.Duration.Seconds()),
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
