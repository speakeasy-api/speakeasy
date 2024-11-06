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
)

type SourceResult struct {
	Source string
	// The merged OAS spec that was input to the source contents as a string
	InputSpec    string
	LintResult   *validation.ValidationResult
	ChangeReport *reports.ReportResult
	Diagnosis    suggestions.Diagnosis
	// The path to the output OAS spec
	OutputPath string
}

type LintingError struct {
	Err      error
	Document string
}

type SourceStep interface {
	Do(ctx context.Context, inputPath, outputPath string) (string, error)
}

func (e *LintingError) Error() string {
	errString := e.Err.Error()
	if strings.Contains(e.Err.Error(), "spec type not supported by libopenapi") {
		errString = "speakeasy supports openapi specs of versions 3.0+ of type yaml and json"
	}
	return fmt.Sprintf("linting failed: %s - %s", e.Document, errString)
}

func (w *Workflow) RunSource(ctx context.Context, parentStep *workflowTracking.WorkflowStep, sourceID, targetID string) (string, *SourceResult, error) {
	rootStep := parentStep.NewSubstep(fmt.Sprintf("Source: %s", sourceID))
	source := w.workflow.Sources[sourceID]
	sourceRes := &SourceResult{
		Source:    sourceID,
		Diagnosis: suggestions.Diagnosis{},
	}
	defer func() {
		w.SourceResults[sourceID] = sourceRes
		w.OnSourceResult(sourceRes, "")
	}()
	w.OnSourceResult(sourceRes, "Overlaying")

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

	var currentDocument string
	if w.FrozenWorkflowLock {
		currentDocument, err = NewFrozenSource(w, rootStep, sourceID).Do(ctx, "unused", "unused")
		if err != nil {
			return "", nil, err
		}
	} else if len(source.Inputs) == 1 {
		var singleLocation *string
		// The output location should be the resolved location
		if len(source.Overlays) == 0 {
			singleLocation = &outputLocation
		}
		currentDocument, err = schemas.ResolveDocument(ctx, source.Inputs[0], singleLocation, rootStep)
		if err != nil {
			return "", nil, err
		}
		if len(source.Overlays) == 0 {
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
	} else {
		currentDocument, err = NewMerge(w, rootStep, source, rulesetToUse).Do(ctx, currentDocument, outputLocation)
		if err != nil {
			return "", nil, err
		}
	}

	sourceRes.InputSpec, err = utils.ReadFileToString(currentDocument)
	if err != nil {
		return "", nil, err
	}

	if len(source.Overlays) > 0 && !w.FrozenWorkflowLock {
		currentDocument, err = NewOverlay(w, rootStep, source, outputLocation, rulesetToUse).Do(ctx, currentDocument, outputLocation)
		if err != nil {
			return "", nil, err
		}
	}

	sourceRes.OutputPath = currentDocument

	if !w.SkipLinting {
		w.OnSourceResult(sourceRes, "Linting")
		sourceRes.LintResult, err = w.validateDocument(ctx, rootStep, sourceID, currentDocument, rulesetToUse, w.ProjectDir)
		if err != nil {
			return "", sourceRes, &LintingError{Err: err, Document: currentDocument}
		}
	}

	step := rootStep.NewSubstep("Diagnosing OpenAPI")
	sourceRes.Diagnosis, err = suggest.Diagnose(ctx, currentDocument)
	if err != nil {
		step.Fail()
		return "", sourceRes, err
	}
	step.Succeed()

	w.OnSourceResult(sourceRes, "Uploading spec")

	if !w.SkipSnapshot {
		err = w.snapshotSource(ctx, rootStep, sourceID, source, currentDocument)
		if err != nil && !errors.Is(err, ocicommon.ErrAccessGated) {
			logger.Warnf("failed to snapshot source: %s", err.Error())
		}
	}

	// If the source has a previous tracked revision, compute changes against it
	if w.lockfileOld != nil && !w.SkipChangeReport {
		if targetLockOld, ok := w.lockfileOld.Targets[targetID]; ok && !utils.IsZeroTelemetryOrganization(ctx) {
			sourceRes.ChangeReport, err = w.computeChanges(ctx, rootStep, targetLockOld, currentDocument)
			if err != nil {
				// Don't fail the whole workflow if this fails
				logger.Warnf("failed to compute OpenAPI changes: %s", err.Error())
			}
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

	res, err := validation.ValidateOpenAPI(ctx, source, schemaPath, "", "", limits, defaultRuleset, projectDir, w.FromQuickstart, w.SkipGenerateLintReport)

	w.validatedDocuments = append(w.validatedDocuments, schemaPath)

	step.SucceedWorkflow()

	return res, err
}

func (w *Workflow) printSourceSuccessMessage(ctx context.Context) {
	if len(w.SourceResults) == 0 {
		return
	}

	logger := log.From(ctx)
	logger.Println("") // Newline for better readability

	for sourceID, sourceRes := range w.SourceResults {
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
		//if sourceRes.Diagnosis != nil && suggest.ShouldSuggest(sourceRes.Diagnosis) {
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
