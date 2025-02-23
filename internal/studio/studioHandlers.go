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

	"github.com/samber/lo"
	"github.com/speakeasy-api/openapi-overlay/pkg/loader"
	sdkGenConfig "github.com/speakeasy-api/sdk-gen-config"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/schemas"

	"github.com/speakeasy-api/openapi-overlay/pkg/overlay"

	"github.com/AlekSi/pointer"
	vErrs "github.com/speakeasy-api/openapi-generation/v2/pkg/errors"
	"github.com/speakeasy-api/speakeasy-core/errors"
	"github.com/speakeasy-api/speakeasy/internal/studio/modifications"

	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/speakeasy-api/speakeasy-core/events"
	"github.com/speakeasy-api/speakeasy/internal/run"
	"github.com/speakeasy-api/speakeasy/internal/studio/sdk/models/components"
	"github.com/speakeasy-api/speakeasy/internal/studio/sdk/models/operations"
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

	err := h.sendLastRunResultToStream(ctx, w, flusher, "Generating SDK")
	if err != nil {
		return fmt.Errorf("error sending last run result to stream: %w", err)
	}

	h.mutexCondition.L.Lock()
	for h.running {
		h.mutexCondition.Wait()
	}
	defer h.mutexCondition.L.Unlock()

	return h.sendLastRunResultToStream(ctx, w, flusher, "Complete")
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
			sourceResponse, err := convertSourceResultIntoSourceResponse(h.SourceID, *sourceResult, h.OverlayPath)
			if err != nil {
				// TODO: How to handle this error and exit the parent function?
				fmt.Println("error converting source result to source response:", err)
				return
			}

			if step == "" {
				step = "Generating SDK"
			}
			response, err := h.convertLastRunResult(ctx, step)
			if err != nil {
				fmt.Println("error getting last completed run result:", err)
				return
			}
			response.SourceResult = *sourceResponse

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

	return h.sendLastRunResultToStream(ctx, w, flusher, "Complete")
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
		err = h.upsertOverlay(overlay)
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
	var requestBody operations.GenerateOverlayRequestBody

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

	var response operations.GenerateOverlayResponseBody
	response.Overlay = string(resBytes)

	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		return errors.ErrBadRequest.Wrap(fmt.Errorf("error encoding response: %w", err))
	}

	return nil
}

func (h *StudioHandlers) suggestMethodNames(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	sourceResult, err := h.runSource()
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

// ---------------------------------
// Helper functions
// ---------------------------------

func (h *StudioHandlers) convertLastRunResult(ctx context.Context, step string) (*components.RunResponse, error) {
	ret := components.RunResponse{
		TargetResults:    make(map[string]components.TargetRunSummary),
		WorkingDirectory: h.WorkflowRunner.ProjectDir,
		Step:             step,
		IsPartial:        step != "Complete",
		Took:             h.WorkflowRunner.Duration.Milliseconds(),
	}

	wf, err := convertWorkflowToComponentsWorkflow(*h.WorkflowRunner.GetWorkflowFile(), ret.WorkingDirectory)
	if err != nil {
		return &ret, fmt.Errorf("error converting workflow to components.Workflow: %w", err)
	}
	ret.Workflow = wf

	if h.WorkflowRunner.Error != nil {
		errStr := h.WorkflowRunner.Error.Error()
		ret.Error = &errStr
	}

	for k, v := range h.WorkflowRunner.TargetResults {
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

		workflowConfig := h.WorkflowRunner.GetWorkflowFile()
		targetConfig := workflowConfig.Targets[k]

		outputDirectory := ""

		if targetConfig.Output != nil {
			outputDirectory = *targetConfig.Output
		} else {
			outputDirectory = ret.WorkingDirectory
		}
		ret.TargetResults[k] = components.TargetRunSummary{
			TargetID:        k,
			SourceID:        h.SourceID,
			Readme:          readMeContents,
			GenYaml:         genYamlContents,
			GenYamlPath:     &absGenYamlPath,
			Language:        targetConfig.Target,
			OutputDirectory: outputDirectory,
		}
	}

	sourceResult := h.WorkflowRunner.SourceResults[h.SourceID]
	if sourceResult != nil {
		sourceResponse, err := convertSourceResultIntoSourceResponse(h.SourceID, *sourceResult, h.OverlayPath)
		if err != nil {
			return &ret, fmt.Errorf("error converting source result to source response: %w", err)
		}
		ret.SourceResult = *sourceResponse
	}

	return &ret, nil
}

func (h *StudioHandlers) sendLastRunResultToStream(ctx context.Context, w http.ResponseWriter, flusher http.Flusher, step string) error {
	ret, err := h.convertLastRunResult(ctx, step)
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

func (h *StudioHandlers) runSource() (*run.SourceResult, error) {
	workflowRunner := h.WorkflowRunner
	sourceID := h.SourceID

	workflowRunnerPtr, err := workflowRunner.Clone(h.Ctx,
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

	_, sourceResult, err := workflowRunner.RunSource(h.Ctx, workflowRunner.RootStep, sourceID, "")
	if err != nil {
		return nil, fmt.Errorf("error running source: %w", err)
	}

	return sourceResult, nil
}

func convertSourceResultIntoSourceResponse(sourceID string, sourceResult run.SourceResult, overlayPath string) (*components.SourceResponse, error) {
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

	return &components.SourceResponse{
		SourceID:    sourceID,
		Input:       string(formattedInputSpec),
		Overlay:     overlayContents,
		OverlayPath: finalOverlayPath,
		Output:      outputDocumentString,
		Diagnosis:   diagnosis,
	}, nil
}

func (h *StudioHandlers) upsertOverlay(overlay overlay.Overlay) error {
	workflowConfig := h.WorkflowRunner.GetWorkflowFile()
	source := workflowConfig.Sources[h.SourceID]

	if h.OverlayPath == "" {
		var err error
		h.OverlayPath, err = modifications.GetOverlayPath(h.WorkflowRunner.ProjectDir)
		if err != nil {
			return err
		}
	}

	overlayPath, err := modifications.UpsertOverlay(h.OverlayPath, &source, overlay)
	if err != nil {
		return err
	}

	h.OverlayPath = overlayPath

	workflowConfig.Sources[h.SourceID] = source

	return workflow.Save(h.WorkflowRunner.ProjectDir, workflowConfig)
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
