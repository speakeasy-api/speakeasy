package run

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/speakeasy-api/speakeasy-core/errors"
	"github.com/speakeasy-api/speakeasy/internal/run/generated-studio-sdk/models/components"
	"github.com/speakeasy-api/speakeasy/internal/utils"
)

type StudioHandlers struct {
	Workflow Workflow
}

func (h *StudioHandlers) health(ctx context.Context, w http.ResponseWriter, r *http.Request) error {

	workflow, err := convertWorkflowToComponentsWorkflow(h.Workflow.workflow)
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

	source, err := findWorkflowSourceBasedOnTarget(workflow, workflow.Target)
	if err != nil {
		return errors.ErrBadRequest.Wrap(fmt.Errorf("error finding source: %w", err))
	}
	if source == "" {
		return errors.New("unable to find source")
	}

	outputDocument, runSourceResult, err := workflow.runSource(ctx, workflow.RootStep, source, "", true)

	if err != nil {
		return errors.ErrBadRequest.Wrap(fmt.Errorf("error running source: %w", err))
	}

	outputDocumentString, err := utils.ReadFileToString(outputDocument)
	if err != nil {
		return errors.ErrBadRequest.Wrap(fmt.Errorf("error reading output document: %w", err))
	}

	ret := components.SourceResponse{
		SourceID: source,
		Input:    runSourceResult.InputSpec,
		Overlay:  runSourceResult.StudioModificationsOverlayContents,
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

func findWorkflowSourceBasedOnTarget(workflow Workflow, targetID string) (string, error) {
	if workflow.Source != "" {
		return workflow.Source, nil
	}

	if targetID == "" {
		return "", errors.ErrBadRequest.Wrap(fmt.Errorf("no source or target provided"))
	}

	if targetID == "all" {
		// If we're running multiple targets that's fine as long as they all have the same source
		for _, t := range workflow.workflow.Targets {
			if t.Source == "" {
				return "", errors.ErrBadRequest.Wrap(fmt.Errorf("all targets must have the same source"))
			}
		}
		return workflow.workflow.Targets[targetID].Source, nil
	}

	t, ok := workflow.workflow.Targets[targetID]
	if !ok {
		return "", errors.ErrBadRequest.Wrap(fmt.Errorf("target %s not found", targetID))
	}

	return t.Source, nil
}
