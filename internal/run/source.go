package run

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/speakeasy-api/versioning-reports/versioning"
	"golang.org/x/sync/errgroup"

	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/speakeasy-api/speakeasy-core/errors"
	"github.com/speakeasy-api/speakeasy-core/ocicommon"
	"github.com/speakeasy-api/speakeasy-core/suggestions"
	"github.com/speakeasy-api/speakeasy/internal/charm/styles"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/reports"
	"github.com/speakeasy-api/speakeasy/internal/schemas"
	"github.com/speakeasy-api/speakeasy/internal/suggest"
	"github.com/speakeasy-api/speakeasy/internal/utils"
	"github.com/speakeasy-api/speakeasy/internal/validation"
	"github.com/speakeasy-api/speakeasy/internal/workflowTracking"
	"github.com/speakeasy-api/speakeasy/pkg/transform"
)

type SourceResultCallback func(sourceRes *SourceResult, sourceStep SourceStepID) error

type SourceStepID string

const (
	// CLI steps
	SourceStepFetch     SourceStepID = "Fetching spec"
	SourceStepOverlay   SourceStepID = "Overlaying"
	SourceStepTransform SourceStepID = "Transforming"
	SourceStepLint      SourceStepID = "Linting"
	SourceStepUpload    SourceStepID = "Uploading spec"
	// Generator steps
	SourceStepStart    SourceStepID = "Started"
	SourceStepGenerate SourceStepID = "Generating SDK"
	SourceStepCompile  SourceStepID = "Compiling SDK"
	SourceStepComplete SourceStepID = "Completed"
	SourceStepCancel   SourceStepID = "Cancelling"
	SourceStepExit     SourceStepID = "Exiting"
)

type SourceResult struct {
	Source string
	// The merged OAS spec that was input to the source contents as a string
	InputSpec     string
	LintResult    *validation.ValidationResult
	ChangeReport  *reports.ReportResult
	Diagnosis     suggestions.Diagnosis
	OverlayResult OverlayResult
	MergeResult   MergeResult
	CLIVersion    string
	// The path to the output OAS spec
	OutputPath  string
	oldSpecPath string
	newSpecPath string
}

type LintingError struct {
	Err      error
	Document string
}

func (e *LintingError) Error() string {
	errString := e.Err.Error()
	if strings.Contains(e.Err.Error(), "spec type not supported by libopenapi") {
		errString = "cannot parse spec: speakeasy supports valid yaml or JSON openapi documents of version 3.0+"
	}
	return fmt.Sprintf("linting failed: %s - %s", e.Document, errString)
}

func (w *Workflow) RunSource(ctx context.Context, parentStep *workflowTracking.WorkflowStep, sourceID, targetID, targetLanguage string) (string, *SourceResult, error) {
	// Fast path: return cached result if this source was already run
	w.sourceMu.Lock()
	if cached, ok := w.SourceResults[sourceID]; ok && cached.OutputPath != "" {
		w.sourceMu.Unlock()
		return cached.OutputPath, cached, nil
	}
	w.sourceMu.Unlock()

	// Check if another goroutine is already running this source (diamond dependency case).
	// If so, wait for it to finish and return its result.
	w.sourceInflightMu.Lock()
	if inflight, ok := w.sourceInflight[sourceID]; ok {
		w.sourceInflightMu.Unlock()
		<-inflight.done
		if inflight.err != nil {
			return "", nil, inflight.err
		}
		return inflight.path, inflight.result, nil
	}
	// Register ourselves as the in-flight resolver for this source
	inflight := &sourceInflight{done: make(chan struct{})}
	w.sourceInflight[sourceID] = inflight
	w.sourceInflightMu.Unlock()

	path, result, err := w.runSourceInner(ctx, parentStep, sourceID, targetID, targetLanguage)

	// Signal completion to any waiters
	inflight.path = path
	inflight.result = result
	inflight.err = err
	close(inflight.done)

	return path, result, err
}

