package studio

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/AlekSi/pointer"
	"github.com/samber/lo"
	vErrs "github.com/speakeasy-api/openapi-generation/v2/pkg/errors"
	"github.com/speakeasy-api/openapi-overlay/pkg/overlay"
	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/speakeasy-api/speakeasy-core/errors"
	"github.com/speakeasy-api/speakeasy/internal/run"
	"github.com/speakeasy-api/speakeasy/internal/schemas"
	"github.com/speakeasy-api/speakeasy/internal/studio/modifications"
	"github.com/speakeasy-api/speakeasy/internal/studio/sdk/models/components"
	"github.com/speakeasy-api/speakeasy/internal/utils"
	"gopkg.in/yaml.v3"
)

func runSource(ctx context.Context, workflowRunner run.Workflow, sourceID string) (*run.SourceResult, error) {
	workflowRunnerPtr, err := workflowRunner.Clone(
		ctx,
		run.WithSkipCleanup(),
		run.WithSkipLinting(),
		run.WithSkipGenerateLintReport(),
		run.WithSkipSnapshot(true),
		run.WithSkipChangeReport(true),
	)
	if err != nil {
		return nil, fmt.Errorf("error cloning workflow runner: %w", err)
	}
	workflowRunner = *workflowRunnerPtr

	_, sourceResult, err := workflowRunner.RunSource(ctx, workflowRunner.RootStep, sourceID, "", "")
	if err != nil {
		return nil, fmt.Errorf("error running source: %w", err)
	}

	return sourceResult, nil
}

func sendLastRunResultToStream(ctx context.Context, w http.ResponseWriter, flusher http.Flusher, workflowRunner run.Workflow, sourceID, overlayPath, step string) error {
	ret, err := convertLastRunResult(ctx, workflowRunner, sourceID, overlayPath, step)
	if err != nil {
		return fmt.Errorf("error getting last completed run result: %w", err)
	}

	responseJSON, err := json.Marshal(ret)
	if err != nil {
		return fmt.Errorf("error marshaling run response: %w", err)
	}
	fmt.Fprintf(w, "event: message\ndata: %s\n\n", responseJSON)
	flusher.Flush()

	return nil
}

func convertLastRunResult(ctx context.Context, workflowRunner run.Workflow, sourceID, overlayPath, step string) (*components.RunResponseData, error) {
	ret := components.RunResponseData{
		TargetResults:    make(map[string]components.TargetRunSummary),
		WorkingDirectory: workflowRunner.ProjectDir,
		Step:             step,
		IsPartial:        step != "Complete",
		Took:             workflowRunner.Duration.Milliseconds(),
	}

	workflowConfig := workflowRunner.GetWorkflowFile()

	wf, err := convertWorkflowToComponentsWorkflow(*workflowConfig, ret.WorkingDirectory)
	if err != nil {
		return &ret, fmt.Errorf("error converting workflow to components.Workflow: %w", err)
	}
	ret.Workflow = wf

	if workflowRunner.Error != nil {
		errStr := workflowRunner.Error.Error()
		ret.Error = &errStr
	}

	for k, v := range workflowRunner.TargetResults {
		if v == nil {
			continue
		}

		genYamlContents, err := utils.ReadFileToString(v.GenYamlPath)
		if err != nil {
			return &ret, fmt.Errorf("error reading gen.yaml: %w", err)
		}
		absGenYamlPath, err := filepath.Abs(v.GenYamlPath)
		if err != nil {
			return &ret, fmt.Errorf("error getting absolute path to gen.yaml: %w", err)
		}
		readMePath := filepath.Join(v.OutputPath, "README.md")
		readMeContents, err := utils.ReadFileToString(readMePath)
		if err != nil {
			return &ret, fmt.Errorf("error reading gen.yaml: %w", err)
		}

		targetConfig := workflowConfig.Targets[k]
		outputDirectory := ""

		if targetConfig.Output != nil {
			outputDirectory = *targetConfig.Output
		} else {
			outputDirectory = ret.WorkingDirectory
		}
		ret.TargetResults[k] = components.TargetRunSummary{
			TargetID:        k,
			SourceID:        sourceID,
			Readme:          readMeContents,
			GenYaml:         genYamlContents,
			GenYamlPath:     &absGenYamlPath,
			Language:        targetConfig.Target,
			OutputDirectory: outputDirectory,
		}
	}

	sourceResult := workflowRunner.SourceResults[sourceID]
	if sourceResult != nil {
		sourceResponseData, err := convertSourceResultIntoSourceResponseData(*sourceResult, sourceID, overlayPath)
		if err != nil {
			return &ret, fmt.Errorf("error converting source result to source response: %w", err)
		}
		ret.SourceResult = *sourceResponseData
	}

	return &ret, nil
}

