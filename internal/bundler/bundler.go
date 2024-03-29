package bundler

import (
	"context"
	"fmt"
	charmLog "github.com/charmbracelet/log"
	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/speakeasy-api/speakeasy-core/bundler"
	"github.com/speakeasy-api/speakeasy/internal/env"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/validation"
	"github.com/speakeasy-api/speakeasy/internal/workflowTracking"
	"go.uber.org/zap"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

func CompileSource(ctx context.Context, rootStep *workflowTracking.WorkflowStep, source workflow.Source) error {
	memFS := bundler.NewMemFS()
	rwfs := bundler.NewReadWriteFS(memFS, memFS)
	pipeline := bundler.NewPipeline(&bundler.PipelineOptions{
		Logger: slog.New(charmLog.New(log.From(ctx).GetWriter())),
	})

	/*
	 * Fetch input docs
	 */

	rootStep.NewSubstep("Loading OpenAPI document(s)")

	// Account for GitHub action secrets
	for i, doc := range source.Inputs {
		if doc.Auth != nil && doc.Auth.Secret != "" && env.IsGithubAction() {
			envVar := strings.TrimPrefix(doc.Auth.Secret, "$")

			// GitHub action secrets are prefixed with INPUT_
			envVar = "INPUT_" + envVar
			source.Inputs[i].Auth.Secret = envVar
		}
	}

	resolvedDocLocations, err := pipeline.FetchDocumentsLocalFS(ctx, rwfs, bundler.FetchDocumentsOptions{
		SourceFSBasePath: ".",
		OutputRoot:       bundler.InputsRootPath,
		Documents:        source.Inputs,
	})
	if err != nil || len(resolvedDocLocations) == 0 {
		return fmt.Errorf("error loading input OpenAPI documents: %w", err)
	}

	/*
	 * Validate input docs
	 */

	rootStep.NewSubstep("Validating OpenAPI document(s)")

	for _, doc := range resolvedDocLocations {
		if err := validate(ctx, rwfs, doc); err != nil {
			return fmt.Errorf("error validating input OpenAPI documents: %w", err)
		}
	}

	/*
	 * Merge input docs
	 */

	finalDocLocation := resolvedDocLocations[0]
	if len(source.Inputs) > 1 {
		rootStep.NewSubstep(fmt.Sprintf("Merging %d documents", len(source.Inputs)))

		finalDocLocation, err = pipeline.Merge(ctx, rwfs, bundler.MergeOptions{
			BasePath:   bundler.InputsRootPath,
			InputPaths: resolvedDocLocations,
		})
		if err != nil {
			return fmt.Errorf("error merging documents: %w", err)
		}

		rootStep.NewSubstep("Validating Merged Document")
		if err := validate(ctx, rwfs, finalDocLocation); err != nil {
			return fmt.Errorf("error validating merged OpenAPI document: %w", err)
		}
	}

	/*
	 * Fetch and apply overlays, if there are any
	 */

	if len(source.Overlays) > 0 {
		overlayStep := rootStep.NewSubstep(fmt.Sprintf("Detected %d overlay(s)", len(source.Overlays)))

		overlayStep.NewSubstep("Loading overlay documents")

		overlays, err := pipeline.FetchDocumentsLocalFS(ctx, rwfs, bundler.FetchDocumentsOptions{
			SourceFSBasePath: ".",
			OutputRoot:       bundler.OverlaysRootPath,
			Documents:        source.Overlays,
		})
		if err != nil {
			return fmt.Errorf("error fetching overlay documents: %w", err)
		}

		overlayStep.NewSubstep("Applying overlay documents")

		finalDocLocation, err = pipeline.Overlay(ctx, rwfs, bundler.OverlayOptions{
			BaseDocumentPath: finalDocLocation,
			OverlayPaths:     overlays,
		})
		if err != nil {
			return fmt.Errorf("error applying overlays: %w", err)
		}

		overlayStep.NewSubstep("Validating Overlaid Document")
		if err := validate(ctx, rwfs, finalDocLocation); err != nil {
			return fmt.Errorf("error validating overlaid OpenAPI document: %w", err)
		}

		overlayStep.Finalize(true)
	}

	/*
	 * Persist final document
	 */

	rootStep.NewSubstep("Writing final document")

	outputLocation, err := source.GetOutputLocation()
	if err != nil {
		return err
	}

	if err = os.MkdirAll(filepath.Dir(outputLocation), os.ModePerm); err != nil {
		return err
	}

	dst, err := os.Create(outputLocation)
	if err != nil {
		return fmt.Errorf("error creating output file: %w", err)
	}

	file, err := rwfs.Open(finalDocLocation)
	if err != nil {
		return fmt.Errorf("error opening final document: %w", err)
	}

	_, err = io.Copy(dst, file)
	return err
}

func validate(ctx context.Context, fs fs.FS, schemaPath string) error {
	logger := log.From(ctx)
	logger.Info(fmt.Sprintf("Validating OpenAPI spec %s...\n", schemaPath))

	schema, err := fs.Open(schemaPath)
	if err != nil {
		return fmt.Errorf("failed to open schema: %w", err)
	}
	schemaData, err := io.ReadAll(schema)
	if err != nil {
		return fmt.Errorf("failed to read schema: %w", err)
	}

	prefixedLogger := logger.WithAssociatedFile(schemaPath).WithFormatter(log.PrefixedFormatter)

	limits := &validation.OutputLimits{
		MaxWarns: 10,
	}

	vErrs, vWarns, _, err := validation.Validate(ctx, schemaData, schemaPath, limits, false, "", "")
	if err != nil {
		return err
	}

	for _, warn := range vWarns {
		prefixedLogger.Warn("", zap.Error(warn))
	}
	for _, err := range vErrs {
		prefixedLogger.Error("", zap.Error(err))
	}

	if len(vErrs) > 0 {
		status := "\nOpenAPI spec invalid ✖"
		return fmt.Errorf(status)
	}

	log.From(ctx).Success(fmt.Sprintf("Successfully validated %s", schemaPath))

	return nil
}