func (w *Workflow) runSourceInner(ctx context.Context, parentStep *workflowTracking.WorkflowStep, sourceID, targetID, targetLanguage string) (string, *SourceResult, error) {
	source := w.workflow.Sources[sourceID]

	// Resolve any source ref inputs before proceeding. Source refs are resolved
	// in parallel using an errgroup, which significantly speeds up workflows
	// where a source has many independent source ref inputs.
	// Note: we resolve refs BEFORE creating this source's progress step so that
	// dependency steps appear first in the progress tree.
	resolvedInputs := make([]workflow.Document, len(source.Inputs))
	copy(resolvedInputs, source.Inputs)

	// Collect source ref indices for parallel resolution
	var sourceRefIndices []int
	for i, input := range resolvedInputs {
		if input.IsSourceRef() {
			refName := input.SourceRefName()
			if _, ok := w.workflow.Sources[refName]; !ok {
				return "", nil, fmt.Errorf("source %q references unknown source %q", sourceID, refName)
			}
			sourceRefIndices = append(sourceRefIndices, i)
		}
	}

	if len(sourceRefIndices) > 0 {
		g, gCtx := errgroup.WithContext(ctx)
		for _, idx := range sourceRefIndices {
			idx := idx // capture loop variable
			refName := resolvedInputs[idx].SourceRefName()
			g.Go(func() error {
				refPath, _, err := w.RunSource(gCtx, parentStep, refName, targetID, targetLanguage)
				if err != nil {
					return fmt.Errorf("failed to resolve source ref %q: %w", refName, err)
				}
				resolvedInputs[idx].Location = workflow.LocationString(refPath)
				return nil
			})
		}
		if err := g.Wait(); err != nil {
			return "", nil, err
		}
	}
	source.Inputs = resolvedInputs

	rootStep := parentStep.NewSubstep(fmt.Sprintf("Source: %s", sourceID))

	hasRemoteInputs := workflowSourceHasRemoteInputs(source)
	sourceRes := &SourceResult{
		Source:    sourceID,
		Diagnosis: suggestions.Diagnosis{},
	}
	defer func() {
		w.sourceMu.Lock()
		w.SourceResults[sourceID] = sourceRes
		w.sourceOrder = append(w.sourceOrder, sourceID)
		w.sourceMu.Unlock()
		_ = w.OnSourceResult(sourceRes, SourceStepComplete)
	}()
	_ = w.OnSourceResult(sourceRes, SourceStepFetch)

	rulesetToUse := "speakeasy-generation"
	if source.Ruleset != nil {
		rulesetToUse = *source.Ruleset
	}

	logger := log.From(ctx)
	logger.Infof("Running Source %s...", sourceID)

	outputLocation, err := source.GetOutputLocation()
	if err != nil {
		return "", nil, err
	}

	frozenSource := false

	var currentDocument string
	switch {
	case w.SourceLocation != "":
		rootStep.NewSubstep("Using Source Location Override")
		currentDocument = w.SourceLocation
		frozenSource = true
	case w.FrozenWorkflowLock && hasRemoteInputs:
		frozenSource = true
		currentDocument, err = NewFrozenSource(w, rootStep, sourceID).Do(ctx, "unused")
		if err != nil {
			return "", nil, err
		}
	case len(source.Inputs) == 1:
		var singleLocation *string
		// The output location should be the resolved location
		if source.IsSingleInput() {
			singleLocation = &outputLocation
		}
		currentDocument, err = schemas.ResolveDocument(ctx, source.Inputs[0], singleLocation, rootStep)
		if err != nil {
			return "", nil, err
		}
		if source.IsSingleInput() {
			// In registry bundles specifically we cannot know the exact file output location before pulling the bundle down
			if source.Inputs[0].IsSpeakeasyRegistry() {
				outputLocation = currentDocument
			}
			// If we aren't going to touch the document because it's a single input document with no overlay, then check if we should reformat it
			// Primarily this is to improve readability of single-line documents in the Studio and Linting output
			if reformattedLocation, wasReformatted, err := maybeReformatDocument(ctx, currentDocument, rootStep); err == nil && wasReformatted {
				currentDocument = reformattedLocation
				outputLocation = reformattedLocation
			}
		}
		sourceRes.MergeResult.InputSchemaLocation = []string{currentDocument}
	default:
		sourceRes.MergeResult, err = NewMerge(rootStep, source).Do(ctx, currentDocument)
		if err != nil {
			return "", nil, err
		}
		currentDocument = sourceRes.MergeResult.Location
	}

	sourceRes.InputSpec, err = utils.ReadFileToString(currentDocument)
	if err != nil {
		return "", nil, err
	}

	if len(source.Overlays) > 0 && !frozenSource {
		_ = w.OnSourceResult(sourceRes, SourceStepOverlay)
		sourceRes.OverlayResult, err = NewOverlay(rootStep, source).Do(ctx, currentDocument)
		if err != nil {
			return "", nil, err
		}
		currentDocument = sourceRes.OverlayResult.Location
	}

	// Automatically convert Swagger 2.0 documents to OpenAPI 3.0
	// Note: This is handled here rather than as a transformation type in source.Transformations
	// as we don't want to expose this as a controllable transformation in a workflow file
	if !frozenSource {
		currentDocument, err = maybeConvertSwagger(ctx, rootStep, currentDocument, logger)
		if err != nil {
			return "", nil, err
		}
	}

	if len(source.Transformations) > 0 && !frozenSource {
		_ = w.OnSourceResult(sourceRes, SourceStepTransform)
		currentDocument, err = NewTransform(rootStep, source).Do(ctx, currentDocument)
		if err != nil {
			return "", nil, err
		}
	}

	// Must not be frozen source check! We DO want to write for source overrides
	if !w.FrozenWorkflowLock {
		if err := writeToOutputLocation(ctx, currentDocument, outputLocation); err != nil {
			return "", nil, fmt.Errorf("failed to write to output location: %w %s", err, outputLocation)
		}
	}
	sourceRes.OutputPath = outputLocation

	var lintingErr error
	if !w.SkipLinting {
		_ = w.OnSourceResult(sourceRes, SourceStepLint)
		sourceRes.LintResult, err = w.validateDocument(ctx, rootStep, sourceID, currentDocument, rulesetToUse, w.ProjectDir, targetLanguage)
		if err != nil {
			lintingErr = &LintingError{Err: err, Document: currentDocument}
		}
	}

	var diagnoseErr error
	step := rootStep.NewSubstep("Diagnosing OpenAPI")
	sourceRes.Diagnosis, diagnoseErr = suggest.Diagnose(ctx, currentDocument)
	if diagnoseErr != nil {
		step.Fail()
	} else {
		step.Succeed()
	}

	_ = w.OnSourceResult(sourceRes, SourceStepUpload)

	// Upload if validation passed OR if the spec looks like an OpenAPI spec (even if invalid)
	if !w.SkipSnapshot && (lintingErr == nil || validation.LooksLikeAnOpenAPISpec(currentDocument)) {
		err = w.snapshotSource(ctx, rootStep, sourceID, source, sourceRes, lintingErr)
		if err != nil && !errors.Is(err, ocicommon.ErrAccessGated) {
			logger.Warnf("failed to snapshot source: %s", err.Error())
		}
	}

	// Return errors after snapshot attempt - prefer linting error over diagnose error
	if lintingErr != nil {
		return "", sourceRes, lintingErr
	}
	if diagnoseErr != nil {
		return "", sourceRes, diagnoseErr
	}

	// If the source has a previous tracked revision, compute changes against it
	if w.lockfileOld != nil && !w.SkipChangeReport {
		if targetLockOld, ok := w.lockfileOld.Targets[targetID]; ok && !utils.IsZeroTelemetryOrganization(ctx) {
			changesComputed, err := w.computeChanges(ctx, rootStep, targetLockOld, currentDocument)
			if err != nil {
				// Don't fail the whole workflow if this fails
				logger.Warnf("failed to compute OpenAPI changes: %s", err.Error())
			}
			sourceRes.ChangeReport = changesComputed.report
			sourceRes.newSpecPath = currentDocument
			sourceRes.oldSpecPath = changesComputed.oldSpecPath
		}
	}

	if sourceRes.ChangeReport == nil {
		// If we failed to compute changes, always generate the SDK
		_ = versioning.AddVersionReport(ctx, versioning.VersionReport{
			MustGenerate: true,
			Key:          "openapi_change_summary",
			Priority:     5,
		})
	}

	rootStep.SucceedWorkflow()

	return currentDocument, sourceRes, nil
}