func convertSourceResultIntoSourceResponseData(sourceResult run.SourceResult, sourceID, overlayPath string) (*components.SourceResponseData, error) {
	var err error
	overlayContents := ""
	if overlayPath != "" {
		overlayContents, err = utils.ReadFileToString(overlayPath)
		if err != nil {
			return nil, fmt.Errorf("error reading modifications overlay: %w", err)
		}
	}

	outputDocumentString, err := utils.ReadFileToString(sourceResult.OutputPath)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("error reading output document: %w", err)
	}

	diagnosis := make([]components.Diagnostic, 0)

	if sourceResult.LintResult != nil {
		for _, e := range sourceResult.LintResult.AllErrors {
			vErr := vErrs.GetValidationErr(e)
			if vErr != nil {
				diagnosis = append(diagnosis, components.Diagnostic{
					Message:  vErr.Message,
					Severity: string(vErr.Severity),
					Line:     pointer.ToInt64(int64(vErr.LineNumber)),
					Type:     vErr.Rule,
				})
				continue
			}
		}
		for _, w := range sourceResult.LintResult.Warnings {
			diagnosis = append(diagnosis, convertWarningToDiagnostic(w))
		}
	}

	for t, d := range sourceResult.Diagnosis {
		for _, diagnostic := range d {
			diagnosis = append(diagnosis, components.Diagnostic{
				Message:  diagnostic.Message,
				Type:     string(t),
				Path:     diagnostic.SchemaPath,
				Severity: "suggestion",
			})
		}
	}

	finalOverlayPath := ""

	if overlayPath != "" {
		finalOverlayPath, _ = filepath.Abs(overlayPath)
	}

	// Reformat the input spec - this is so there are minimal diffs after overlay
	inputSpecBytes := []byte(sourceResult.InputSpec)
	isJSON := json.Valid(inputSpecBytes)
	inputPath := "openapi.yaml"
	if isJSON {
		inputPath = "openapi.json"
	}
	inputNode := &yaml.Node{}
	err = yaml.Unmarshal(inputSpecBytes, inputNode)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling input spec: %w", err)
	}
	formattedInputSpec, err := schemas.RenderDocument(inputNode, inputPath, !isJSON, !isJSON)
	if err != nil {
		return nil, fmt.Errorf("error formatting input spec: %w", err)
	}

	return &components.SourceResponseData{
		SourceID:    sourceID,
		Input:       string(formattedInputSpec),
		Overlay:     overlayContents,
		OverlayPath: finalOverlayPath,
		Output:      outputDocumentString,
		Diagnosis:   diagnosis,
	}, nil
}

func convertWarningToDiagnostic(w error) components.Diagnostic {
	if vErr, ok := w.(*vErrs.ValidationError); ok {
		return components.Diagnostic{
			Message:  vErr.Message,
			Severity: string(vErr.Severity),
			Line:     pointer.ToInt64(int64(vErr.LineNumber)),
			Type:     vErr.Rule,
			Path:     []string{vErr.Path},
		}
	}

	if skippedErr, ok := w.(*vErrs.SkippedError); ok {
		skippedStr := fmt.Sprintf("Skipping path %q", skippedErr.SkippedEntity.Path)
		if skippedErr.SkippedEntity.Operation != "" {
			skippedStr = fmt.Sprintf("Skipping operation %q", skippedErr.SkippedEntity.Operation)
		}
		return components.Diagnostic{
			Message:     skippedErr.Message,
			Line:        pointer.ToInt64(int64(skippedErr.LineNumber)),
			Severity:    "warn",
			Type:        fmt.Sprintf("Skipped %ss", skippedErr.SkippedEntity.GetSpecificEntity()),
			Path:        []string{skippedErr.SkippedEntity.Path},
			HelpMessage: pointer.ToString(skippedStr),
		}
	}

	if uErr, ok := w.(*vErrs.UnsupportedError); ok {
		return components.Diagnostic{
			Message:  uErr.Message,
			Line:     pointer.ToInt64(int64(uErr.LineNumber)),
			Severity: "warn",
			Type:     "Unsupported",
		}
	}

	// TODO: Try to extract the warning type, message, and line number at a minimum
	// parts := strings.Split(w.Error(), ":")
	// if len(parts) == 2 {
	// 	warnType := strings.TrimSpace(parts[0])
	// 	message := strings.TrimSpace(parts[1])
	// 	return components.Diagnostic{
	// 		Message:  message,
	// 		Severity: "warn",
	// 		Type:     warnType,
	// 	}
	// }

	return components.Diagnostic{
		Message:  w.Error(),
		Severity: "warn",
		Type:     "Warnings",
	}
}

