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
	"github.com/speakeasy-api/speakeasy/internal/env"
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

	runMutex        sync.Mutex
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

	ret.runMutex = sync.Mutex{}

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

	err := sendLastRunResultToStream(ctx, w, flusher, h.WorkflowRunner, h.SourceID, h.OverlayPath, run.SourceStepStart)
	if err != nil {
		return fmt.Errorf("error sending last run result to stream: %w", err)
	}

	h.runMutex.Lock()
	defer h.runMutex.Unlock()

	return sendLastRunResultToStream(ctx, w, flusher, h.WorkflowRunner, h.SourceID, h.OverlayPath, run.SourceStepComplete)
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
	r.Body = io.NopCloser(bytes.NewBuffer(body))

	var runRequestBody components.RunRequestBody
	if err := json.NewDecoder(r.Body).Decode(&runRequestBody); err != nil {
		return errors.ErrBadRequest.Wrap(fmt.Errorf("could not decode RunRequestBody: %w", err))
	}

	// Wait for previous run to finish, if any
	h.WorkflowRunner.CancelGeneration()
	h.runMutex.Lock()
	defer h.runMutex.Unlock()

	updatedOverlayPath, err := updateSourceAndTarget(h.WorkflowRunner, h.SourceID, h.OverlayPath, runRequestBody)
	if err != nil {
		return fmt.Errorf("error updating source: %w", err)
	}
	h.OverlayPath = updatedOverlayPath

	options := studioRunnerOptions{
		Cancellable:    !runRequestBody.Disconnect,
		Debug:          env.IsLocalDev(),
		OnSourceResult: onSourceResult(h.Ctx, w, flusher, h.WorkflowRunner, h.SourceID, h.OverlayPath),
	}

	defer func() {
		h.WorkflowRunner.OnSourceResult = NoSourceResultCallback
	}()

	h.WorkflowRunner, err = cloneWorkflowRunner(h.Ctx, h.WorkflowRunner, options, h.SourceID)
	if err != nil {
		return err
	}

	if runRequestBody.Stream != nil {
		h.enableGenerationProgressUpdates(w, flusher, runRequestBody.Stream.GenSteps, runRequestBody.Stream.FileStatus)
		defer h.disableGenerationProgressUpdates()
	}

	err = h.WorkflowRunner.RunWithVisualization(h.Ctx)
	if err != nil {
		fmt.Println("error running workflow:", err)
	}

	if runRequestBody.Disconnect {
		if err = h.Server.Shutdown(ctx); err != nil {
			return fmt.Errorf("error shutting down server: %w", err)
		}

		return nil
	}

	return sendLastRunResultToStream(ctx, w, flusher, h.WorkflowRunner, h.SourceID, h.OverlayPath, run.SourceStepComplete)
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
