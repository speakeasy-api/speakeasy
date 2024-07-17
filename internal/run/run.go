package run

import (
	"bufio"
	"bytes"
	"context"
	stdErrors "errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/speakeasy-api/speakeasy/internal/download"
	"gopkg.in/yaml.v3"

	"github.com/speakeasy-api/openapi-generation/v2/pkg/generate"
	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/speakeasy-api/speakeasy-core/errors"
	"github.com/speakeasy-api/speakeasy-core/events"
	"github.com/speakeasy-api/speakeasy/internal/ask"
	"github.com/speakeasy-api/speakeasy/internal/charm/styles"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/overlay"
	"github.com/speakeasy-api/speakeasy/internal/sdkgen"
	"github.com/speakeasy-api/speakeasy/internal/transform"
	"github.com/speakeasy-api/speakeasy/internal/utils"
	"github.com/speakeasy-api/speakeasy/internal/workflowTracking"
)

const minimumViableOverlayPath = "valid-overlay.yaml"

const speakeasySelf = "speakeasy-self"

type LintingError struct {
	Err      error
	Document string
}

func (e *LintingError) Error() string {
	return fmt.Sprintf("linting failed: %s - %s", e.Document, e.Err.Error())
}

type Workflow struct {
	Target           string
	Source           string
	Repo             string
	RepoSubDirs      map[string]string
	InstallationURLs map[string]string
	SDKOverviewURLs  map[string]string
	Debug            bool
	ShouldCompile    bool
	ForceGeneration  bool
	RegistryTags     []string
	SetVersion       string

	RootStep           *workflowTracking.WorkflowStep
	workflow           workflow.Workflow
	projectDir         string
	validatedDocuments []string
	generationAccess   *sdkgen.GenerationAccess
	FromQuickstart     bool
	OperationsRemoved  []string
	computedChanges    map[string]bool
	sourceResults      map[string]*sourceResult
	lockfile           *workflow.LockFile
	lockfileOld        *workflow.LockFile // the lockfile as it was before the current run
}

func NewWorkflow(
	ctx context.Context,
	name, target, source, repo string,
	repoSubDirs, installationURLs map[string]string,
	debug, shouldCompile, forceGeneration bool,
	registryTags []string, setVersion string,
) (*Workflow, error) {
	wf, projectDir, err := utils.GetWorkflowAndDir()
	if err != nil || wf == nil {
		return nil, fmt.Errorf("failed to load workflow.yaml: %w", err)
	}

	// Load the current lockfile so that we don't overwrite all targets
	lockfile, err := workflow.LoadLockfile(projectDir)
	lockfileOld := lockfile

	if err != nil || lockfile == nil {
		lockfile = &workflow.LockFile{
			Sources: make(map[string]workflow.SourceLock),
			Targets: make(map[string]workflow.TargetLock),
		}
	}
	lockfile.SpeakeasyVersion = events.GetSpeakeasyVersionFromContext(ctx)
	lockfile.Workflow = *wf

	rootStep := workflowTracking.NewWorkflowStep(name, log.From(ctx), nil)

	return &Workflow{
		Target:           target,
		Source:           source,
		Repo:             repo,
		RepoSubDirs:      repoSubDirs,
		InstallationURLs: installationURLs,
		SDKOverviewURLs:  make(map[string]string),
		Debug:            debug,
		ShouldCompile:    shouldCompile,
		RegistryTags:     registryTags,
		workflow:         *wf,
		projectDir:       projectDir,
		RootStep:         rootStep,
		ForceGeneration:  forceGeneration,
		sourceResults:    make(map[string]*sourceResult),
		computedChanges:  make(map[string]bool),
		lockfile:         lockfile,
		lockfileOld:      lockfileOld,
		SetVersion:       setVersion,
	}, nil
}

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

	startTime := time.Now()
	err = w.RootStep.RunWithVisualization(runFnCli, updatesChannel)
	endDuration := time.Since(startTime)

	if err != nil {
		logger.Errorf("Workflow failed with error: %s", err)
	}

	criticalWarns := getCriticalWarnings(warnings)

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
	} else if len(criticalWarns) > 0 { // Display warning logs if the workflow succeeded with critical warnings
		s := strings.Join(criticalWarns, "\n")
		logger.PrintlnUnstyled(styles.MakeSection("Critical warnings found", strings.TrimSpace(s), styles.Colors.Yellow))
	}

	// Display success message if the workflow succeeded
	if err == nil && runErr == nil {
		w.printSourceSuccessMessage(ctx, logger)
		w.printTargetSuccessMessage(ctx, logger)
		_ = w.printGenerationOverview(logger, endDuration, len(criticalWarns) > 0)
	}

	return stdErrors.Join(err, runErr)
}

func (w *Workflow) Run(ctx context.Context) error {
	err := w.RunInner(ctx)

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
			sourceRes, err := w.runTarget(ctx, t)
			if err != nil {
				return err
			}

			w.sourceResults[sourceRes.Source] = sourceRes
		}
	} else if w.Source == "all" {
		for id := range w.workflow.Sources {
			_, sourceRes, err := w.runSource(ctx, w.RootStep, id, "", true)
			if err != nil {
				return err
			}

			w.sourceResults[sourceRes.Source] = sourceRes
		}
	} else if w.Target != "" {
		if _, ok := w.workflow.Targets[w.Target]; !ok {
			return fmt.Errorf("target %s not found", w.Target)
		}

		sourceRes, err := w.runTarget(ctx, w.Target)
		if err != nil {
			return err
		}

		w.sourceResults[sourceRes.Source] = sourceRes
	} else if w.Source != "" {
		if _, ok := w.workflow.Sources[w.Source]; !ok {
			return fmt.Errorf("source %s not found", w.Source)
		}

		_, sourceRes, err := w.runSource(ctx, w.RootStep, w.Source, "", true)
		if err != nil {
			return err
		}

		w.sourceResults[sourceRes.Source] = sourceRes
	}

	if err := workflow.SaveLockfile(w.projectDir, w.lockfile); err != nil {
		return err
	}
	return nil
}

func (w *Workflow) printGenerationOverview(logger log.Logger, endDuration time.Duration, criticalWarnings bool) error {
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
		fmt.Sprintf("⏲ Generated in %.1f Seconds", endDuration.Seconds()),
		"✎ Output written to " + tOut,
	}

	if w.FromQuickstart {
		additionalLines = append(additionalLines, "Execute `speakeasy run` to regenerate your SDK!")
	}

	if t.CodeSamples != nil {
		additionalLines = append(additionalLines, fmt.Sprintf("Code samples overlay file written to %s", t.CodeSamples.Output))
	}

	if criticalWarnings {
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

func (w *Workflow) retryWithMinimumViableSpec(ctx context.Context, parentStep *workflowTracking.WorkflowStep, sourceID, targetID string, cleanUp bool, viableOperations []string) (string, *sourceResult, error) {
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

	tempOmitted := fmt.Sprintf("omitted_%s%s", randStringBytes(10), filepath.Ext(baseLocation))
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

	sourcePath, sourceRes, err := w.runSource(ctx, subStep, sourceID, targetID, cleanUp)
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
