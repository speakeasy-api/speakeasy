package studio

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/speakeasy-api/speakeasy-core/openapi"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/AlekSi/pointer"
	vErrs "github.com/speakeasy-api/openapi-generation/v2/pkg/errors"
	"github.com/speakeasy-api/speakeasy-client-sdk-go/v3/pkg/models/shared"
	"github.com/speakeasy-api/speakeasy-core/errors"

	"github.com/speakeasy-api/jsonpath/pkg/overlay"
	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/speakeasy-api/speakeasy-core/events"
	"github.com/speakeasy-api/speakeasy/internal/run"
	"github.com/speakeasy-api/speakeasy/internal/studio/sdk/models/components"
	"github.com/speakeasy-api/speakeasy/internal/suggest"
	"github.com/speakeasy-api/speakeasy/internal/utils"
	"gopkg.in/yaml.v2"
)

type StudioHandlers struct {
	WorkflowRunner run.Workflow
	SourceID       string
	OverlayPath    string
	Ctx            context.Context
}

const (
	ModificationsOverlayPath = ".speakeasy/speakeasy-modifications-overlay.yaml"
)

func NewStudioHandlers(ctx context.Context, workflow *run.Workflow) (*StudioHandlers, error) {
	ret := &StudioHandlers{WorkflowRunner: *workflow, Ctx: ctx}

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

func (h *StudioHandlers) getRun(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
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

	if len(h.WorkflowRunner.SourceResults) != 1 {
		return errors.New("expected exactly one source")
	}

	sourceResult := h.WorkflowRunner.SourceResults[h.SourceID]
	sourceResponse, err := convertSourceResultIntoSourceResponse(h.SourceID, *sourceResult)
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

func (h *StudioHandlers) updateRun(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	cloned, err := h.WorkflowRunner.Clone(h.Ctx, run.WithSkipCleanup())
	if err != nil {
		return fmt.Errorf("error cloning workflow runner: %w", err)
	}
	h.WorkflowRunner = *cloned
	err = h.WorkflowRunner.Run(h.Ctx)
	if err != nil {
		return fmt.Errorf("error running workflow: %w", err)
	}

	return h.getRun(ctx, w, r)
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

func (h *StudioHandlers) getSource(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	var err error

	sourceID := h.SourceID
	sourceResult, err := h.getSourceInner(ctx)
	if err != nil {
		return fmt.Errorf("error running source: %w", err)
	}

	ret, err := convertSourceResultIntoSourceResponse(sourceID, *sourceResult)
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

	return h.getSource(ctx, w, r)
}

func (h *StudioHandlers) suggestMethodNames(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	sourceResult, err := h.getSourceInner(ctx)
	if err != nil {
		return fmt.Errorf("error running source: %w", err)
	}

	_, model, err := openapi.Load([]byte(sourceResult.InputSpec), sourceResult.OutputPath)
	if err != nil {
		return fmt.Errorf("error loading document: %w", err)
	}

	_, overlay, err := suggest.SuggestOperationIDs(h.Ctx, []byte(sourceResult.InputSpec), model.Model, shared.StyleResource, shared.DepthStyleOriginal)
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

func (h *StudioHandlers) getSourceInner(ctx context.Context) (*run.SourceResult, error) {
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

func convertSourceResultIntoSourceResponse(sourceID string, sourceResult run.SourceResult) (*components.SourceResponse, error) {
	overlayContents, err := utils.ReadFileToString(ModificationsOverlayPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("error reading modifications overlay: %w", err)
		}
	}

	outputDocumentString, err := utils.ReadFileToString(sourceResult.OutputPath)
	if err != nil {
		return nil, fmt.Errorf("error reading output document: %w", err)
	}

	var diagnosis []components.Diagnostic

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

	// TODO: this probably doesn't need to be stored in h.OverlayPath? Do we need to support custom modification filepaths?
	h.OverlayPath = ModificationsOverlayPath

	workflowConfig := h.WorkflowRunner.GetWorkflowFile()

	source := workflowConfig.Sources[h.SourceID]

	if !slices.ContainsFunc(source.Overlays, func(o workflow.Overlay) bool { return o.Document.Location == h.OverlayPath }) {
		source.Overlays = append(source.Overlays, workflow.Overlay{
			Document: &workflow.Document{
				Location: h.OverlayPath,
			},
		})
		workflowConfig.Sources[h.SourceID] = source
		return workflow.Save(h.WorkflowRunner.ProjectDir, workflowConfig)
	}

	return nil
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
