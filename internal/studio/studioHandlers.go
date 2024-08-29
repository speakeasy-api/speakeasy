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

	"github.com/AlekSi/pointer"
	vErrs "github.com/speakeasy-api/openapi-generation/v2/pkg/errors"
	"github.com/speakeasy-api/speakeasy-core/errors"
	"github.com/speakeasy-api/speakeasy-core/openapi"
	"github.com/speakeasy-api/speakeasy/internal/studio/modifications"

	"github.com/speakeasy-api/jsonpath/pkg/overlay"
	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/speakeasy-api/speakeasy-client-sdk-go/v3/pkg/models/shared"
	"github.com/speakeasy-api/speakeasy-core/events"
	"github.com/speakeasy-api/speakeasy/internal/run"
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
	mutex          sync.Mutex
	mutexCondition *sync.Cond
	running        bool
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
			ret.OverlayPath = overlay.Document.Location
		}
	}

	ret.mutex = sync.Mutex{}
	ret.mutexCondition = sync.NewCond(&ret.mutex)

	return ret, nil
}

func (h *StudioHandlers) getLastCompletedRunResult(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	h.mutexCondition.L.Lock()
	for h.running {
		h.mutexCondition.Wait()
	}
	defer h.mutexCondition.L.Unlock()

	return h.getLastCompletedRunResultInner(ctx, w, r)
}

func (h *StudioHandlers) reRun(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
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

	cloned, err := h.WorkflowRunner.Clone(h.Ctx, run.WithSkipCleanup(), run.WithLinting())
	if err != nil {
		return fmt.Errorf("error cloning workflow runner: %w", err)
	}
	h.WorkflowRunner = *cloned
	err = h.WorkflowRunner.Run(h.Ctx)
	if err != nil {
		return fmt.Errorf("error running workflow: %w", err)
	}

	ret := h.getLastCompletedRunResultInner(ctx, w, r)

	return ret
}

func (h *StudioHandlers) health(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		return errors.New("streaming unsupported")
	}

	response := map[string]string{"status": "ok", "version": events.GetSpeakeasyVersionFromContext(h.Ctx)}

	responseJSON, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("error marshaling health response: %w", err)
	}

	fmt.Fprintf(w, "data: %s\n\n", responseJSON)
	flusher.Flush()

	// Wait for the context to be done
	<-ctx.Done()

	return nil
}

