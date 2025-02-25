package studio

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/AlekSi/pointer"
	"github.com/samber/lo"
	vErrs "github.com/speakeasy-api/openapi-generation/v2/pkg/errors"
	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/speakeasy-api/speakeasy-core/errors"
	"github.com/speakeasy-api/speakeasy/internal/run"
	"github.com/speakeasy-api/speakeasy/internal/schemas"
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

	_, sourceResult, err := workflowRunner.RunSource(ctx, workflowRunner.RootStep, sourceID, "")
	if err != nil {
		return nil, fmt.Errorf("error running source: %w", err)
	}

	return sourceResult, nil
}

func convertSourceResultIntoSourceResponseData(sourceID string, sourceResult run.SourceResult, overlayPath string) (*components.SourceResponseData, error) {
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
			diagnosis = append(diagnosis, components.Diagnostic{
				Message:  vErr.Message,
				Severity: string(vErr.Severity),
				Line:     pointer.ToInt64(int64(vErr.LineNumber)),
				Type:     vErr.Rule,
			})
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
