package run

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"golang.org/x/exp/maps"

	sdkGenConfig "github.com/speakeasy-api/sdk-gen-config"
	"github.com/speakeasy-api/speakeasy-client-sdk-go/v3/pkg/models/shared"
	"github.com/speakeasy-api/speakeasy/internal/usagegen"

	"github.com/speakeasy-api/speakeasy-core/auth"
	"github.com/speakeasy-api/speakeasy-core/bundler"
	"github.com/speakeasy-api/speakeasy-core/events"
	"github.com/speakeasy-api/speakeasy-core/fsextras"
	"github.com/speakeasy-api/speakeasy/internal/env"

	"github.com/speakeasy-api/openapi-generation/v2/pkg/generate"
	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/speakeasy-api/speakeasy/internal/charm/styles"
	"github.com/speakeasy-api/speakeasy/internal/config"
	"github.com/speakeasy-api/speakeasy/internal/download"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/overlay"
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
	workflow           workflow.Workflow
	projectDir         string
	validatedDocuments []string
	generationAccess   *sdkgen.GenerationAccess
	FromQuickstart     bool
	lockfile           *workflow.LockFile
}

type sourceResult struct {
	Source       string
	ReportOutput string
}

func NewWorkflow(
	ctx context.Context,
	name, target, source, repo string,
	repoSubDirs, installationURLs map[string]string,
	debug, shouldCompile, forceGeneration bool,
) (*Workflow, error) {
	wf, projectDir, err := utils.GetWorkflowAndDir()
	if err != nil || wf == nil {
		return nil, fmt.Errorf("failed to load workflow.yaml: %w", err)
	}

	// Load the current lockfile so that we don't overwrite all targets
	lockfile, err := workflow.LoadLockfile(projectDir)
	if err != nil || lockfile == nil {
		lockfile = &workflow.LockFile{
			Sources: make(map[string]workflow.SourceLock),
			Targets: make(map[string]workflow.TargetLock),
		}
	}
	lockfile.SpeakeasyVersion = events.GetSpeakeasyVersionFromContext(ctx)
	lockfile.Workflow = *wf

	rootStep := NewWorkflowStep(name, nil)

	return &Workflow{
		Target:           target,
		Source:           source,
		Repo:             repo,
		RepoSubDirs:      repoSubDirs,
		InstallationURLs: installationURLs,
		Debug:            debug,
		ShouldCompile:    shouldCompile,
		workflow:         *wf,
		projectDir:       projectDir,
		RootStep:         rootStep,
		ForceGeneration:  forceGeneration,
		lockfile:         lockfile,
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
	var sourceResults map[string]*sourceResult
	logger := log.From(ctx)

	runFnCli := func() error {
		l := logger.WithWriter(&logs).WithWarnCapture(&warnings) // Swallow but retain the logs to be displayed later, upon failure
		runCtx := log.With(ctx, l)
		sourceResults, err = w.Run(runCtx)

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
		w.printSourceSuccessMessage(logger, sourceResults)
		_ = w.printTargetSuccessMessage(logger, endDuration, len(criticalWarns) > 0)
	}

	return errors.Join(err, runErr)
}

func (w *Workflow) Run(ctx context.Context) (map[string]*sourceResult, error) {
	if w.Source != "" && w.Target != "" {
		return nil, fmt.Errorf("cannot specify both a target and a source")
	}

	sourceResults := make(map[string]*sourceResult)

	if w.Target == "all" {
		for t := range w.workflow.Targets {
			sourceRes, err := w.runTarget(ctx, t)
			if err != nil {
				return nil, err
			}

			sourceResults[sourceRes.Source] = sourceRes
		}
	} else if w.Source == "all" {
		for id := range w.workflow.Sources {
			_, sourceRes, err := w.runSource(ctx, w.RootStep, id, true)
			if err != nil {
				return nil, err
			}

			sourceResults[sourceRes.Source] = sourceRes
		}
	} else if w.Target != "" {
		if _, ok := w.workflow.Targets[w.Target]; !ok {
			return nil, fmt.Errorf("target %s not found", w.Target)
		}

		sourceRes, err := w.runTarget(ctx, w.Target)
		if err != nil {
			return nil, err
		}

		sourceResults[sourceRes.Source] = sourceRes
	} else if w.Source != "" {
		if _, ok := w.workflow.Sources[w.Source]; !ok {
			return nil, fmt.Errorf("source %s not found", w.Source)
		}

		_, sourceRes, err := w.runSource(ctx, w.RootStep, w.Source, true)
		if err != nil {
			return nil, err
		}

		sourceResults[sourceRes.Source] = sourceRes
	}

	if err := workflow.SaveLockfile(w.projectDir, w.lockfile); err != nil {
		return nil, err
	}

	return sourceResults, nil
}

func getTarget(target string) (*workflow.Target, error) {
	wf, _, err := utils.GetWorkflowAndDir()
	if err != nil {
		return nil, err
	}
	t := wf.Targets[target]
	return &t, nil
}

func (w *Workflow) runTarget(ctx context.Context, target string) (*sourceResult, error) {
	rootStep := w.RootStep.NewSubstep(fmt.Sprintf("Target: %s", target))

	t := w.workflow.Targets[target]

	log.From(ctx).Infof("Running target %s (%s)...\n", target, t.Target)

	source, sourcePath, err := w.workflow.GetTargetSource(target)
	if err != nil {
		return nil, err
	}

	var sourceRes *sourceResult

	if source != nil {
		sourcePath, sourceRes, err = w.runSource(ctx, rootStep, t.Source, false)
		if err != nil {
			return nil, err
		}
	} else {
		reportOutput, err := w.validateDocument(ctx, rootStep, sourcePath, "", w.projectDir)
		if err != nil {
			return nil, err
		}

		sourceRes = &sourceResult{
			Source:       t.Source,
			ReportOutput: reportOutput,
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
		return nil, err
	}

	err = validation.ValidateConfigAndPrintErrors(ctx, t.Target, genConfig, published)
	if err != nil {
		if errors.Is(err, validation.NoConfigFound) {
			genYamlStep.Skip("gen.yaml not found, assuming new SDK")
		} else {
			return nil, err
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
		return nil, err
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
			return nil, err
		}
	}

	rootStep.NewSubstep("Cleaning up")

	// Clean up temp files on success
	os.RemoveAll(workflow.GetTempDir())

	rootStep.SucceedWorkflow()

	w.lockfile.Targets[target] = workflow.TargetLock{
		// TODO: fill with registry info (namespace + revision digest)
		Source:      t.Source,
		OutLocation: outDir,
	}

	return sourceRes, nil
}

func (w *Workflow) runSource(ctx context.Context, parentStep *WorkflowStep, id string, cleanUp bool) (string, *sourceResult, error) {
	rootStep := parentStep.NewSubstep(fmt.Sprintf("Source: %s", id))
	source := w.workflow.Sources[id]

	rulesetToUse := ""
	if source.Ruleset != nil {
		rulesetToUse = *source.Ruleset
	}

	logger := log.From(ctx)
	logger.Infof("Running source %s...", id)

	outputLocation, err := source.GetOutputLocation()
	if err != nil {
		return "", nil, err
	}

	var currentDocument string

	if len(source.Inputs) == 1 {
		if source.Inputs[0].IsRemote() {
			rootStep.NewSubstep("Downloading document")

			downloadLocation := outputLocation
			if len(source.Overlays) > 0 {
				downloadLocation = source.Inputs[0].GetTempDownloadPath(workflow.GetTempDir())
			}

			currentDocument, err = resolveRemoteDocument(ctx, source.Inputs[0], downloadLocation)
			if err != nil {
				return "", nil, err
			}
		} else {
			currentDocument = source.Inputs[0].Location
		}
	} else {
		mergeStep := rootStep.NewSubstep("Merge documents")

		mergeLocation := source.GetTempMergeLocation()
		if len(source.Overlays) == 0 {
			mergeLocation = outputLocation
		}

		logger.Infof("Merging %d schemas into %s...", len(source.Inputs), mergeLocation)

		inSchemas := []string{}
		for _, input := range source.Inputs {
			if input.IsRemote() {
				mergeStep.NewSubstep(fmt.Sprintf("Download document from %s", input.Location))

				downloadedPath, err := resolveRemoteDocument(ctx, input, input.GetTempDownloadPath(workflow.GetTempDir()))
				if err != nil {
					return "", nil, err
				}

				inSchemas = append(inSchemas, downloadedPath)
			} else {
				inSchemas = append(inSchemas, input.Location)
			}
		}

		mergeStep.NewSubstep(fmt.Sprintf("Merge %d documents", len(source.Inputs)))

		if err := mergeDocuments(ctx, inSchemas, mergeLocation, rulesetToUse, w.projectDir); err != nil {
			return "", nil, err
		}

		currentDocument = mergeLocation
	}

	if len(source.Overlays) > 0 {
		overlayStep := rootStep.NewSubstep("Applying overlays")

		overlayLocation := outputLocation

		logger.Infof("Applying %d overlays into %s...", len(source.Overlays), overlayLocation)

		overlaySchemas := []string{}
		for _, overlay := range source.Overlays {
			if overlay.IsRemote() {
				overlayStep.NewSubstep(fmt.Sprintf("Download document from %s", overlay.Location))

				downloadedPath, err := resolveRemoteDocument(ctx, overlay, overlay.GetTempDownloadPath(workflow.GetTempDir()))
				if err != nil {
					return "", nil, err
				}

				overlaySchemas = append(overlaySchemas, downloadedPath)
			} else {
				overlaySchemas = append(overlaySchemas, overlay.Location)
			}
		}

		overlayStep.NewSubstep(fmt.Sprintf("Apply %d overlay(s)", len(source.Overlays)))

		if err := overlayDocument(ctx, currentDocument, overlaySchemas, overlayLocation); err != nil {
			return "", nil, err
		}
	}

	hasSchemaRegistry, _ := auth.HasWorkspaceFeatureFlag(ctx, shared.FeatureFlagsSchemaRegistry)
	if hasSchemaRegistry {
		pl := bundler.NewPipeline(&bundler.PipelineOptions{})
		memfs := fsextras.NewMemFS()

		registryStep := rootStep.NewSubstep("Tracking OpenAPI Changes")

		registryStep.NewSubstep("Snapshotting OpenAPI Revision")

		_, err := pl.Localize(ctx, memfs, bundler.LocalizeOptions{
			DocumentPath: filepath.Join(w.projectDir, outputLocation),
		})
		if err != nil {
			return "", nil, fmt.Errorf("error localizing openapi document: %w", err)
		}

		err = pl.BuildOCIImage(ctx, bundler.NewReadWriteFS(memfs, memfs), &bundler.OCIBuildOptions{
			Tags: []string{"latest"},
		})
		if err != nil {
			return "", nil, fmt.Errorf("error bundling openapi artifact: %w", err)
		}

		serverURL := auth.GetServerURL()
		insecurePublish := false
		if strings.HasPrefix(serverURL, "http://") {
			insecurePublish = true
		}

		reg := strings.TrimPrefix(serverURL, "http://")
		reg = strings.TrimPrefix(reg, "https://")

		registryStep.NewSubstep("Storing OpenAPI Revision")
		pushResult, err := pl.PushOCIImage(ctx, memfs, &bundler.OCIPushOptions{
			Tags:     []string{"latest"},
			Registry: reg,
			Access: bundler.NewRepositoryAccess(config.GetSpeakeasyAPIKey(), id, bundler.RepositoryAccessOptions{
				Insecure: insecurePublish,
			}),
		})
		if err != nil && !errors.Is(err, bundler.ErrAccessGated) {
			return "", nil, fmt.Errorf("error publishing openapi bundle to registry: %w", err)
		}

		registryStep.SucceedWorkflow()

		var manifestDigest *string
		var blobDigest *string
		if pushResult.References != nil && len(pushResult.References) > 0 {
			manifestDigestStr := pushResult.References[0].ManifestDescriptor.Digest.String()
			manifestDigest = &manifestDigestStr
			manifestLayers := pushResult.References[0].Manifest.Layers
			for _, layer := range manifestLayers {
				if layer.MediaType == bundler.MediaTypeOpenAPIBundleV0 {
					blobDigestStr := layer.Digest.String()
					blobDigest = &blobDigestStr
					break
				}
			}
		}

		cliEvent := events.GetTelemetryEventFromContext(ctx)
		if cliEvent != nil {
			cliEvent.SourceRevisionDigest = manifestDigest
			cliEvent.SourceNamespaceName = &id
			cliEvent.SourceBlobDigest = blobDigest
		}

	}

	reportOutput, err := w.validateDocument(ctx, rootStep, outputLocation, rulesetToUse, w.projectDir)
	if err != nil {
		return "", nil, err
	}

	sourceRes := &sourceResult{
		Source:       id,
		ReportOutput: reportOutput,
	}

	rootStep.SucceedWorkflow()

	if cleanUp {
		rootStep.NewSubstep("Cleaning up")

		// Clean up temp files on success
		os.RemoveAll(workflow.GetTempDir())
	}

	w.lockfile.Sources[id] = workflow.SourceLock{
		// TODO: fill with registry info (namespace + revision digest)
	}

	return outputLocation, sourceRes, nil
}

func (w *Workflow) validateDocument(ctx context.Context, parentStep *WorkflowStep, schemaPath, defaultRuleset, projectDir string) (string, error) {
	step := parentStep.NewSubstep("Validating document")

	if slices.Contains(w.validatedDocuments, schemaPath) {
		step.Skip("already validated")
		return "", nil
	}

	limits := &validation.OutputLimits{
		MaxErrors: 1000,
		MaxWarns:  1000,
	}

	reportOutput, err := validation.ValidateOpenAPI(ctx, schemaPath, "", "", limits, defaultRuleset, projectDir)

	w.validatedDocuments = append(w.validatedDocuments, schemaPath)

	return reportOutput, err
}

func (w *Workflow) printTargetSuccessMessage(logger log.Logger, endDuration time.Duration, criticalWarnings bool) error {
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

	titleMsg := " SDK Generated Successfully"
	additionalLines := []string{
		"✎ Output written to " + tOut,
		fmt.Sprintf("⏲ Generated in %.1f Seconds", endDuration.Seconds()),
	}

	if w.FromQuickstart {
		additionalLines = append(additionalLines, "Execute `speakeasy run` to regenerate your SDK!")
	}

	if t.CodeSamples != nil {
		additionalLines = append(additionalLines, fmt.Sprintf("Code samples overlay file written to %s", t.CodeSamples.Output))
	}

	if criticalWarnings {
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

	return nil
}

func (w *Workflow) printSourceSuccessMessage(logger log.Logger, sourceResults map[string]*sourceResult) {
	if len(sourceResults) == 0 {
		return
	}

	titleMsg := fmt.Sprintf("Source %s Compiled Successfully", maps.Keys(sourceResults)[0])
	if len(sourceResults) > 1 {
		titleMsg = "Sources Compiled Successfully"
	}

	var additionalLines []string

	for sourceID, sourceRes := range sourceResults {
		sourceLabel := ""
		if len(sourceResults) > 1 {
			sourceLabel = styles.Emphasized.Render(sourceID) + " - "
		}

		parts := strings.SplitN(sourceRes.ReportOutput, ":", 2)

		additionalLines = append(additionalLines, fmt.Sprintf("%s%s:", sourceLabel, styles.Dimmed.Render(parts[0])))
		additionalLines = append(additionalLines, parts[1])
	}

	msg := styles.RenderSuccessMessage(
		titleMsg,
		additionalLines...,
	)

	logger.Println(msg)
}

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
		token = os.Getenv(strings.ToUpper(envVar))
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

func overlayDocument(ctx context.Context, schema string, overlayFiles []string, outFile string) error {
	currentBase := schema
	if err := os.MkdirAll(workflow.GetTempDir(), os.ModePerm); err != nil {
		return err
	}

	for _, overlayFile := range overlayFiles {
		applyPath := getTempApplyPath(overlayFile)

		tempOutFile, err := os.Create(applyPath)
		if err != nil {
			return err
		}

		if err := overlay.Apply(currentBase, overlayFile, tempOutFile); err != nil {
			return err
		}

		if err := tempOutFile.Close(); err != nil {
			return err
		}

		currentBase = applyPath
	}

	if err := os.MkdirAll(filepath.Dir(outFile), os.ModePerm); err != nil {
		return err
	}

	finalTempFile, err := os.Open(currentBase)
	if err != nil {
		return err
	}
	defer finalTempFile.Close()

	outFileWriter, err := os.Create(outFile)
	if err != nil {
		return err
	}
	defer outFileWriter.Close()

	if _, err := io.Copy(outFileWriter, finalTempFile); err != nil {
		return err
	}

	log.From(ctx).Successf("Successfully applied %d overlays into %s", len(overlayFiles), outFile)

	return nil
}

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

var randStringBytes = func(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}

func getTempApplyPath(overlayFile string) string {
	return filepath.Join(workflow.GetTempDir(), fmt.Sprintf("applied_%s%s", randStringBytes(10), filepath.Ext(overlayFile)))
}
