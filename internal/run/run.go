package run

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/speakeasy-api/speakeasy/internal/bundler"
	"github.com/speakeasy-api/speakeasy/internal/workflowTracking"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	sdkGenConfig "github.com/speakeasy-api/sdk-gen-config"
	"github.com/speakeasy-api/speakeasy/internal/usagegen"

	"github.com/speakeasy-api/openapi-generation/v2/pkg/generate"
	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/speakeasy-api/speakeasy-core/events"
	"github.com/speakeasy-api/speakeasy/internal/charm/styles"
	"github.com/speakeasy-api/speakeasy/internal/config"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/sdkgen"
	"github.com/speakeasy-api/speakeasy/internal/utils"
	"github.com/speakeasy-api/speakeasy/internal/validation"
)

type Workflow struct {
	Target           string
	Source           string
	Repo             string
	RepoSubDirs      map[string]string
	InstallationURLs map[string]string
	Debug            bool
	ShouldCompile    bool
	ForceGeneration  bool

	RootStep           *workflowTracking.WorkflowStep
	workflow           *workflow.Workflow
	projectDir         string
	validatedDocuments []string
	generationAccess   *sdkgen.GenerationAccess
	FromQuickstart     bool
}

func NewWorkflow(name, target, source, repo string, repoSubDirs, installationURLs map[string]string, debug, shouldCompile, forceGeneration bool) (*Workflow, error) {
	wf, projectDir, err := utils.GetWorkflowAndDir()
	if err != nil {
		return nil, err
	}

	rootStep := workflowTracking.NewWorkflowStep(name, nil)

	return &Workflow{
		Target:           target,
		Source:           source,
		Repo:             repo,
		RepoSubDirs:      repoSubDirs,
		InstallationURLs: installationURLs,
		Debug:            debug,
		ShouldCompile:    shouldCompile,
		workflow:         wf,
		projectDir:       projectDir,
		RootStep:         rootStep,
		ForceGeneration:  forceGeneration,
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

func (w *Workflow) RunWithVisualization(ctx context.Context) error {
	updatesChannel := make(chan workflowTracking.UpdateMsg)
	w.RootStep = workflowTracking.NewWorkflowStep("Workflow", updatesChannel)

	var logs bytes.Buffer
	warnings := make([]string, 0)

	var err, runErr error
	logger := log.From(ctx)

	runFnCli := func() error {
		l := logger.WithWriter(&logs).WithWarnCapture(&warnings) // Swallow but retain the logs to be displayed later, upon failure
		runCtx := log.With(ctx, l)
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

		logger.PrintlnUnstyled(styles.MakeSection("Workflow run logs", strings.TrimSpace(logs.String()), styles.Colors.Grey))
	} else if len(criticalWarns) > 0 { // Display warning logs if the workflow succeeded with critical warnings
		s := strings.Join(criticalWarns, "\n")
		logger.PrintlnUnstyled(styles.MakeSection("Critical warnings found", strings.TrimSpace(s), styles.Colors.Yellow))
	}

	// Display success message if the workflow succeeded
	if err == nil && runErr == nil {
		t, err := getTarget(w.Target)
		if err != nil {
			return err
		}
		tOut := "the current directory"
		if t.Output != nil && *t.Output != "" && *t.Output != "." {
			tOut = *t.Output
		}
		if w.Target == "all" {
			tOut = "the paths specified in workflow.yaml"
		}

		titleMsg := " SDK Generated Successfully"
		additionalLines := []string{
			"✎ Output written to " + tOut,
			fmt.Sprintf("⏲ Generated in %.1f Seconds", endDuration.Seconds()),
		}

		if w.FromQuickstart {
			additionalLines = append(additionalLines, "Execute speakeasy run to regenerate your SDK!")
		}

		if t.CodeSamples != nil {
			additionalLines = append(additionalLines, fmt.Sprintf("Code samples overlay file written to %s", t.CodeSamples.Output))
		}

		if len(criticalWarns) > 0 {
			additionalLines = append(additionalLines, "⚠ Critical warnings found. Please review the logs above.")
			titleMsg = " SDK Generated with Warnings"
		}

		msg := styles.RenderSuccessMessage(
			t.Target+titleMsg,
			additionalLines...,
		)
		logger.Println(msg)

		if w.generationAccess != nil && !w.generationAccess.AccessAllowed {
			msg := styles.RenderInfoMessage(
				"🚀 Time to Upgrade 🚀\n",
				strings.Split(w.generationAccess.Message, "\n")...,
			)
			logger.Println("\n\n" + msg)
		}
	}

	return err
}

func (w *Workflow) Run(ctx context.Context) error {
	if w.Source != "" && w.Target != "" {
		return fmt.Errorf("cannot specify both a target and a source")
	}

	if w.Target == "all" {
		for t := range w.workflow.Targets {
			err := w.runTarget(ctx, t)
			if err != nil {
				return err
			}
		}
	} else if w.Source == "all" {
		for id := range w.workflow.Sources {
			_, err := w.runWorkflowSource(ctx, w.RootStep, id, true)
			if err != nil {
				return err
			}
		}
	} else if w.Target != "" {
		if _, ok := w.workflow.Targets[w.Target]; !ok {
			return fmt.Errorf("target %s not found", w.Target)
		}

		err := w.runTarget(ctx, w.Target)
		if err != nil {
			return err
		}
	} else if w.Source != "" {
		if _, ok := w.workflow.Sources[w.Source]; !ok {
			return fmt.Errorf("source %s not found", w.Source)
		}

		_, err := w.runWorkflowSource(ctx, w.RootStep, w.Source, true)
		if err != nil {
			return err
		}
	}

	return nil
}

func getTarget(target string) (*workflow.Target, error) {
	wf, _, err := utils.GetWorkflowAndDir()
	if err != nil {
		return nil, err
	}
	t := wf.Targets[target]
	return &t, nil
}

func (w *Workflow) runTarget(ctx context.Context, target string) error {
	rootStep := w.RootStep.NewSubstep(fmt.Sprintf("Target: %s", target))

	t := w.workflow.Targets[target]
	sourceID := t.Source

	log.From(ctx).Infof("Running target %s (%s)...\n", target, t.Target)

	source, sourcePath, err := w.workflow.GetTargetSource(target)
	if err != nil {
		return err
	}

	// If the inputPath is supplied directly, construct a wrapper source for it
	if source == nil {
		sourceID = sourcePath
		source = &workflow.Source{
			Inputs: []workflow.Document{{Location: sourcePath}},
		}
	}

	sourcePath, err = w.runSource(ctx, rootStep, sourceID, *source, false)
	if err != nil {
		return err
	}

	var outDir string
	if t.Output != nil {
		outDir = *t.Output
	} else {
		outDir = w.projectDir
	}

	published := t.IsPublished()

	genYamlStep := rootStep.NewSubstep("Validating gen.yaml")

	genConfig, err := sdkGenConfig.Load(outDir)
	if err != nil {
		return err
	}

	err = validation.ValidateConfigAndPrintErrors(ctx, t.Target, genConfig, published)
	if err != nil {
		if errors.Is(err, validation.NoConfigFound) {
			genYamlStep.Skip("gen.yaml not found, assuming new SDK")
		} else {
			return err
		}
	}

	genStep := rootStep.NewSubstep(fmt.Sprintf("Generating %s SDK", utils.CapitalizeFirst(t.Target)))

	logListener := make(chan log.Msg)
	logger := log.From(ctx).WithListener(logListener)
	ctx = log.With(ctx, logger)
	go genStep.ListenForSubsteps(logListener)

	generationAccess, err := sdkgen.Generate(
		ctx,
		config.GetCustomerID(),
		config.GetWorkspaceID(),
		t.Target,
		sourcePath,
		"",
		"",
		outDir,
		events.GetSpeakeasyVersionFromContext(ctx),
		w.InstallationURLs[target],
		w.Debug,
		true,
		published,
		false,
		w.Repo,
		w.RepoSubDirs[target],
		w.ShouldCompile,
		w.ForceGeneration,
	)
	if err != nil {
		return err
	}
	w.generationAccess = generationAccess

	if t.CodeSamples != nil {
		rootStep.NewSubstep("Generating Code Samples")
		configPath := "."
		outputPath := t.CodeSamples.Output
		if t.Output != nil {
			configPath = *t.Output
			outputPath = filepath.Join(*t.Output, outputPath)
		}

		err = usagegen.GenerateCodeSamplesOverlay(ctx, sourcePath, "", "", configPath, outputPath, []string{t.Target}, true)
		if err != nil {
			return err
		}
	}

	rootStep.NewSubstep("Cleaning up")

	// Clean up temp files on success
	os.RemoveAll(workflow.GetTempDir())

	rootStep.SucceedWorkflow()

	return nil
}

func (w *Workflow) runWorkflowSource(ctx context.Context, parentStep *workflowTracking.WorkflowStep, id string, cleanUp bool) (string, error) {
	source := w.workflow.Sources[id]
	return w.runSource(ctx, parentStep, id, source, cleanUp)
}

func (w *Workflow) runSource(ctx context.Context, parentStep *workflowTracking.WorkflowStep, id string, source workflow.Source, cleanUp bool) (string, error) {
	rootStep := parentStep.NewSubstep(fmt.Sprintf("Source: %s", id))
	logger := log.From(ctx)
	logger.Infof("Running source %s...", id)

	outputLocation, err := bundler.CompileSource(ctx, rootStep, id, source)
	if err != nil {
		return "", err
	}

	if cleanUp {
		rootStep.NewSubstep("Cleaning up")

		// Clean up temp files on success
		os.RemoveAll(workflow.GetTempDir())
	}

	rootStep.SucceedWorkflow()

	return outputLocation, nil
}

func (w *Workflow) validateDocument(ctx context.Context, parentStep *workflowTracking.WorkflowStep, schemaPath, defaultRuleset string) error {
	step := parentStep.NewSubstep("Validating document")

	if slices.Contains(w.validatedDocuments, schemaPath) {
		step.Skip("already validated")
		return nil
	}

	limits := &validation.OutputLimits{
		MaxErrors: 1000,
		MaxWarns:  1000,
	}

	res := validation.ValidateOpenAPI(ctx, schemaPath, "", "", limits, defaultRuleset, w.projectDir)

	w.validatedDocuments = append(w.validatedDocuments, schemaPath)

	return res
}
