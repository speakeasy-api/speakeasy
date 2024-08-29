package run

import (
	"context"
	"fmt"
	"github.com/speakeasy-api/openapi-generation/v2/pkg/errors"
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

func (w *Workflow) retryWithMinimumViableSpec(ctx context.Context, parentStep *workflowTracking.WorkflowStep, sourceID, targetID string, vErrs []error) (string, *SourceResult, error) {
	invalidOperationToErr := make(map[string]error)
	for _, err := range vErrs {
		vErr := errors.GetValidationErr(err)
		for _, op := range vErr.AffectedOperationIDs {
			invalidOperationToErr[op] = err // TODO: support multiple errors per operation?
		}
	}

	substep := parentStep.NewSubstep("Retrying with minimum viable document")
	source := w.workflow.Sources[sourceID]
	baseLocation := source.Inputs[0].Location
	workingDir := workflow.GetTempDir()

	// This is intended to only be used from quickstart, we must assume a singular input document
	if len(source.Inputs)+len(source.Overlays) > 1 {
		return "", nil, fmt.Errorf("multiple inputs are not supported for minimum viable spec")
	}

	tempBase := fmt.Sprintf("downloaded_%s%s", randStringBytes(10), filepath.Ext(baseLocation))

	if source.Inputs[0].IsRemote() {
		outResolved, err := download.ResolveRemoteDocument(ctx, source.Inputs[0], tempBase)
		if err != nil {
			return "", nil, fmt.Errorf("failed to download remote document: %w", err)
		}

		baseLocation = outResolved
	}

	overlayOut := filepath.Join(workingDir, fmt.Sprintf("mvs_overlay_%s.yaml", randStringBytes(10)))
	if err := os.MkdirAll(workingDir, os.ModePerm); err != nil {
		return "", nil, err
	}
	overlayFile, err := os.Create(overlayOut)
	if err != nil {
		return "", nil, fmt.Errorf("failed to create overlay file: %w", err)
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
		return "", nil, fmt.Errorf("failed to load document: %w", err)
	}

	overlay := transform.BuildRemoveInvalidOperationsOverlay(model, invalidOperationToErr)
	if err = modifications.UpsertOverlay(&source, overlay); err != nil {
		return "", nil, fmt.Errorf("failed to upsert overlay: %w", err)
	}

	w.workflow.Sources[sourceID] = source

	sourcePath, sourceRes, err := w.RunSource(ctx, substep, sourceID, targetID)
	if err != nil {
		failedRetry = true
		return "", nil, fmt.Errorf("failed to re-run source: %w", err)
	}

	if err := workflow.Save(w.ProjectDir, &w.workflow); err != nil {
		return "", nil, fmt.Errorf("failed to save workflow: %w", err)
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
