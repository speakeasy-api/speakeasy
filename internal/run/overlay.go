package run

import (
	"context"
	"fmt"
	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/speakeasy-api/speakeasy/internal/defaultcodesamples"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/schemas"
	"github.com/speakeasy-api/speakeasy/internal/utils"
	"github.com/speakeasy-api/speakeasy/internal/workflowTracking"
	"github.com/speakeasy-api/speakeasy/pkg/overlay"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type Overlay struct {
	parentStep *workflowTracking.WorkflowStep
	source     workflow.Source
}

func NewOverlay(parentStep *workflowTracking.WorkflowStep, source workflow.Source) Overlay {
	return Overlay{
		parentStep: parentStep,
		source:     source,
	}
}

type OverlayResult struct {
	Location            string
	InputSchemaLocation []string
}

func (o Overlay) Do(ctx context.Context, inputPath string) (result OverlayResult, err error) {
	overlayStep := o.parentStep.NewSubstep("Applying Overlays")

	overlayLocation := o.source.GetTempOverlayLocation()
	result.Location = overlayLocation

	log.From(ctx).Infof("Applying %d overlays into %s...", len(o.source.Overlays), overlayLocation)

	var overlaySchemas []string
	for _, overlay := range o.source.Overlays {
		overlayFilePath := ""
		if overlay.Document != nil {
			overlayFilePath, err = schemas.ResolveDocument(ctx, *overlay.Document, nil, overlayStep)
			if err != nil {
				return
			}
		} else if overlay.FallbackCodeSamples != nil {
			// Make temp file for the overlay output
			overlayFilePath = filepath.Join(workflow.GetTempDir(), fmt.Sprintf("fallback_code_samples_overlay_%s.yaml", randStringBytes(10)))
			if err = os.MkdirAll(filepath.Dir(overlayFilePath), 0o755); err != nil {
				return
			}

			err = defaultcodesamples.DefaultCodeSamples(ctx, defaultcodesamples.DefaultCodeSamplesFlags{
				SchemaPath: inputPath,
				Language:   overlay.FallbackCodeSamples.FallbackCodeSamplesLanguage,
				Out:        overlayFilePath,
			})
			if err != nil {
				log.From(ctx).Errorf("failed to generate default code samples: %s", err.Error())
				return
			}
		}

		overlaySchemas = append(overlaySchemas, overlayFilePath)
		result.InputSchemaLocation = append(result.InputSchemaLocation, overlayFilePath)
	}

	overlayStep.NewSubstep(fmt.Sprintf("Apply %d overlay(s)", len(o.source.Overlays)))

	if err = overlayDocument(ctx, inputPath, overlaySchemas, overlayLocation); err != nil {
		return
	}

	overlayStep.Succeed()
	return
}

func overlayDocument(ctx context.Context, schema string, overlayFiles []string, outFile string) error {
	currentBase := schema
	if err := os.MkdirAll(workflow.GetTempDir(), os.ModePerm); err != nil {
		return err
	}

	for _, overlayFile := range overlayFiles {
		applyPath := getTempApplyPath(outFile)

		tempOutFile, err := os.Create(applyPath)
		if err != nil {
			return err
		}

		// YamlOut param needs to be based on the eventual output file
		if _, err := overlay.Apply(currentBase, overlayFile, utils.HasYAMLExt(outFile), tempOutFile, false, false); err != nil && !strings.Contains(err.Error(), "overlay must define at least one action") {
			return err
		}

		if err := tempOutFile.Close(); err != nil {
			return err
		}

		currentBase = applyPath
	}

	if err := os.MkdirAll(filepath.Dir(outFile), os.ModePerm); err != nil {
		return err
	}

	finalTempFile, err := os.Open(currentBase)
	if err != nil {
		return err
	}
	defer finalTempFile.Close()

	outFileWriter, err := os.Create(outFile)
	if err != nil {
		return err
	}
	defer outFileWriter.Close()

	if _, err := io.Copy(outFileWriter, finalTempFile); err != nil {
		return err
	}

	log.From(ctx).Successf("Successfully applied %d overlays into %s", len(overlayFiles), outFile)

	return nil
}