func convertWorkflowToComponentsWorkflow(w workflow.Workflow, workingDir string) (components.Workflow, error) {
	// 1. Marshal to JSON
	// 2. Unmarshal to components.Workflow

	jsonBytes, err := json.Marshal(w)
	if err != nil {
		return components.Workflow{}, err
	}

	var c components.Workflow
	err = json.Unmarshal(jsonBytes, &c)
	if err != nil {
		return components.Workflow{}, err
	}

	for key, source := range c.Sources {
		updatedInputs := lo.Map(source.Inputs, func(input components.Document, _ int) components.Document {
			// URL
			if strings.HasPrefix(input.Location, "https://") || strings.HasPrefix(input.Location, "http://") {
				return input
			}
			// Absolute path
			if strings.HasPrefix(input.Location, "/") {
				return input
			}
			// Registry uri
			if strings.HasPrefix(input.Location, "registry.speakeasyapi.dev") {
				return input
			}
			if workingDir == "" {
				return input
			}

			// Produce the lexically shortest path based on the base path and the location
			shortestPath, err := filepath.Rel(workingDir, input.Location)

			if err != nil {
				shortestPath = input.Location
			}

			return components.Document{
				Location: shortestPath,
			}
		})

		source.Inputs = updatedInputs

		source.Overlays = lo.Map(source.Overlays, func(overlay components.Overlay, _ int) components.Overlay {
			// If the overlay is a local file, read the contents
			if overlay.Document.Location != "" &&
				!strings.HasPrefix(overlay.Document.Location, "https://") &&
				!strings.HasPrefix(overlay.Document.Location, "http://") &&
				!strings.HasPrefix(overlay.Document.Location, "registry.speakeasyapi.dev") {
				contents, err := utils.ReadFileToString(overlay.Document.Location)
				if err != nil {
					return overlay
				}

				return components.Overlay{
					Document: &components.OverlayDocument{
						Location: overlay.Document.Location,
						Contents: &contents,
					},
				}
			}

			return overlay
		})

		c.Sources[key] = source
	}

	return c, nil
}

func findWorkflowSourceIDBasedOnTarget(workflow run.Workflow, targetID string) (string, error) {
	if workflow.Source != "" {
		return workflow.Source, nil
	}

	if targetID == "" {
		return "", errors.ErrBadRequest.Wrap(fmt.Errorf("no source or target provided"))
	}

	workflowFile := workflow.GetWorkflowFile()

	if targetID == "all" {
		// If we're running multiple targets that's fine as long as they all have the same source
		source := ""
		for _, t := range workflowFile.Targets {
			if source == "" {
				source = t.Source
			} else if source != t.Source {
				return "", errors.ErrBadRequest.Wrap(fmt.Errorf("multiple targets with different sources"))
			}
		}
		return source, nil
	}

	t, ok := workflowFile.Targets[targetID]
	if !ok {
		return "", errors.ErrBadRequest.Wrap(fmt.Errorf("target %s not found", targetID))
	}

	return t.Source, nil
}

func isStudioModificationsOverlay(overlay workflow.Overlay) (string, error) {
	isLocalFile := overlay.Document != nil &&
		!strings.HasPrefix(overlay.Document.Location.Resolve(), "https://") &&
		!strings.HasPrefix(overlay.Document.Location.Resolve(), "http://") &&
		!strings.HasPrefix(overlay.Document.Location.Resolve(), "registry.speakeasyapi.dev")
	if !isLocalFile {
		return "", nil
	}

	asString, err := utils.ReadFileToString(overlay.Document.Location.Resolve())

	if err != nil {
		return "", err
	}

	looksLikeStudioModifications := strings.Contains(asString, "x-speakeasy-metadata")
	if !looksLikeStudioModifications {
		return "", nil
	}

	return asString, nil
}

func upsertOverlay(overlay overlay.Overlay, workflowRunner run.Workflow, sourceID, overlayPath string) (string, error) {
	if overlayPath == "" {
		var err error
		overlayPath, err = modifications.GetOverlayPath(workflowRunner.ProjectDir)
		if err != nil {
			return overlayPath, err
		}
	}

	workflowConfig := workflowRunner.GetWorkflowFile()
	source := workflowConfig.Sources[sourceID]

	overlayPath, err := modifications.UpsertOverlay(overlayPath, &source, overlay)
	if err != nil {
		return overlayPath, err
	}

	workflowConfig.Sources[sourceID] = source

	return overlayPath, workflow.Save(workflowRunner.ProjectDir, workflowConfig)
}
