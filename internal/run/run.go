package run

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math/rand"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/iancoleman/strcase"
	"github.com/speakeasy-api/openapi-generation/v2/pkg/generate"
	sdkGenConfig "github.com/speakeasy-api/sdk-gen-config"
	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/speakeasy-api/speakeasy-client-sdk-go/v3/pkg/models/shared"
	"github.com/speakeasy-api/speakeasy-core/auth"
	"github.com/speakeasy-api/speakeasy-core/bundler"
	"github.com/speakeasy-api/speakeasy-core/events"
	"github.com/speakeasy-api/speakeasy-core/fsextras"
	"github.com/speakeasy-api/speakeasy-core/ocicommon"
	"github.com/speakeasy-api/speakeasy/registry"
	"go.uber.org/zap"

	"github.com/speakeasy-api/speakeasy/internal/changes"
	"github.com/speakeasy-api/speakeasy/internal/charm/styles"
	"github.com/speakeasy-api/speakeasy/internal/config"
	"github.com/speakeasy-api/speakeasy/internal/download"
	"github.com/speakeasy-api/speakeasy/internal/env"
	"github.com/speakeasy-api/speakeasy/internal/git"
	"github.com/speakeasy-api/speakeasy/internal/github"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/overlay"
	"github.com/speakeasy-api/speakeasy/internal/reports"
	"github.com/speakeasy-api/speakeasy/internal/sdkgen"
	"github.com/speakeasy-api/speakeasy/internal/usagegen"
	"github.com/speakeasy-api/speakeasy/internal/utils"
	"github.com/speakeasy-api/speakeasy/internal/validation"
	"github.com/speakeasy-api/speakeasy/internal/workflowTracking"
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

	RootStep           *workflowTracking.WorkflowStep
	workflow           workflow.Workflow
	projectDir         string
	validatedDocuments []string
	generationAccess   *sdkgen.GenerationAccess
	FromQuickstart     bool
	sourceResults      map[string]*sourceResult
	lockfile           *workflow.LockFile
	lockfileOld        *workflow.LockFile // the lockfile as it was before the current run
}

type sourceResult struct {
	Source       string
	LintResult   *validation.ValidationResult
	ChangeReport *reports.ReportResult
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
		Debug:            debug,
		ShouldCompile:    shouldCompile,
		workflow:         *wf,
		projectDir:       projectDir,
		RootStep:         rootStep,
		ForceGeneration:  forceGeneration,
		sourceResults:    make(map[string]*sourceResult),
		lockfile:         lockfile,
		lockfileOld:      lockfileOld,
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

		logger.PrintlnUnstyled(styles.MakeSection("Workflow run logs", strings.TrimSpace(logs.String()), styles.Colors.Grey))
	} else if len(criticalWarns) > 0 { // Display warning logs if the workflow succeeded with critical warnings
		s := strings.Join(criticalWarns, "\n")
		logger.PrintlnUnstyled(styles.MakeSection("Critical warnings found", strings.TrimSpace(s), styles.Colors.Yellow))
	}

	// Display success message if the workflow succeeded
	if err == nil && runErr == nil {
		w.printSourceSuccessMessage(logger)
		_ = w.printTargetSuccessMessage(logger, endDuration, len(criticalWarns) > 0)
	}

	return errors.Join(err, runErr)
}

