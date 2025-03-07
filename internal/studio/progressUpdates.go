package studio

import (
	"fmt"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/sdkgen"
	"time"
)

func (h *StudioHandlers) enableGenerationProgressUpdates(updateSteps bool) {

	// v QYNN TEST v
	h.WorkflowRunner.Debug = true
	// ^ QYNN TEST ^

	onProgressUpdate := func(progressUpdate sdkgen.ProgressUpdate) {
		if h.WorkflowRunner.Debug {
			h.logGenerationProgress(progressUpdate)
		}

		// v QYNN TEST v
		time.Sleep(1 * time.Second)
		// ^ QYNN TEST ^
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
			msg += fmt.Sprintf(" -> MAIN README")
		}

		if progressUpdate.File.Content != nil {
			msg += fmt.Sprintf(" -> Content: %v", len(progressUpdate.File.Content.Bytes()))
		}

	case progressUpdate.Step != nil:
		msg = fmt.Sprintf("[%s] %s", progressUpdate.Step.ID, progressUpdate.Step.Message)
	}

	logChan <- log.Msg{Type: "studio", Msg: fmt.Sprintf("──DEBUG %s", msg)}
}
