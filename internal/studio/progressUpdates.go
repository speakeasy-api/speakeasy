package studio

import (
	"fmt"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/run"
	"github.com/speakeasy-api/speakeasy/internal/sdkgen"
	"github.com/speakeasy-api/speakeasy/internal/studio/sdk/models/components"
	"net/http"
)

func (h *StudioHandlers) enableGenerationProgressUpdates(w http.ResponseWriter, flusher http.Flusher, updateSteps bool) {
	workflowConfig := h.WorkflowRunner.GetWorkflowFile()
	workflow, _ := convertWorkflowToComponentsWorkflow(*workflowConfig, h.WorkflowRunner.ProjectDir)

	onProgressUpdate := func(progressUpdate sdkgen.ProgressUpdate) {
		targetID := progressUpdate.TargetID
		targetConfig, found := workflowConfig.Targets[targetID]
		if !found {
			return // TODO handle error
		}

		if h.WorkflowRunner.Debug {
			h.logGenerationProgress(progressUpdate)
		}

		var step run.SourceStepID
		targetResults := make(map[string]components.TargetRunSummary)

		switch {
		case progressUpdate.Step != nil:

			switch progressUpdate.Step.ID {
			case "genSDK":
				step = run.SourceStepGenerate
			case "compileSDK":
				step = run.SourceStepCompile
			default:
				return
			}

		case progressUpdate.File != nil && progressUpdate.File.IsMainReadme && progressUpdate.File.Content != nil:
			step = run.SourceStepReadme

			readme := components.FileData{
				Name:    "README.md",
				Path:    progressUpdate.File.Path,
				Content: string(progressUpdate.File.Content.Bytes()),
			}

			targetDirectory := h.WorkflowRunner.ProjectDir
			if targetConfig.Output != nil {
				targetDirectory = *targetConfig.Output
			}

			targetResults[targetID] = components.TargetRunSummary{
				TargetID:        targetID,
				SourceID:        h.SourceID,
				Readme:          &readme,
				Language:        targetConfig.Target,
				OutputDirectory: targetDirectory,
			}
		default:
			return
		}

		runResponseData := components.RunResponseData{
			TargetResults:    targetResults,
			WorkingDirectory: h.WorkflowRunner.ProjectDir,
			Workflow:         workflow,
			Step:             components.Step(step),
			IsPartial:        true,
			Took:             h.WorkflowRunner.Duration.Milliseconds(), // ?
		}
		sendRunResponseDataToStream(w, flusher, runResponseData)

	}

	h.WorkflowRunner.StreamableGeneration = &sdkgen.StreamableGeneration{
		OnProgressUpdate: onProgressUpdate,
		UpdateSteps:      updateSteps,
	}
}

func (h *StudioHandlers) disableGenerationProgressUpdates() {
	h.WorkflowRunner.StreamableGeneration = nil
}

func (h *StudioHandlers) logGenerationProgress(progressUpdate sdkgen.ProgressUpdate) {
	logChan := h.WorkflowRunner.StreamableGeneration.LogListener
	if logChan == nil {
		return
	}

	var msg string
	switch {
	case progressUpdate.File != nil:

		msg = fmt.Sprintf(
			"[%s] %s",
			progressUpdate.File.Status,
			progressUpdate.File.Path,
		)

		if progressUpdate.File.IsMainReadme {
			msg += fmt.Sprintf(" - MAIN README")
		}

		if progressUpdate.File.Content != nil {
			msg += fmt.Sprintf(" - got %v bytes", len(progressUpdate.File.Content.Bytes()))
		}

	case progressUpdate.Step != nil:
		msg = fmt.Sprintf("[%s] %s", progressUpdate.Step.ID, progressUpdate.Step.Message)
	}

	logChan <- log.Msg{Type: "studio", Msg: fmt.Sprintf("──STUDIO %s", msg)}
}