func (w *Workflow) validateDocument(ctx context.Context, parentStep *workflowTracking.WorkflowStep, source, schemaPath, defaultRuleset, projectDir string, target string) (*validation.ValidationResult, error) {
	step := parentStep.NewSubstep("Validating Document")

	w.sourceMu.Lock()
	alreadyValidated := slices.Contains(w.validatedDocuments, schemaPath)
	w.sourceMu.Unlock()

	if alreadyValidated {
		step.Skip("already validated")
		return nil, nil
	}

	limits := &validation.OutputLimits{
		MaxErrors: 1000,
		MaxWarns:  1000,
	}

	res, err := validation.ValidateOpenAPI(ctx, source, schemaPath, "", "", limits, defaultRuleset, projectDir, w.FromQuickstart, w.SkipGenerateLintReport, target)

	w.sourceMu.Lock()
	w.validatedDocuments = append(w.validatedDocuments, schemaPath)
	w.sourceMu.Unlock()
	if err != nil {
		step.FailWorkflow()
	} else {
		step.SucceedWorkflow()
	}

	return res, err
}

func (w *Workflow) printSourceSuccessMessage(ctx context.Context) {
	if len(w.SourceResults) == 0 {
		return
	}

	logger := log.From(ctx)
	logger.Println("") // Newline for better readability

	for _, sourceID := range w.sourceOrder {
		sourceRes := w.SourceResults[sourceID]
		if sourceRes == nil {
			continue
		}
		heading := fmt.Sprintf("Source `%s` Compiled Successfully", sourceID)
		var additionalLines []string

		appendReportLocation := func(report reports.ReportResult) {
			if location := report.Location(); location != "" {
				additionalLines = append(additionalLines, styles.Success.Render(fmt.Sprintf("└─%s: ", report.Title()))+styles.DimmedItalic.Render(location))
			}
		}

		if sourceRes.LintResult != nil && sourceRes.LintResult.Report != nil {
			appendReportLocation(*sourceRes.LintResult.Report)
		}
		if sourceRes.ChangeReport != nil {
			appendReportLocation(*sourceRes.ChangeReport)
		}

		// TODO: reintroduce with studio
		// if sourceRes.Diagnosis != nil && suggest.ShouldSuggest(sourceRes.Diagnosis) {
		//	baseURL := auth.GetWorkspaceBaseURL(ctx)
		//	link := fmt.Sprintf(`%s/apis/%s/suggest`, baseURL, w.lockfile.Sources[sourceID].SourceNamespace)
		//	link = links.Shorten(ctx, link)
		//
		//	msg := fmt.Sprintf("%s %s", styles.Dimmed.Render(sourceRes.Diagnosis.Summarize()+"."), styles.DimmedItalic.Render(link))
		//	additionalLines = append(additionalLines, fmt.Sprintf("`└─Improve with AI:` %s", msg))
		//}

		msg := fmt.Sprintf("%s\n%s\n", styles.Success.Render(heading), strings.Join(additionalLines, "\n"))
		logger.Println(msg)
	}
}

