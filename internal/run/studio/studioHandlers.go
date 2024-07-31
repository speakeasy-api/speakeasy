package studio

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/speakeasy-api/speakeasy-core/errors"
	"github.com/speakeasy-api/speakeasy/internal/run"
	"github.com/speakeasy-api/speakeasy/internal/run/studio/generated-studio-sdk/models/components"
	"github.com/speakeasy-api/speakeasy/internal/utils"
)

type StudioHandlers struct {
	Workflow run.Workflow
}

func (h *StudioHandlers) health(ctx context.Context, w http.ResponseWriter, r *http.Request) error {

	workflow, err := convertWorkflowToComponentsWorkflow(*h.Workflow.GetWorkflowFile())
	if err != nil {
		return fmt.Errorf("error converting workflow to components.Workflow: %w", err)
	}

	ret := components.HealthResponse{
		Workflow:         workflow,
		TargetID:         h.Workflow.Target,
		WorkingDirectory: os.Getenv("PWD"),
	}
	err = json.NewEncoder(w).Encode(ret)
	if err != nil {
		return fmt.Errorf("error encoding health response: %w", err)
	}

	return nil
}

func (h *StudioHandlers) getSource(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	var err error

	workflow := h.Workflow
	workflowFile := workflow.GetWorkflowFile()

	sourceID, err := findWorkflowSourceIDBasedOnTarget(workflow, workflow.Target)
	if err != nil {
		return errors.ErrBadRequest.Wrap(fmt.Errorf("error finding source: %w", err))
	}
	if sourceID == "" {
		return errors.New("unable to find source")
	}

	outputDocument, runSourceResult, err := workflow.RunSource(ctx, workflow.RootStep, sourceID, "", true)

	if err != nil {
		return errors.ErrBadRequest.Wrap(fmt.Errorf("error running source: %w", err))
	}

	outputDocumentString, err := utils.ReadFileToString(outputDocument)
	if err != nil {
		return errors.ErrBadRequest.Wrap(fmt.Errorf("error reading output document: %w", err))
	}

	source := workflowFile.Sources[sourceID]
	overlayContents := ""
	for _, overlay := range source.Overlays {
		contents, _ := isStudioModificationsOverlay(overlay)
		if contents != "" {
			overlayContents = contents
			break
		}
	}

	ret := components.SourceResponse{
		SourceID: sourceID,
		Input:    runSourceResult.InputSpec,
		Overlay:  overlayContents,
		Output:   outputDocumentString,
	}

	_ = json.NewEncoder(w).Encode(ret)

	return nil
}

func (self *StudioHandlers) updateSource(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	// TODO: Implement this
	return nil
}

// ====  Helpers ====

func convertWorkflowToComponentsWorkflow(w workflow.Workflow) (components.Workflow, error) {
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
	isLocalFile := overlay.Document != nil && !strings.HasPrefix(overlay.Document.Location, "https://") && !strings.HasPrefix(overlay.Document.Location, "http://") && !strings.HasPrefix(overlay.Document.Location, "registry.speakeasyapi.dev")
	if !isLocalFile {
		return "", nil
	}

	asString, err := utils.ReadFileToString(overlay.Document.Location)

	if err != nil {
		return "", err
	}

	looksLikeStudioModifications := strings.Contains(asString, "x-speakeasy-modification")
	if !looksLikeStudioModifications {
		return "", nil
	}

	return asString, nil
}
