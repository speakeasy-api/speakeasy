package studio

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/speakeasy-api/openapi-overlay/pkg/loader"
	"github.com/speakeasy-api/openapi-overlay/pkg/overlay"
	sdkGenConfig "github.com/speakeasy-api/sdk-gen-config"
	"github.com/speakeasy-api/speakeasy-core/errors"
	"github.com/speakeasy-api/speakeasy-core/events"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/run"
	"github.com/speakeasy-api/speakeasy/internal/studio/modifications"
	"github.com/speakeasy-api/speakeasy/internal/studio/sdk/models/components"
	"github.com/speakeasy-api/speakeasy/internal/suggest"
	"github.com/speakeasy-api/speakeasy/internal/utils"
	"gopkg.in/yaml.v3"
)

type StudioHandlers struct {
	WorkflowRunner run.Workflow
	SourceID       string
	OverlayPath    string
	Ctx            context.Context
	StudioURL      string
	Server         *http.Server

	mutex           sync.Mutex
	mutexCondition  *sync.Cond
	running         bool
	healthCheckSeen bool
}

func NewStudioHandlers(ctx context.Context, workflowRunner *run.Workflow) (*StudioHandlers, error) {
	ret := &StudioHandlers{WorkflowRunner: *workflowRunner, Ctx: ctx}

	sourceID, err := findWorkflowSourceIDBasedOnTarget(*workflowRunner, workflowRunner.Target)
	if err != nil {
		return ret, fmt.Errorf("error finding source: %w", err)
	}
	if sourceID == "" {
		return ret, errors.New("unable to find source")
	}
	ret.SourceID = sourceID
	sourceConfig := workflowRunner.GetWorkflowFile().Sources[sourceID]

	for _, overlay := range sourceConfig.Overlays {
		// If there are multiple modifications overlays - we take the last one
		contents, _ := isStudioModificationsOverlay(overlay)
		if contents != "" {
			ret.OverlayPath = overlay.Document.Location.Resolve()
		}
	}

	ret.mutex = sync.Mutex{}
	ret.mutexCondition = sync.NewCond(&ret.mutex)

	return ret, nil
}

func (h *StudioHandlers) getLastRunResult(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, ok := w.(http.Flusher)
	if !ok {
		return errors.New("streaming unsupported")
	}

	err := sendLastRunResultToStream(ctx, w, flusher, h.WorkflowRunner, h.SourceID, h.OverlayPath, "Generating SDK")
	if err != nil {
		return fmt.Errorf("error sending last run result to stream: %w", err)
	}

	h.mutexCondition.L.Lock()
	for h.running {
		h.mutexCondition.Wait()
	}
	defer h.mutexCondition.L.Unlock()

	return sendLastRunResultToStream(ctx, w, flusher, h.WorkflowRunner, h.SourceID, h.OverlayPath, "Complete")
}

func (h *StudioHandlers) reRun(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, ok := w.(http.Flusher)
	if !ok {
		return errors.New("streaming unsupported")
	}

	// Wait for the run to finish
	h.mutexCondition.L.Lock()
	for h.running {
		h.mutexCondition.Wait()
	}

	h.running = true
	defer func() {
		h.running = false
		h.mutexCondition.Broadcast()
		h.mutexCondition.L.Unlock()
	}()

	// If the client disconnected already, save ourselves the trouble
	if ctx.Err() != nil {
		return ctx.Err()
	}

	err := h.updateSourceAndTarget(r)
	if err != nil {
		return fmt.Errorf("error updating source: %w", err)
	}

	cloned, err := h.WorkflowRunner.Clone(
		h.Ctx,
		run.WithSkipCleanup(),
		run.WithLinting(),
		run.WithSkipGenerateLintReport(),
		run.WithSkipSnapshot(true),
		run.WithSkipChangeReport(true),
	)
	if err != nil {
		return fmt.Errorf("error cloning workflow runner: %w", err)
	}
	h.WorkflowRunner = *cloned

	h.WorkflowRunner.OnSourceResult = func(sourceResult *run.SourceResult, step string) {
		if sourceResult.Source == h.SourceID {
			sourceResponseData, err := convertSourceResultIntoSourceResponseData(h.SourceID, *sourceResult, h.OverlayPath)
			if err != nil {
				// TODO: How to handle this error and exit the parent function?
				fmt.Println("error converting source result to source response:", err)
				return
			}

			if step == "" {
				step = "Generating SDK"
			}
			response, err := convertLastRunResult(ctx, h.WorkflowRunner, h.SourceID, h.OverlayPath, step)
			if err != nil {
				fmt.Println("error getting last completed run result:", err)
				return
			}
			response.SourceResult = *sourceResponseData

			responseJSON, err := json.Marshal(response)
			if err != nil {
				fmt.Println("error marshaling run response:", err)
				return
			}

			fmt.Fprintf(w, "event: message\ndata: %s\n\n", responseJSON)
			flusher.Flush()
		}
	}
	defer func() {
		h.WorkflowRunner.OnSourceResult = func(*run.SourceResult, string) {}
	}()

	err = h.WorkflowRunner.RunWithVisualization(h.Ctx)
	if err != nil {
		fmt.Println("error running workflow:", err)
	}

	return sendLastRunResultToStream(ctx, w, flusher, h.WorkflowRunner, h.SourceID, h.OverlayPath, "Complete")
}

