package run

import (
	"context"
	"fmt"
	"strings"

	"github.com/speakeasy-api/openapi/pointer"
	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/speakeasy-api/speakeasy/internal/charm/styles"
	"github.com/speakeasy-api/speakeasy/internal/studio/modifications"
	"github.com/speakeasy-api/speakeasy/internal/validation"
	"github.com/speakeasy-api/speakeasy/internal/workflowTracking"
)

func (w *Workflow) retryWithMinimumViableSpec(ctx context.Context, parentStep *workflowTracking.WorkflowStep, sourceID, targetID string, res *validation.ValidationResult) (string, *SourceResult, error) {
	targetLanguage := w.workflow.Targets[targetID].Target
	var invalidOperations []string

	invalidOperations = res.InvalidOperations

	substep := parentStep.NewSubstep("Retrying with minimum viable document")
	source := w.workflow.Sources[sourceID]

	if len(invalidOperations) > 0 {
		source.Transformations = append(source.Transformations, workflow.Transformation{
			FilterOperations: &workflow.FilterOperationsOptions{
				Operations: strings.Join(invalidOperations, ","),
				Exclude:    pointer.From(true),
			},
		})
	} else {
		// Sometimes the document has invalid, unused sections
		source.Transformations = append(source.Transformations, workflow.Transformation{
			RemoveUnused: pointer.From(true),
		})
	}
	w.workflow.Sources[sourceID] = source

	sourcePath, sourceRes, err := w.RunSource(ctx, substep, sourceID, targetID, targetLanguage)
	if err != nil {
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
