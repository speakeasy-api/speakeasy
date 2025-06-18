package run

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/utils"
	"github.com/speakeasy-api/speakeasy/internal/workflowTracking"
	"github.com/speakeasy-api/speakeasy/pkg/transform"
)

type Transform struct {
	parentStep *workflowTracking.WorkflowStep
	source     workflow.Source
}

var _ SourceStep = Transform{}

func NewTransform(parentStep *workflowTracking.WorkflowStep, source workflow.Source) Transform {
	return Transform{
		parentStep: parentStep,
		source:     source,
	}
}

func (t Transform) Do(ctx context.Context, inputPath string) (string, error) {
	transformStep := t.parentStep.NewSubstep("Applying Transformations")

	outputPath := t.source.GetTempTransformLocation()

	log.From(ctx).Infof("Applying %d transformations and writing to %s...", len(t.source.Transformations), outputPath)

	if err := os.MkdirAll(filepath.Dir(outputPath), os.ModePerm); err != nil {
		return "", err
	}

	yamlOut := utils.HasYAMLExt(outputPath)

	var in io.Reader
	in, err := os.Open(inputPath)
	if err != nil {
		return "", err
	}

	var out *bytes.Buffer
	for _, transformation := range t.source.Transformations {
		out = &bytes.Buffer{}

		if transformation.Cleanup != nil {
			transformStep.NewSubstep("Cleaning up document")

			if err := transform.CleanupFromReader(ctx, in, inputPath, out, yamlOut); err != nil {
				return "", err
			}
		} else if transformation.RemoveUnused != nil {
			transformStep.NewSubstep("Removing unused nodes")

			if err := transform.RemoveUnusedFromReader(ctx, in, inputPath, out, yamlOut); err != nil {
				return "", err
			}
		} else if transformation.FilterOperations != nil {
			operations := transformation.FilterOperations.ParseOperations()
			include := true
			if transformation.FilterOperations.Include != nil {
				include = *transformation.FilterOperations.Include
			} else if transformation.FilterOperations.Exclude != nil {
				include = !*transformation.FilterOperations.Exclude
			}

			inOutString := "down to"
			if !include {
				inOutString = "out"
			}
			transformStep.NewSubstep(fmt.Sprintf("Filtering %s %d operations", inOutString, len(operations)))

			if err := transform.FilterOperationsFromReader(ctx, in, inputPath, operations, include, out, yamlOut); err != nil {
				return "", err
			}
		} else if transformation.Format != nil {
			transformStep.NewSubstep("Formatting document")

			if err := transform.FormatFromReader(ctx, in, inputPath, out, yamlOut); err != nil {
				return "", err
			}
		} else if transformation.Normalize != nil {
			transformStep.NewSubstep("Normalizing document")

			if err := transform.NormalizeFromReader(ctx, in, inputPath, *transformation.Normalize.PrefixItems, out, yamlOut); err != nil {
				return "", err
			}
		}

		in = bytes.NewReader(out.Bytes())
	}

	outFile, err := os.Create(outputPath)
	defer outFile.Close()
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(outFile, out); err != nil {
		return "", err
	}

	transformStep.Succeed()
	return outputPath, nil
}