func isSingleRegistrySource(source workflow.Source) bool {
	return len(source.Inputs) == 1 && len(source.Overlays) == 0 && source.Inputs[0].IsSpeakeasyRegistry()
}

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

var randStringBytes = func(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}

func getTempApplyPath(path string) string {
	return filepath.Join(workflow.GetTempDir(), fmt.Sprintf("applied_%s%s", randStringBytes(10), filepath.Ext(path)))
}

func getTempConvertedPath(path string) string {
	return filepath.Join(workflow.GetTempDir(), fmt.Sprintf("converted_%s%s", randStringBytes(10), filepath.Ext(path)))
}

// Returns true if any of the source inputs are remote (including registry inputs).
func workflowSourceHasRemoteInputs(source workflow.Source) bool {
	for _, input := range source.Inputs {
		if input.IsRemote() || input.IsSpeakeasyRegistry() {
			return true
		}
	}

	return false
}

// Reformats yaml to json if necessary and writes to the output location
func writeToOutputLocation(ctx context.Context, documentPath string, outputLocation string) error {
	// If paths are the same, no need to do anything
	if documentPath == outputLocation {
		return nil
	}
	// Make sure the outputLocation directory exists
	if err := os.MkdirAll(filepath.Dir(outputLocation), os.ModePerm); err != nil {
		return err
	}

	// Check if we need to convert between formats based on file extensions
	sourceIsYAML := utils.HasYAMLExt(documentPath)
	targetIsYAML := utils.HasYAMLExt(outputLocation)

	// If formats differ, convert appropriately
	if sourceIsYAML != targetIsYAML {
		formattedBytes, err := schemas.Format(ctx, documentPath, targetIsYAML)
		if err != nil {
			return fmt.Errorf("failed to format document: %w", err)
		}

		return os.WriteFile(outputLocation, formattedBytes, 0o644)
	} else {
		// Otherwise, just copy the file over
		return utils.CopyFile(documentPath, outputLocation)
	}
}

