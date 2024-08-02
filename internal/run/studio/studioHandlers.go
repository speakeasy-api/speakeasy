package studio

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/speakeasy-api/jsonpath/pkg/overlay"
	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/speakeasy-api/speakeasy-core/errors"
	"github.com/speakeasy-api/speakeasy/internal/run"
	"github.com/speakeasy-api/speakeasy/internal/run/studio/generated-studio-sdk/models/components"
	"github.com/speakeasy-api/speakeasy/internal/utils"
	"gopkg.in/yaml.v2"
)

type StudioHandlers struct {
	WorkflowRunner run.Workflow
	SourceID       string
	OverlayPath    string
}

func NewStudioHandlers(workflow *run.Workflow) (StudioHandlers, error) {
	ret := StudioHandlers{WorkflowRunner: *workflow}

	sourceID, err := findWorkflowSourceIDBasedOnTarget(*workflow, workflow.Target)
	if err != nil {
		return ret, fmt.Errorf("error finding source: %w", err)
	}
	if sourceID == "" {
		return ret, errors.New("unable to find source")
	}
	ret.SourceID = sourceID

	return ret, nil
}

func (h *StudioHandlers) health(ctx context.Context, w http.ResponseWriter, r *http.Request) error {

	workflow, err := convertWorkflowToComponentsWorkflow(*h.WorkflowRunner.GetWorkflowFile())
	if err != nil {
		return fmt.Errorf("error converting workflow to components.Workflow: %w", err)
	}

	ret := components.HealthResponse{
		Workflow:         workflow,
		TargetID:         h.WorkflowRunner.Target,
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

	workflowRunner := h.WorkflowRunner
	workflowConfig := workflowRunner.GetWorkflowFile()
	sourceID := h.SourceID

	prevSkipLinting := workflowRunner.SkipLinting
	workflowRunner.SkipLinting = true
	outputDocument, runSourceResult, err := workflowRunner.RunSource(ctx, workflowRunner.RootStep, sourceID, "", false)
	workflowRunner.SkipLinting = prevSkipLinting

	if err != nil {
		return fmt.Errorf("error running source: %w", err)
	}

	outputDocumentString, err := utils.ReadFileToString(outputDocument)
	if err != nil {
		return fmt.Errorf("error reading output document: %w", err)
	}

	source := workflowConfig.Sources[sourceID]
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

func (h *StudioHandlers) updateSource(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	var err error

	// Destructure the request body which is a json object with a single key "overlay" which is a string
	var reqBody struct {
		Overlay string `json:"overlay"`
	}
	err = json.NewDecoder(r.Body).Decode(&reqBody)
	if err != nil {
		return errors.ErrBadRequest.Wrap(fmt.Errorf("error decoding request body: %w", err))
	}

	// Verify this is a valid overlay
	var overlay overlay.Overlay
	dec := yaml.NewDecoder(strings.NewReader(reqBody.Overlay))
	err = dec.Decode(&overlay)
	if err != nil {
		return errors.ErrBadRequest.Wrap(fmt.Errorf("error decoding overlay: %w", err))
	}

	// Write the overlay to a file
	err = h.getOrCreateOverlayPath()
	if err != nil {
		return errors.ErrBadRequest.Wrap(fmt.Errorf("error getting or creating overlay path: %w", err))
	}
	err = utils.WriteStringToFile(h.OverlayPath, reqBody.Overlay)
	if err != nil {
		return errors.ErrBadRequest.Wrap(fmt.Errorf("error writing overlay to file: %w", err))
	}

	return h.getSource(ctx, w, r)
}

// ========================================
// Helper functions
// ========================================

func (h *StudioHandlers) getOrCreateOverlayPath() error {
	if h.OverlayPath != "" {
		return nil
	}

	for i := 0; i < 100; i++ {
		x := ""
		if i > 0 {
			x = fmt.Sprintf("-%d", i)
		}
		path := "./speakeasy-modifications-overlay" + x + ".yaml"
		_, err := os.Stat(path)
		if os.IsNotExist(err) {
			h.OverlayPath = path
			workflowConfig := h.WorkflowRunner.GetWorkflowFile()

			source := workflowConfig.Sources[h.SourceID]

			source.Overlays = append(source.Overlays, workflow.Overlay{
				Document: &workflow.Document{
					Location: path,
				},
			})
			workflowConfig.Sources[h.SourceID] = source
			workflow.Save(h.WorkflowRunner.ProjectDir, workflowConfig)
			return nil
		}
	}

	return errors.New("unable to create overlay file")
}

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
