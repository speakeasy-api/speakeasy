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

func CompileSource(ctx context.Context, rootStep *workflowTracking.WorkflowStep, sourceId string, source workflow.Source) (string, error) {
	outputLocation, err := source.GetOutputLocation()
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(filepath.Dir(outputLocation), os.ModePerm); err != nil {
		return "", err
	}

	dst, err := os.Create(outputLocation)
	if err != nil {
		return "", fmt.Errorf("error creating output file: %w", err)
	}

	defer dst.Close()
	return outputLocation, CompileSourceTo(ctx, rootStep, sourceId, source, dst)
}

func CompileSourceTo(ctx context.Context, rootStep *workflowTracking.WorkflowStep, sourceId string, source workflow.Source, dst io.Writer) error {
	if len(source.Inputs) == 0 {
		return fmt.Errorf("source %s has no inputs", sourceId)
	}
	// We will only use the ruleset to validate the final document
	rulesetToUse := ""
	if source.Ruleset != nil {
		rulesetToUse = *source.Ruleset
	}

	// outputLocation will be the same as the input location if it's a single local file with no overlays
	// In that case, we only need to run validation
	if outputLocation, _ := source.GetOutputLocation(); outputLocation == source.Inputs[0].Location {
		rootStep.NewSubstep("Validating OpenAPI document")
		absPath, err := filepath.Abs(source.Inputs[0].Location)
		if err != nil {
			return err
		}
		dir, base := filepath.Split(absPath)
		return validate(ctx, os.DirFS(dir), base, rulesetToUse)
	}

	/*
	 * Prepare in memory filesystem and bundler pipeline
	 */

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

	// Only validate here if we are going to merge or overlay
	if len(source.Inputs) > 1 || len(source.Overlays) > 0 {
		rootStep.NewSubstep("Validating OpenAPI document(s)")

		for _, doc := range resolvedDocLocations {
			if err := validate(ctx, rwfs, doc, ""); err != nil {
				return fmt.Errorf("error validating input OpenAPI documents: %w", err)
			}
		}
	}

	/*
	 * Merge input docs
	 */

	finalDocLocation := resolvedDocLocations[0]

	numMerged := 0
	if len(source.Inputs) > 1 {
		numMerged = len(source.Inputs)
		rootStep.NewSubstep(fmt.Sprintf("Merging %d documents", numMerged))

		finalDocLocation, err = pipeline.Merge(ctx, rwfs, bundler.MergeOptions{
			BasePath:   bundler.InputsRootPath,
			InputPaths: resolvedDocLocations,
		})
		if err != nil {
			return fmt.Errorf("error merging documents: %w", err)
		}

		// If we don't have overlays, this will be the final document and will be validated at the end
		if len(source.Overlays) > 0 {
			rootStep.NewSubstep("Validating Merged Document")
			if err := validate(ctx, rwfs, finalDocLocation, ""); err != nil {
				return fmt.Errorf("error validating merged OpenAPI document: %w", err)
			}
		}
	}

	/*
	 * Fetch and apply overlays, if there are any
	 */

	numOverlaid := 0
	if len(source.Overlays) > 0 {
		numOverlaid = len(source.Overlays)
		overlayStep := rootStep.NewSubstep(fmt.Sprintf("Detected %d overlay(s)", numOverlaid))

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

		overlayStep.SucceedWorkflow()
	}

	/*
	 * Validate final document
	 */

	stepName := "Validating OpenAPI document"
	if numMerged > 0 || numOverlaid > 0 {
		stepName = fmt.Sprintf("Validating final document (%d merged, %d overlaid)", numMerged, numOverlaid)
	}
	rootStep.NewSubstep(stepName)

	if err = validate(ctx, rwfs, finalDocLocation, rulesetToUse); err != nil {
		return fmt.Errorf("error validating final OpenAPI document: %w", err)
	}

	/*
	 * Persist final document
	 */

	rootStep.NewSubstep("Writing final document")

	file, err := rwfs.Open(finalDocLocation)
	if err != nil {
		return fmt.Errorf("error opening final document: %w", err)
	}

	_, err = io.Copy(dst, file)
	return err
}

func validate(ctx context.Context, fs fs.FS, schemaPath, ruleset string) error {
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
		MaxWarns: 20,
	}

	vErrs, vWarns, _, err := validation.Validate(ctx, schemaData, schemaPath, limits, false, ruleset, "")
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

	logger.Success(fmt.Sprintf("Successfully validated %s", schemaPath))

	return nil
}
