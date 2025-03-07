package studio

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"

	"github.com/speakeasy-api/openapi-overlay/pkg/loader"
	"github.com/speakeasy-api/openapi-overlay/pkg/overlay"
	"github.com/speakeasy-api/speakeasy-core/errors"
	"github.com/speakeasy-api/speakeasy-core/events"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/run"
	"github.com/speakeasy-api/speakeasy/internal/studio/modifications"
	"github.com/speakeasy-api/speakeasy/internal/studio/sdk/models/components"
	"github.com/speakeasy-api/speakeasy/internal/suggest"
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

	// If the client disconnected already, save ourselves the trouble
	if ctx.Err() != nil {
		return ctx.Err()
	}

	body, _ := io.ReadAll(r.Body)
	fmt.Println("> > >Received JSON payload:")
	fmt.Println(string(body))
	r.Body = io.NopCloser(bytes.NewBuffer(body))

	var runRequestBody components.RunRequestBody
	if err := json.NewDecoder(r.Body).Decode(&runRequestBody); err != nil {
		return errors.ErrBadRequest.Wrap(fmt.Errorf("could not decode RunRequestBody: %w", err))
	}

	// Wait for the run to finish
	h.cancelRun(ctx, w, r)
	h.mutexCondition.L.Lock()
	h.running = true
	defer func() {
		h.running = false
		h.mutexCondition.Broadcast()
		h.mutexCondition.L.Unlock()
	}()

	updatedOverlayPath, err := updateSourceAndTarget(h.WorkflowRunner, h.SourceID, h.OverlayPath, runRequestBody)
	if err != nil {
		return fmt.Errorf("error updating source: %w", err)
	}
	h.OverlayPath = updatedOverlayPath

	cloned, err := h.WorkflowRunner.Clone(
		h.Ctx,
		run.WithSkipCleanup(),
		run.WithLinting(),
		run.WithSkipGenerateLintReport(),
		run.WithSkipSnapshot(true),
		run.WithSkipChangeReport(true),
		run.WithShouldCompile(runRequestBody.Compile),
		run.WithCancellableGeneration(),
	)
	if err != nil {
		return fmt.Errorf("error cloning workflow runner: %w", err)
	}
	h.WorkflowRunner = *cloned

	if runRequestBody.Stream != nil {
		h.enableGenerationProgressUpdates(runRequestBody.Stream.Steps)
		defer h.disableGenerationProgressUpdates()
	}

	// RunSource updates
	h.WorkflowRunner.OnSourceResult = func(sourceResult *run.SourceResult, step string) {
		if sourceResult.Source == h.SourceID {
			sourceResponseData, err := convertSourceResultIntoSourceResponseData(*sourceResult, h.SourceID, h.OverlayPath)
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

func (h *StudioHandlers) cancelRun(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	return h.WorkflowRunner.CancelGeneration()
}