func (h *StudioHandlers) health(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	h.healthCheckSeen = true

	flusher, ok := w.(http.Flusher)
	if !ok {
		return errors.New("streaming unsupported")
	}

	response := map[string]string{"status": "ok", "version": events.GetSpeakeasyVersionFromContext(h.Ctx)}

	responseJSON, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("error marshaling health response: %w", err)
	}

	fmt.Fprintf(w, "event: message\ndata: %s\n\n", responseJSON)
	flusher.Flush()

	// This keeps the connection open while the client is still connected
	<-ctx.Done()

	return nil
}

func (h *StudioHandlers) root(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	// In case the user navigates to the root of the studio, redirect them to the studio URL
	http.Redirect(w, r, h.StudioURL, http.StatusSeeOther)
	return nil
}

func (h *StudioHandlers) updateSourceAndTarget(r *http.Request) error {
	var err error

	type target struct {
		ID     string `json:"id"`
		Config string `json:"config"`
	}

	// Destructure the request body which is a json object with a single key "overlay" which is a string
	var reqBody struct {
		Overlay string                                     `json:"overlay"`
		Input   string                                     `json:"input"`
		Targets map[string]components.TargetSpecificInputs `json:"targets"` // this is only present if a target input is modified
	}
	err = json.NewDecoder(r.Body).Decode(&reqBody)
	if err != nil {
		return errors.ErrBadRequest.Wrap(fmt.Errorf("error decoding request body: %w", err))
	}

	if reqBody.Input != "" {
		// Assert that the workflow source input is a single local file
		workflowConfig := h.WorkflowRunner.GetWorkflowFile()
		source := workflowConfig.Sources[h.SourceID]
		if len(source.Inputs) != 1 {
			return errors.ErrBadRequest.Wrap(fmt.Errorf("cannot update source input for source with multiple inputs"))
		}
		if strings.HasPrefix(reqBody.Input, "http://") || strings.HasPrefix(reqBody.Input, "https://") {
			return errors.ErrBadRequest.Wrap(fmt.Errorf("cannot update source input to a remote file"))
		}

		inputLocation := source.Inputs[0].Location.Resolve()

		// if it's absolute that's fine, otherwise it's relative to the project directory
		if !filepath.IsAbs(inputLocation) {
			inputLocation = filepath.Join(h.WorkflowRunner.ProjectDir, inputLocation)
		}

		err = utils.WriteStringToFile(inputLocation, reqBody.Input)
		if err != nil {
			return errors.ErrBadRequest.Wrap(fmt.Errorf("error writing input to file: %w", err))
		}
	}

	if reqBody.Overlay != "" {
		// Verify this is a valid overlay
		var overlay overlay.Overlay
		dec := yaml.NewDecoder(strings.NewReader(reqBody.Overlay))
		err = dec.Decode(&overlay)
		if err != nil {
			return errors.ErrBadRequest.Wrap(fmt.Errorf("error decoding overlay: %w", err))
		}

		// Write the overlay to a file
		h.OverlayPath, err = upsertOverlay(overlay, h.WorkflowRunner, h.SourceID, h.OverlayPath)
		if err != nil {
			return errors.ErrBadRequest.Wrap(fmt.Errorf("error getting or creating overlay path: %w", err))
		}
	}

	for targetID, input := range reqBody.Targets {
		sdkPath := ""

		wfTarget, ok := h.WorkflowRunner.GetWorkflowFile().Targets[targetID]
		if !ok {
			return errors.ErrBadRequest.Wrap(fmt.Errorf("target %s not found", targetID))
		}
		sdkPath = h.WorkflowRunner.ProjectDir
		if wfTarget.Output != nil {
			sdkPath = filepath.Join(sdkPath, *wfTarget.Output)
		}

		cfg, err := sdkGenConfig.Load(sdkPath)
		if err != nil {
			return errors.ErrBadRequest.Wrap(fmt.Errorf("error loading config file: %w", err))
		}

		currentFileContent, err := os.ReadFile(cfg.ConfigPath)
		if err != nil {
			return errors.ErrBadRequest.Wrap(fmt.Errorf("error loading config file: %w", err))
		}

		err = utils.WriteStringToFile(cfg.ConfigPath, input.Config)
		if err != nil {
			return errors.ErrBadRequest.Wrap(fmt.Errorf("error writing input to file: %w", err))
		}

		if _, err := sdkGenConfig.Load(sdkPath); err != nil {
			err = utils.WriteStringToFile(cfg.ConfigPath, string(currentFileContent))
			return errors.ErrBadRequest.Wrap(fmt.Errorf("invalid config file changes rolling back: %w", err))
		}
	}

	return nil
}