func maybeReformatDocument(ctx context.Context, documentPath string, rootStep *workflowTracking.WorkflowStep) (string, bool, error) {
	content, err := os.ReadFile(documentPath)
	if err != nil {
		log.From(ctx).Warnf("Failed to read document: %v", err)
		return documentPath, false, err
	}

	// Check if the file is only a single line
	if bytes.Count(content, []byte("\n")) == 0 {
		reformatStep := rootStep.NewSubstep("Reformatting Single-Line Document")

		returnErr := func(err error) (string, bool, error) {
			log.From(ctx).Warnf("Failed to reformat document: %v", err)
			reformatStep.Fail()
			return documentPath, false, err
		}

		isJSON := json.Valid(content)

		reformattedContent, err := schemas.Format(ctx, documentPath, !isJSON)
		if err != nil {
			return returnErr(fmt.Errorf("failed to format document: %w", err))
		}

		// Write reformatted content to a new temporary file
		if err := os.MkdirAll(workflow.GetTempDir(), os.ModePerm); err != nil {
			return returnErr(fmt.Errorf("failed to create temp dir: %w", err))
		}
		tempFile, err := os.CreateTemp(workflow.GetTempDir(), "reformatted*"+filepath.Ext(documentPath))
		if err != nil {
			return returnErr(fmt.Errorf("failed to create temporary file: %w", err))
		}
		defer tempFile.Close()

		if _, err := tempFile.Write(reformattedContent); err != nil {
			return returnErr(fmt.Errorf("failed to write reformatted content: %w", err))
		}

		reformatStep.Succeed()
		log.From(ctx).Infof("Document reformatted and saved to: %s", tempFile.Name())
		return tempFile.Name(), true, nil
	}

	return documentPath, false, nil
}

// maybeConvertSwagger checks if a document is Swagger 2.0 and automatically converts it to OpenAPI 3.0
func maybeConvertSwagger(ctx context.Context, rootStep *workflowTracking.WorkflowStep, documentPath string, logger log.Logger) (string, error) {
	isSwagger, err := schemas.IsSwaggerDocument(ctx, documentPath)
	if err != nil {
		logger.Warnf("failed to check if document is Swagger: %s", err.Error())
		return documentPath, nil
	}

	if !isSwagger {
		return documentPath, nil
	}

	convertStep := rootStep.NewSubstep("Converting Swagger 2.0 to OpenAPI 3.0")
	logger.Infof("Detected Swagger 2.0 document, automatically converting to OpenAPI 3.0...")

	convertedPath := getTempConvertedPath(documentPath)
	if err := os.MkdirAll(filepath.Dir(convertedPath), os.ModePerm); err != nil {
		convertStep.Fail()
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}

	convertedFile, err := os.Create(convertedPath)
	if err != nil {
		convertStep.Fail()
		return "", fmt.Errorf("failed to create converted file: %w", err)
	}
	defer convertedFile.Close()

	yamlOut := utils.HasYAMLExt(documentPath)
	if err := transform.ConvertSwagger(ctx, documentPath, yamlOut, convertedFile); err != nil {
		convertStep.Fail()
		return "", fmt.Errorf("failed to convert Swagger to OpenAPI: %w", err)
	}

	convertStep.Succeed()
	return convertedPath, nil
}
