package run

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/speakeasy-api/speakeasy-core/bundler"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	sdkGenConfig "github.com/speakeasy-api/sdk-gen-config"
	"github.com/speakeasy-api/speakeasy/internal/usagegen"

	"github.com/speakeasy-api/speakeasy-core/events"
	"github.com/speakeasy-api/speakeasy/internal/env"

	charmLog "github.com/charmbracelet/log"
	"github.com/speakeasy-api/openapi-generation/v2/pkg/generate"
	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/speakeasy-api/speakeasy/internal/charm/styles"
	"github.com/speakeasy-api/speakeasy/internal/config"
	"github.com/speakeasy-api/speakeasy/internal/download"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/sdkgen"
	"github.com/speakeasy-api/speakeasy/internal/utils"
	"github.com/speakeasy-api/speakeasy/internal/validation"
	"github.com/speakeasy-api/speakeasy/pkg/merge"
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

	RootStep           *WorkflowStep
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

	rootStep := NewWorkflowStep(name, nil)

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
	updatesChannel := make(chan UpdateMsg)
	w.RootStep = NewWorkflowStep("Workflow", updatesChannel)

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
			_, err := w.runSource(ctx, w.RootStep, id, true)
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

		_, err := w.runSource(ctx, w.RootStep, w.Source, true)
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

	log.From(ctx).Infof("Running target %s (%s)...\n", target, t.Target)

	source, sourcePath, err := w.workflow.GetTargetSource(target)
	if err != nil {
		return err
	}

	if source != nil {
		sourcePath, err = w.runSource(ctx, rootStep, t.Source, false)
		if err != nil {
			return err
		}
	} else {
		if err := w.validateDocument(ctx, rootStep, sourcePath, ""); err != nil {
			return err
		}
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

func (w *Workflow) runSource(ctx context.Context, parentStep *WorkflowStep, id string, cleanUp bool) (string, error) {
	rootStep := parentStep.NewSubstep(fmt.Sprintf("Source: %s", id))
	source := w.workflow.Sources[id]

	if len(source.Inputs) == 0 {
		return "", fmt.Errorf("source %s has no inputs", id)
	}

	rulesetToUse := ""
	if source.Ruleset != nil {
		rulesetToUse = *source.Ruleset
	}

	logger := log.From(ctx)
	logger.Infof("Running source %s...", id)

	outputLocation, err := source.GetOutputLocation()
	if err != nil {
		return "", err
	}

	// outputLocation will be the same as the input location if it's a single local file with no overlays
	// In that case, we don't need to run the bundler
	if outputLocation != source.Inputs[0].Location {
		err := w.runBundler(ctx, rootStep, source, outputLocation)
		if err != nil {
			return "", err
		}
	}

	/*
	 * Validate
	 */

	if err := w.validateDocument(ctx, rootStep, outputLocation, rulesetToUse); err != nil {
		return "", err
	}

	rootStep.SucceedWorkflow()

	if cleanUp {
		rootStep.NewSubstep("Cleaning up")

		// Clean up temp files on success
		os.RemoveAll(workflow.GetTempDir())
	}

	return outputLocation, nil
}

func (w *Workflow) runBundler(ctx context.Context, rootStep *WorkflowStep, source workflow.Source, outputLocation string) error {
	memFS := bundler.NewMemFS()
	rwfs := bundler.NewReadWriteFS(memFS, memFS)
	pipeline := bundler.NewPipeline(&bundler.PipelineOptions{
		Logger: slog.New(charmLog.New(log.From(ctx).GetWriter())),
	})

	/*
	 * Fetch input docs
	 */

	rootStep.NewSubstep("Loading OpenAPI document(s)")

	resolvedDocLocations, err := pipeline.FetchDocumentsLocalFS(ctx, rwfs, bundler.FetchDocumentsOptions{
		SourceFSBasePath: ".",
		OutputRoot:       bundler.InputsRootPath,
		Documents:        source.Inputs,
	})
	if err != nil || len(resolvedDocLocations) == 0 {
		return fmt.Errorf("error loading input OpenAPI documents: %w", err)
	}

	/*
	 * Merge input docs
	 */

	finalDocLocation := resolvedDocLocations[0]
	if len(source.Inputs) > 1 {
		rootStep.NewSubstep(fmt.Sprintf("Merging %d documents", len(source.Inputs)))

		finalDocLocation, err = pipeline.Merge(ctx, rwfs, bundler.MergeOptions{
			BasePath:   bundler.InputsRootPath,
			InputPaths: resolvedDocLocations,
		})
		if err != nil {
			return fmt.Errorf("error merging documents: %w", err)
		}
	}

	/*
	 * Fetch and apply overlays, if there are any
	 */

	if len(source.Overlays) > 0 {
		overlayStep := rootStep.NewSubstep(fmt.Sprintf("Detected %d overlay(s)", len(source.Overlays)))

		overlayStep.NewSubstep("Loading overlay documents")

		overlays, err := pipeline.FetchDocumentsLocalFS(ctx, rwfs, bundler.FetchDocumentsOptions{
			SourceFSBasePath: ".",
			OutputRoot:       bundler.OverlaysRootPath,
			Documents:        source.Overlays,
		})
		if err != nil {
			return fmt.Errorf("error fetching overlay documents: %w", err)
		}

		overlayStep.NewSubstep("Applying overlay documents")

		finalDocLocation, err = pipeline.Overlay(ctx, rwfs, bundler.OverlayOptions{
			BaseDocumentPath: finalDocLocation,
			OverlayPaths:     overlays,
		})
		if err != nil {
			return fmt.Errorf("error applying overlays: %w", err)
		}
	}

	/*
	 * Persist final document
	 */

	rootStep.NewSubstep("Writing final document")

	if err = os.MkdirAll(filepath.Dir(outputLocation), os.ModePerm); err != nil {
		return err
	}

	dst, err := os.Create(outputLocation)
	if err != nil {
		return fmt.Errorf("error creating output file: %w", err)
	}

	file, err := rwfs.Open(finalDocLocation)
	if err != nil {
		return fmt.Errorf("error opening final document: %w", err)
	}

	_, err = io.Copy(dst, file)
	return err
}

func (w *Workflow) validateDocument(ctx context.Context, parentStep *WorkflowStep, schemaPath, defaultRuleset string) error {
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

// TODO: RETAIN THE INPUT_ addition
func resolveRemoteDocument(ctx context.Context, d workflow.Document, outPath string) (string, error) {
	log.From(ctx).Infof("Downloading %s... to %s\n", d.Location, outPath)

	if err := os.MkdirAll(filepath.Dir(outPath), os.ModePerm); err != nil {
		return "", err
	}

	var token, header string
	if d.Auth != nil {
		header = d.Auth.Header
		envVar := strings.TrimPrefix(d.Auth.Secret, "$")

		// GitHub action secrets are prefixed with INPUT_
		if env.IsGithubAction() {
			envVar = "INPUT_" + envVar
		}
		token = os.Getenv(envVar)
	}

	if err := download.DownloadFile(d.Location, outPath, header, token); err != nil {
		return "", err
	}

	return outPath, nil
}

func mergeDocuments(ctx context.Context, inSchemas []string, outFile, defaultRuleset, workingDir string) error {
	if err := os.MkdirAll(filepath.Dir(outFile), os.ModePerm); err != nil {
		return err
	}

	if err := merge.MergeOpenAPIDocuments(ctx, inSchemas, outFile, defaultRuleset, workingDir); err != nil {
		return err
	}

	log.From(ctx).Printf("Successfully merged %d schemas into %s", len(inSchemas), outFile)

	return nil
}