func (h *StudioHandlers) getLastCompletedSourceResult(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	var err error

	sourceID := h.SourceID
	sourceResult, err := h.runSource(ctx)
	if err != nil {
		return fmt.Errorf("error running source: %w", err)
	}

	ret, err := convertSourceResultIntoSourceResponse(sourceID, *sourceResult, h.OverlayPath)
	if err != nil {
		return fmt.Errorf("error converting source result to source response: %w", err)
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

	return h.getLastCompletedSourceResult(ctx, w, r)
}

func (h *StudioHandlers) suggestMethodNames(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	sourceResult, err := h.runSource(ctx)
	if err != nil {
		return fmt.Errorf("error running source: %w", err)
	}

	specBytes, err := os.ReadFile(sourceResult.OutputPath)
	if err != nil {
		return fmt.Errorf("error reading output spec: %w", err)
	}
	specPath := sourceResult.OutputPath

	_, model, err := openapi.Load(specBytes, specPath)
	if err != nil {
		return fmt.Errorf("error loading document: %w", err)
	}

	_, overlay, err := suggest.SuggestOperationIDs(h.Ctx, specBytes, model.Model, shared.StyleResource, shared.DepthStyleOriginal)
	if err != nil {
		return fmt.Errorf("error suggesting method names: %w", err)
	}

	yamlBytes, err := yaml.Marshal(overlay)
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

// ---------------------------------
// Helper functions
// ---------------------------------

func (h *StudioHandlers) getLastCompletedRunResultInner(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	res := components.RunResponse{
		TargetResults: make(map[string]components.TargetRunSummary),
	}

	res.TargetResults = make(map[string]components.TargetRunSummary)

	for k, v := range h.WorkflowRunner.TargetResults {
		genYamlContents, err := utils.ReadFileToString(v.GenYamlPath)
		if err != nil {
			return fmt.Errorf("error reading gen.yaml: %w", err)
		}
		readMePath := filepath.Join(v.OutputPath, "README.md")
		readMeContents, err := utils.ReadFileToString(readMePath)
		if err != nil {
			return fmt.Errorf("error reading gen.yaml: %w", err)
		}

		workflowConfig := h.WorkflowRunner.GetWorkflowFile()
		targetConfig := workflowConfig.Targets[k]

		outputDirectory := ""

		if targetConfig.Output != nil {
			// TODO: Otherwise the current directory
			outputDirectory = *targetConfig.Output
		}
		res.TargetResults[k] = components.TargetRunSummary{
			TargetID:        k,
			SourceID:        h.SourceID,
			Readme:          readMeContents,
			GenYaml:         genYamlContents,
			Language:        targetConfig.Target,
			OutputDirectory: outputDirectory,
		}
	}

	if len(h.WorkflowRunner.SourceResults) == 0 {
		return errors.New("source failed to run")
	}

	if len(h.WorkflowRunner.SourceResults) != 1 {
		return errors.New("expected exactly one source")
	}

	sourceResult := h.WorkflowRunner.SourceResults[h.SourceID]
	sourceResponse, err := convertSourceResultIntoSourceResponse(h.SourceID, *sourceResult, h.OverlayPath)
	if err != nil {
		return fmt.Errorf("error converting source result to source response: %w", err)
	}
	res.SourceResult = *sourceResponse

	wf, err := convertWorkflowToComponentsWorkflow(*h.WorkflowRunner.GetWorkflowFile())
	if err != nil {
		return fmt.Errorf("error converting workflow to components.Workflow: %w", err)
	}
	res.Workflow = wf

	err = json.NewEncoder(w).Encode(res)
	if err != nil {
		return fmt.Errorf("error encoding getRun response: %w", err)
	}

	return nil
}

func (h *StudioHandlers) runSource(ctx context.Context) (*run.SourceResult, error) {
	workflowRunner := h.WorkflowRunner
	sourceID := h.SourceID

	workflowRunnerPtr, err := workflowRunner.Clone(h.Ctx, run.WithSkipLinting(), run.WithSkipCleanup())
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
	if err != nil {
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

	return &components.SourceResponse{
		SourceID:  sourceID,
		Input:     sourceResult.InputSpec,
		Overlay:   overlayContents,
		Output:    outputDocumentString,
		Diagnosis: diagnosis,
	}, nil
}

func (h *StudioHandlers) getOrCreateOverlayPath() error {
	if h.OverlayPath != "" {
		return nil
	}

	// Look for an unused filename for writing the overlay
	overlayPath := filepath.Join(h.WorkflowRunner.ProjectDir, modifications.OverlayPath)
	for i := 0; i < 100; i++ {
		if _, err := os.Stat(overlayPath); os.IsNotExist(err) {
			break
		}
		// Remove the .yaml suffix and add a number
		overlayPath = filepath.Join(h.WorkflowRunner.ProjectDir, fmt.Sprintf("%s-%d.yaml", modifications.OverlayPath[:len(modifications.OverlayPath)-5], i+1))
	}
	h.OverlayPath = overlayPath

	workflowConfig := h.WorkflowRunner.GetWorkflowFile()

	source := workflowConfig.Sources[h.SourceID]

	relativeOverlayPath, err := filepath.Rel(h.WorkflowRunner.ProjectDir, overlayPath)
	if err != nil {
		return fmt.Errorf("error getting relative path: %w", err)
	}

	modifications.UpsertOverlayIntoSource(&source, relativeOverlayPath)

	workflowConfig.Sources[h.SourceID] = source

	return workflow.Save(h.WorkflowRunner.ProjectDir, workflowConfig)
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
	isLocalFile := overlay.Document != nil &&
		!strings.HasPrefix(overlay.Document.Location, "https://") &&
		!strings.HasPrefix(overlay.Document.Location, "http://") &&
		!strings.HasPrefix(overlay.Document.Location, "registry.speakeasyapi.dev")
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