func (h *StudioHandlers) compareOverlay(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	var requestBody components.OverlayCompareRequestBody

	err := json.NewDecoder(r.Body).Decode(&requestBody)
	if err != nil {
		return errors.ErrBadRequest.Wrap(fmt.Errorf("error decoding request body: %w", err))
	}

	var before yaml.Node
	var after yaml.Node

	err = yaml.Unmarshal([]byte(requestBody.Before), &before)
	if err != nil {
		return errors.ErrBadRequest.Wrap(fmt.Errorf("error unmarshalling before overlay: %w", err))
	}
	err = yaml.Unmarshal([]byte(requestBody.After), &after)
	if err != nil {
		return errors.ErrBadRequest.Wrap(fmt.Errorf("error unmarshalling after overlay: %w", err))
	}

	res, err := overlay.Compare("Studio Overlay Diff", &before, after)
	if err != nil {
		return errors.ErrBadRequest.Wrap(fmt.Errorf("error comparing overlays: %w", err))
	}

	resBytes, err := yaml.Marshal(res)
	if err != nil {
		return errors.ErrBadRequest.Wrap(fmt.Errorf("error marshalling response: %w", err))
	}

	var response components.OverlayCompareResponse
	response.Overlay = string(resBytes)

	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		return errors.ErrBadRequest.Wrap(fmt.Errorf("error encoding response: %w", err))
	}

	return nil
}

func (h *StudioHandlers) suggestMethodNames(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	sourceResult, err := runSource(h.Ctx, h.WorkflowRunner, h.SourceID)
	if err != nil {
		return fmt.Errorf("error running source: %w", err)
	}

	specBytes, err := os.ReadFile(sourceResult.OutputPath)
	if err != nil {
		return fmt.Errorf("error reading output spec: %w", err)
	}

	suggestOverlay, err := suggest.SuggestOperationIDs(h.Ctx, specBytes, sourceResult.OutputPath)
	if err != nil {
		return fmt.Errorf("error suggesting method names: %w", err)
	}

	if h.OverlayPath != "" {
		existingOverlay, err := loader.LoadOverlay(h.OverlayPath)
		if err != nil {
			log.From(ctx).Warnf("error loading existing overlay: %s", err.Error())
		} else {
			// Theoretically this shouldn't be necessary anymore because suggestions are filtered by suggest.SuggestOperationIDs
			suggestOverlay.Actions = modifications.RemoveAlreadySuggested(existingOverlay.Actions, suggestOverlay.Actions)
		}
	}

	yamlBytes, err := yaml.Marshal(suggestOverlay)
	if err != nil {
		return fmt.Errorf("error marshaling overlay to yaml: %w", err)
	}
	overlayAsYaml := string(yamlBytes)

	data := components.SuggestResponse{Overlay: overlayAsYaml}

	err = json.NewEncoder(w).Encode(data)
	if err != nil {
		return fmt.Errorf("error encoding method name suggestions: %w", err)
	}

	return nil
}

func (h *StudioHandlers) exit(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	err := h.Server.Shutdown(ctx)
	if err != nil {
		return fmt.Errorf("error shutting down server: %w", err)
	}
	return nil
}