func (w *Workflow) Run(ctx context.Context) error {
	if w.Source != "" && w.Target != "" {
		return fmt.Errorf("cannot specify both a target and a source")
	}

	if w.Target == "all" {
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
		sourcePath, sourceRes, err = w.runSource(ctx, rootStep, t.Source, target, false)
		if err != nil {
			return nil, err
		}
	} else {
		res, err := w.validateDocument(ctx, rootStep, t.Source, sourcePath, "", w.projectDir)
		if err != nil {
			return nil, err
		}

		sourceRes = &sourceResult{
			Source:     t.Source,
			LintResult: res,
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

	if sourceLock, ok := w.lockfile.Sources[t.Source]; ok {
		w.lockfile.Targets[target] = workflow.TargetLock{
			Source:               t.Source,
			SourceNamespace:      sourceLock.SourceNamespace,
			SourceRevisionDigest: sourceLock.SourceRevisionDigest,
			SourceBlobDigest:     sourceLock.SourceBlobDigest,
			OutLocation:          outDir,
		}
	}

	return sourceRes, nil
}

func (w *Workflow) runSource(ctx context.Context, parentStep *workflowTracking.WorkflowStep, sourceID, targetID string, cleanUp bool) (string, *sourceResult, error) {
	rootStep := parentStep.NewSubstep(fmt.Sprintf("Source: %s", sourceID))
	source := w.workflow.Sources[sourceID]
	sourceRes := &sourceResult{
		Source: sourceID,
	}

	rulesetToUse := ""
	if source.Ruleset != nil {
		rulesetToUse = *source.Ruleset
	}

	logger := log.From(ctx)
	logger.Infof("Running Source %s...", sourceID)

	outputLocation, err := source.GetOutputLocation()
	if err != nil {
		return "", nil, err
	}

	var currentDocument string
	if len(source.Inputs) == 1 {
		var singleLocation *string
		// The output location should be the resolved location
		if len(source.Overlays) == 0 {
			singleLocation = &outputLocation
		}
		currentDocument, err = resolveDocument(ctx, source.Inputs[0], singleLocation, rootStep)
		if err != nil {
			return "", nil, err
		}
		// In registry bundles specifically we cannot know the exact file output location before pulling the bundle down
		if len(source.Overlays) == 0 && source.Inputs[0].IsSpeakeasyRegistry() {
			outputLocation = currentDocument
		}
	} else {
		mergeStep := rootStep.NewSubstep("Merge Documents")

		mergeLocation := source.GetTempMergeLocation()
		if len(source.Overlays) == 0 {
			mergeLocation = outputLocation
		}

		logger.Infof("Merging %d schemas into %s...", len(source.Inputs), mergeLocation)

		inSchemas := []string{}
		for _, input := range source.Inputs {
			resolvedPath, err := resolveDocument(ctx, input, nil, mergeStep)
			if err != nil {
				return "", nil, err
			}
			inSchemas = append(inSchemas, resolvedPath)
		}

		mergeStep.NewSubstep(fmt.Sprintf("Merge %d documents", len(source.Inputs)))

		if err := mergeDocuments(ctx, inSchemas, mergeLocation, rulesetToUse, w.projectDir); err != nil {
			return "", nil, err
		}

		currentDocument = mergeLocation
	}

	if len(source.Overlays) > 0 {
		overlayStep := rootStep.NewSubstep("Applying Overlays")

		overlayLocation := outputLocation

		logger.Infof("Applying %d overlays into %s...", len(source.Overlays), overlayLocation)

		overlaySchemas := []string{}
		for _, overlay := range source.Overlays {
			resolvedPath, err := resolveDocument(ctx, overlay, nil, overlayStep)
			if err != nil {
				return "", nil, err
			}
			overlaySchemas = append(overlaySchemas, resolvedPath)
		}

		overlayStep.NewSubstep(fmt.Sprintf("Apply %d overlay(s)", len(source.Overlays)))

		if err := overlayDocument(ctx, currentDocument, overlaySchemas, overlayLocation); err != nil {
			return "", nil, err
		}
	}

	if !isSingleRegistrySource(source) {
		err = w.snapshotSource(ctx, rootStep, sourceID, currentDocument)
		if err != nil {
			return "", nil, err
		}
	}

	// If the source has a previous tracked revision, compute changes against it
	if w.lockfileOld != nil {
		if targetLockOld, ok := w.lockfileOld.Targets[targetID]; ok {
			sourceRes.ChangeReport, err = computeChanges(ctx, rootStep, targetLockOld, currentDocument)
			if err != nil {
				// Don't fail the whole workflow if this fails
				logger.Warnf("failed to compute OpenAPI changes: %s", err.Error())
			}
		}
	}

	sourceRes.LintResult, err = w.validateDocument(ctx, rootStep, sourceID, currentDocument, rulesetToUse, w.projectDir)
	if err != nil {
		return "", nil, err
	}

	rootStep.SucceedWorkflow()

	if cleanUp {
		rootStep.NewSubstep("Cleaning Up")
		os.RemoveAll(workflow.GetTempDir())
	}

	return currentDocument, sourceRes, nil
}

func computeChanges(ctx context.Context, rootStep *workflowTracking.WorkflowStep, targetLock workflow.TargetLock, newDocPath string) (r *reports.ReportResult, err error) {
	hasSchemaRegistry, _ := auth.HasWorkspaceFeatureFlag(ctx, shared.FeatureFlagsSchemaRegistry)
	if !hasSchemaRegistry {
		return
	}

	changesStep := rootStep.NewSubstep("Computing Document Changes")

	defer func() {
		if err != nil {
			changesStep.Fail()
		}
	}()

	oldRegistryLocation := ""
	if targetLock.SourceRevisionDigest != "" && targetLock.SourceNamespace != "" {
		oldRegistryLocation = fmt.Sprintf("%s/%s@%s", "registry.speakeasyapi.dev", targetLock.SourceNamespace, targetLock.SourceRevisionDigest)
	} else {
		changesStep.Skip("no previous revision found")
		return
	}

	changesStep.NewSubstep("Downloading prior revision")

	d := workflow.Document{Location: oldRegistryLocation}
	oldDocPath, err := registry.ResolveSpeakeasyRegistryBundle(ctx, d, d.GetTempRegistryDir(workflow.GetTempDir()))
	if err != nil {
		return
	}

	changesStep.NewSubstep("Computing changes")

	c, err := changes.GetChanges(oldDocPath.LocalFilePath, newDocPath)
	if err != nil {
		return r, fmt.Errorf("error computing changes: %w", err)
	}

	cliEvent := events.GetTelemetryEventFromContext(ctx)
	if cliEvent != nil {
		summary, err := c.GetSummary()
		if err != nil {
			// cliEvent.OpenapiDiffBumpType = (*shared.OpenapiDiffBumpType)(&summary.Bump)
			// cliEvent.OpenapiDiffBreakingChangesCount = &summary.Text
			// TODO!!
			cliEvent.OpenapiDiffBumpType = shared.OpenapiDiffBumpTypeMajor.ToPointer()
			count := int64(len(summary.Table))
			cliEvent.OpenapiDiffBreakingChangesCount = &count
		}
	}

	changesStep.NewSubstep("Uploading changes report")
	report, err := reports.UploadReport(ctx, c.GetHTMLReport(), shared.TypeChanges)
	if err != nil {
		return r, fmt.Errorf("failed to persist report: %w", err)
	}
	r = &report

	log.From(ctx).Info(r.Message)

	summary, err := c.GetSummary()
	if err != nil || summary == nil {
		return r, fmt.Errorf("failed to get report summary: %w", err)
	}
	github.GenerateChangesSummary(ctx, r.URL, *summary)

	changesStep.SucceedWorkflow()
	return
}

func (w *Workflow) publishSource(ctx context.Context, rootStep *workflowTracking.WorkflowStep, sourceID, outputLocation string) error {
	pl := bundler.NewPipeline(&bundler.PipelineOptions{})
	memfs := fsextras.NewMemFS()

	rootStep.NewSubstep("Snapshotting OpenAPI Revision")

	_, err := pl.Localize(ctx, memfs, bundler.LocalizeOptions{
		DocumentPath: outputLocation,
	})
	if err != nil {
		return fmt.Errorf("error localizing openapi document: %w", err)
	}

	err = pl.BuildOCIImage(ctx, bundler.NewReadWriteFS(memfs, memfs), &bundler.OCIBuildOptions{
		Tags: []string{"latest"},
	})
	if err != nil {
		return fmt.Errorf("error bundling openapi artifact: %w", err)
	}

	serverURL := auth.GetServerURL()
	insecurePublish := false
	if strings.HasPrefix(serverURL, "http://") {
		insecurePublish = true
	}

	reg := strings.TrimPrefix(serverURL, "http://")
	reg = strings.TrimPrefix(reg, "https://")

	tags := []string{"latest"} // TODO: read from workflow.yaml
	namespaceName := strcase.ToKebab(sourceID)

	rootStep.NewSubstep("Storing OpenAPI Revision")
	pushResult, err := pl.PushOCIImage(ctx, memfs, &bundler.OCIPushOptions{
		Tags:     tags,
		Registry: reg,
		Access: ocicommon.NewRepositoryAccess(config.GetSpeakeasyAPIKey(), namespaceName, ocicommon.RepositoryAccessOptions{
			Insecure: insecurePublish,
		}),
	})
	if err != nil && !errors.Is(err, ocicommon.ErrAccessGated) {
		return fmt.Errorf("error publishing openapi bundle to registry: %w", err)
	}

	rootStep.SucceedWorkflow()

	var manifestDigest *string
	var blobDigest *string
	if pushResult.References != nil && len(pushResult.References) > 0 {
		manifestDigestStr := pushResult.References[0].ManifestDescriptor.Digest.String()
		manifestDigest = &manifestDigestStr
		manifestLayers := pushResult.References[0].Manifest.Layers
		for _, layer := range manifestLayers {
			if layer.MediaType == ocicommon.MediaTypeOpenAPIBundleV0 {
				blobDigestStr := layer.Digest.String()
				blobDigest = &blobDigestStr
				break
			}
		}
	}

	cliEvent := events.GetTelemetryEventFromContext(ctx)
	if cliEvent != nil {
		cliEvent.SourceRevisionDigest = manifestDigest
		cliEvent.SourceNamespaceName = &namespaceName
		cliEvent.SourceBlobDigest = blobDigest
	}

	w.lockfile.Sources[sourceID] = workflow.SourceLock{
		SourceNamespace:      namespaceName,
		SourceRevisionDigest: *manifestDigest,
		SourceBlobDigest:     *blobDigest,
		Tags:                 tags,
	}

	return nil
}

func (w *Workflow) validateDocument(ctx context.Context, parentStep *workflowTracking.WorkflowStep, source, schemaPath, defaultRuleset, projectDir string) (*validation.ValidationResult, error) {
	step := parentStep.NewSubstep("Validating Document")

	if slices.Contains(w.validatedDocuments, schemaPath) {
		step.Skip("already validated")
		return nil, nil
	}

	limits := &validation.OutputLimits{
		MaxErrors: 1000,
		MaxWarns:  1000,
	}

	res, err := validation.ValidateOpenAPI(ctx, source, schemaPath, "", "", limits, defaultRuleset, projectDir)

	w.validatedDocuments = append(w.validatedDocuments, schemaPath)

	return res, err
}

func (w *Workflow) snapshotSource(ctx context.Context, parentStep *workflowTracking.WorkflowStep, namespaceName string, documentPath string) error {
	hasSchemaRegistry, _ := auth.HasWorkspaceFeatureFlag(ctx, shared.FeatureFlagsSchemaRegistry)
	if !hasSchemaRegistry {
		return nil
	}

	pl := bundler.NewPipeline(&bundler.PipelineOptions{})
	memfs := fsextras.NewMemFS()

	registryStep := parentStep.NewSubstep("Tracking OpenAPI Changes")

	registryStep.NewSubstep("Snapshotting OpenAPI Revision")

	rootDocumentPath, err := pl.Localize(ctx, memfs, bundler.LocalizeOptions{
		DocumentPath: documentPath,
	})
	if err != nil {
		return fmt.Errorf("error localizing openapi document: %w", err)
	}

	gitRepo, err := git.NewLocalRepository(w.projectDir)
	if err != nil {
		log.From(ctx).Debug("error sniffing git repository", zap.Error(err))
	}

	rootDocument, err := memfs.Open(filepath.Join(bundler.BundleRoot.String(), "openapi.yaml"))
	if errors.Is(err, fs.ErrNotExist) {
		rootDocument, err = memfs.Open(filepath.Join(bundler.BundleRoot.String(), "openapi.json"))
	}
	if err != nil {
		return fmt.Errorf("error opening root document: %w", err)
	}

	annotations, err := ocicommon.NewAnnotationsFromOpenAPI(rootDocument)
	if err != nil {
		return fmt.Errorf("error extracting annotations from openapi document: %w", err)
	}

	revision := ""
	if gitRepo != nil {
		revision, err = gitRepo.HeadHash()
		if err != nil {
			log.From(ctx).Debug("error sniffing head commit hash", zap.Error(err))
		}
	}
	annotations.Revision = revision
	annotations.BundleRoot = strings.TrimPrefix(rootDocumentPath, string(os.PathSeparator))

	err = pl.BuildOCIImage(ctx, bundler.NewReadWriteFS(memfs, memfs), &bundler.OCIBuildOptions{
		Tags:         []string{"latest"},
		Reproducible: true,
		Annotations:  annotations,
	})
	if err != nil {
		return fmt.Errorf("error bundling openapi artifact: %w", err)
	}

	serverURL := auth.GetServerURL()
	insecurePublish := false
	if strings.HasPrefix(serverURL, "http://") {
		insecurePublish = true
	}

	reg := strings.TrimPrefix(serverURL, "http://")
	reg = strings.TrimPrefix(reg, "https://")

	tags := []string{"latest"} // TODO: read from workflow.yaml

	registryStep.NewSubstep("Storing OpenAPI Revision")
	pushResult, err := pl.PushOCIImage(ctx, memfs, &bundler.OCIPushOptions{
		Tags:     tags,
		Registry: reg,
		Access: ocicommon.NewRepositoryAccess(config.GetSpeakeasyAPIKey(), namespaceName, ocicommon.RepositoryAccessOptions{
			Insecure: insecurePublish,
		}),
	})
	if err != nil && !errors.Is(err, ocicommon.ErrAccessGated) {
		return fmt.Errorf("error publishing openapi bundle to registry: %w", err)
	}

	registryStep.SucceedWorkflow()

	var manifestDigest *string
	var blobDigest *string
	if pushResult.References != nil && len(pushResult.References) > 0 {
		manifestDigestStr := pushResult.References[0].ManifestDescriptor.Digest.String()
		manifestDigest = &manifestDigestStr
		manifestLayers := pushResult.References[0].Manifest.Layers
		for _, layer := range manifestLayers {
			if layer.MediaType == ocicommon.MediaTypeOpenAPIBundleV0 {
				blobDigestStr := layer.Digest.String()
				blobDigest = &blobDigestStr
				break
			}
		}
	}

	cliEvent := events.GetTelemetryEventFromContext(ctx)
	if cliEvent != nil {
		cliEvent.SourceRevisionDigest = manifestDigest
		cliEvent.SourceNamespaceName = &namespaceName
		cliEvent.SourceBlobDigest = blobDigest
	}

	w.lockfile.Sources[namespaceName] = workflow.SourceLock{
		SourceNamespace:      namespaceName,
		SourceRevisionDigest: *manifestDigest,
		SourceBlobDigest:     *blobDigest,
		Tags:                 tags,
	}

	return nil
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

	title := utils.CapitalizeFirst(t.Target + " SDK")
	titleMsg := "Generated Successfully"

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
		titleMsg = "Generated with Warnings"
	}

	msg := styles.RenderSuccessMessage(
		fmt.Sprintf("%s %s", styles.HeavilyEmphasized.Render(title), styles.Success.Render(titleMsg)),
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

func (w *Workflow) printSourceSuccessMessage(logger log.Logger) {
	if len(w.sourceResults) == 0 {
		return
	}

	for sourceID, sourceRes := range w.sourceResults {
		heading := fmt.Sprintf("Source %s %s", styles.HeavilyEmphasized.Render(sourceID), styles.Success.Render("Compiled Successfully"))
		var additionalLines []string

		appendReportLocation := func(report reports.ReportResult) {
			if location := report.Location(); location != "" {
				additionalLines = append(additionalLines, styles.Success.Render(fmt.Sprintf("└─%s: ", report.Title())+styles.Dimmed.Render(location)))
			}
		}

		if sourceRes.LintResult != nil {
			appendReportLocation(sourceRes.LintResult.Report)
		}
		if sourceRes.ChangeReport != nil {
			appendReportLocation(*sourceRes.ChangeReport)
		}

		msg := fmt.Sprintf("%s\n%s\n", styles.Success.Render(heading), strings.Join(additionalLines, "\n"))
		logger.Println(msg)
	}
}

func resolveDocument(ctx context.Context, d workflow.Document, outputLocation *string, step *workflowTracking.WorkflowStep) (string, error) {
	if d.IsSpeakeasyRegistry() {
		step.NewSubstep("Downloading registry bundle")
		hasSchemaRegistry, _ := auth.HasWorkspaceFeatureFlag(ctx, shared.FeatureFlagsSchemaRegistry)
		if !hasSchemaRegistry {
			return "", fmt.Errorf("schema registry is not enabled for this workspace")
		}

		location := d.GetTempRegistryDir(workflow.GetTempDir())
		if outputLocation != nil {
			location = *outputLocation
		}
		documentOut, err := registry.ResolveSpeakeasyRegistryBundle(ctx, d, location)
		if err != nil {
			return "", err
		}

		return documentOut.LocalFilePath, nil
	} else if d.IsRemote() {
		step.NewSubstep("Downloading remote document")
		location := d.GetTempDownloadPath(workflow.GetTempDir())
		if outputLocation != nil {
			location = *outputLocation
		}

		documentOut, err := resolveRemoteDocument(ctx, d, location)
		if err != nil {
			return "", err
		}

		return documentOut, nil
	}

	return d.Location, nil
}

func resolveRemoteDocument(ctx context.Context, d workflow.Document, outPath string) (string, error) {
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

	res, err := download.Fetch(d.Location, header, token)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	ext := filepath.Ext(outPath)
	if !slices.Contains([]string{".yaml", ".yml", ".json"}, ext) {
		ext, err := download.SniffDocumentExtension(res)
		if errors.Is(err, download.ErrUnknownDocumentType) {
			ext = ".yaml"
		} else if err != nil {
			return "", err
		}

		outPath += ext
	}

	if err := os.MkdirAll(filepath.Dir(outPath), os.ModePerm); err != nil {
		return "", err
	}

	outFile, err := os.Create(outPath)
	if err != nil {
		return "", fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()

	if _, err := io.Copy(outFile, res.Body); err != nil {
		return "", fmt.Errorf("failed to save response to location: %w", err)
	}

	log.From(ctx).Infof("Downloaded %s to %s\n", d.Location, outPath)

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

func isSingleRegistrySource(source workflow.Source) bool {
	return len(source.Inputs) == 1 && len(source.Overlays) == 0 && source.Inputs[0].IsSpeakeasyRegistry()
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
