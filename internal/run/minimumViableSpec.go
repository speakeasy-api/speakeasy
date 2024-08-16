package run

import (
	"context"
	"errors"
	"fmt"
	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/speakeasy-api/speakeasy-core/openapi"
	"github.com/speakeasy-api/speakeasy/internal/charm/styles"
	"github.com/speakeasy-api/speakeasy/internal/download"
	"github.com/speakeasy-api/speakeasy/internal/studio/modifications"
	"github.com/speakeasy-api/speakeasy/internal/transform"
	"github.com/speakeasy-api/speakeasy/internal/workflowTracking"
	"os"
	"path/filepath"
)

func (w *Workflow) retryWithMinimumViableSpec(ctx context.Context, parentStep *workflowTracking.WorkflowStep, sourceID, targetID string, viableOperations []string) (string, *SourceResult, error) {
	substep := parentStep.NewSubstep("Retrying with minimum viable document")
	source := w.workflow.Sources[sourceID]
	baseLocation := source.Inputs[0].Location
	workingDir := workflow.GetTempDir()

	// This is intended to only be used from quickstart, we must assume a singular input document
	if len(source.Inputs)+len(source.Overlays) > 1 {
		return "", nil, errors.New("multiple inputs are not supported for minimum viable spec")
	}

	tempBase := fmt.Sprintf("downloaded_%s%s", randStringBytes(10), filepath.Ext(baseLocation))

	if source.Inputs[0].IsRemote() {
		outResolved, err := download.ResolveRemoteDocument(ctx, source.Inputs[0], tempBase)
		if err != nil {
			return "", nil, err
		}

		baseLocation = outResolved
	}

	overlayOut := filepath.Join(workingDir, fmt.Sprintf("mvs_overlay_%s.yaml", randStringBytes(10)))
	overlayFile, err := os.Create(overlayOut)
	if err != nil {
		return "", nil, err
	}
	defer overlayFile.Close()

	failedRetry := false
	defer func() {
		os.Remove(overlayOut)
		os.Remove(filepath.Join(workingDir, tempBase))
		if failedRetry {
			source.Overlays = []workflow.Overlay{}
			w.workflow.Sources[sourceID] = source
		}
	}()

	_, _, model, err := openapi.LoadDocument(ctx, source.Inputs[0].Location)
	if err != nil {
		return "", nil, err
	}

	overlay := transform.BuildFilterOperationsOverlay(model, true, viableOperations)
	if err = modifications.UpsertOverlay(&source, overlay); err != nil {
		return "", nil, err
	}

	w.workflow.Sources[sourceID] = source

	sourcePath, sourceRes, err := w.RunSource(ctx, substep, sourceID, targetID)
	if err != nil {
		failedRetry = true
		return "", nil, err
	}

	if err := workflow.Save(w.ProjectDir, &w.workflow); err != nil {
		return "", nil, err
	}

	return sourcePath, sourceRes, err
}

func groupInvalidOperations(input []string) []string {
	var result []string
	for _, op := range input[0:7] {
		joined := styles.DimmedItalic.Render(fmt.Sprintf("- %s", op))
		result = append(result, joined)
	}

	if len(input) > 7 {
		result = append(result, styles.DimmedItalic.Render(fmt.Sprintf("- ... see %s", modifications.OverlayPath)))
	}

	return result
}
