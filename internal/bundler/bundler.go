package bundler

import (
	"bytes"
	"context"
	"errors"
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
	"strings"
)

func CompileSource(ctx context.Context, rootStep *workflowTracking.WorkflowStep, sourceId string, source workflow.Source) (string, error) {
	outputLocation, err := source.GetOutputLocation()
	if err != nil {
		return "", err
	}
	//
	//if err := os.MkdirAll(filepath.Dir(outputLocation), os.ModePerm); err != nil {
	//	return "", err
	//}
	//
	//dst, err := os.Create(outputLocation)
	//if err != nil {
	//	return "", fmt.Errorf("error creating output file: %w", err)
	//}
	//
	//defer dst.Close()
	return outputLocation, compileSource(ctx, rootStep, sourceId, source, outputOptions{mainDocOutputPath: outputLocation})
}

func CompileSourceTo(ctx context.Context, rootStep *workflowTracking.WorkflowStep, sourceId string, source workflow.Source, dst io.Writer) error {
	return compileSource(ctx, rootStep, sourceId, source, outputOptions{mainDocDst: dst})
}

type outputOptions struct {
	mainDocOutputPath string    // Supporting documents will be written as siblings to this file
	mainDocDst        io.Writer // Supporting documents will not be written. The output doc is not guaranteed to be valid
}

func compileSource(
	ctx context.Context,
	rootStep *workflowTracking.WorkflowStep,
	sourceId string,
	source workflow.Source,
	opts outputOptions,
) error {
	if len(source.Inputs) == 0 {
		return fmt.Errorf("source %s has no inputs", sourceId)
	}

	if rootStep == nil {
		rootStep = workflowTracking.NewWorkflowStep("ignored", log.From(ctx), nil)
	}

	// We will only use the ruleset to validate the final document
	rulesetToUse := ""
	if source.Ruleset != nil {
		rulesetToUse = *source.Ruleset
	}

	/*
	 * Prepare in memory filesystem and bundler pipeline
	 */

	logger := log.From(ctx)
	memFS := bundler.NewMemFS()
	rwfs := bundler.NewReadWriteFS(memFS, memFS)
	pipeline := bundler.NewPipeline(&bundler.PipelineOptions{
		Logger: slog.New(charmLog.New(logger.GetWriter())),
	})
	pathToValidationResult := map[string]error{}

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

		for _, docLocation := range resolvedDocLocations {
			err = validate(ctx, pipeline, rwfs, docLocation, "")
			pathToValidationResult[docLocation] = err
			if err != nil {
				// Ignore error, only the validation at the end NEEDS to pass
				logger.Error(err.Error())
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
			//BasePath:   bundler.InputsRootPath,
			InputPaths: resolvedDocLocations,
		})
		if err != nil {
			return fmt.Errorf("error merging documents: %w", err)
		}

		// If we don't have overlays, this will be the final document and will be validated at the end
		if len(source.Overlays) > 0 {
			rootStep.NewSubstep("Validating Merged Document")
			err = validate(ctx, pipeline, rwfs, finalDocLocation, "")
			pathToValidationResult[finalDocLocation] = err
			if err != nil {
				// Ignore error, only the validation at the end NEEDS to pass
				logger.Error(err.Error())
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
	 * Localize final document
	 * This step will end up moving the final document and all its referenced files to the output directory,
	 * resulting in a self-contained document.
	 */

	if opts.mainDocOutputPath != "" {
		rootStep.NewSubstep("Localizing final document")

		rwfs = bundler.NewReadWriteFS(memFS, bundler.NewOSTarget())
		finalDocLocation, err = pipeline.Localize(ctx, rwfs, bundler.LocalizeOptions{
			BaseDocumentPath: finalDocLocation,
			UseAbsoluteRefs:  false,
			OutputRoot:       opts.mainDocOutputPath,
		})
		if err != nil {
			return err
		}

		if err = os.Rename(finalDocLocation, opts.mainDocOutputPath); err != nil {
			return fmt.Errorf("error renaming final document: %w", err)
		}

		// Set these for the final validation
		finalDocLocation = opts.mainDocOutputPath
		rwfs = bundler.NewReadWriteFS(os.DirFS("."), bundler.NewOSTarget())
	}

	/*
	 * Validate final document
	 */

	stepName := "Validating OpenAPI document"
	if numMerged > 0 || numOverlaid > 0 {
		stepName = fmt.Sprintf("Validating final document (%d merged, %d overlaid)", numMerged, numOverlaid)
	}
	rootStep.NewSubstep(stepName)

	if err = validate(ctx, pipeline, rwfs, finalDocLocation, rulesetToUse); err != nil {
		inputSpecValidationSummary := constructValidationSummary(pipeline, pathToValidationResult)
		return fmt.Errorf("error validating final OpenAPI document:\n%w\n%s", err, inputSpecValidationSummary)
	}

	/*
	 * Persist final document
	 */

	if opts.mainDocDst != nil {
		rootStep.NewSubstep("Writing final document")

		file, err := rwfs.Open(finalDocLocation)
		if err != nil {
			return fmt.Errorf("error opening final document: %w", err)
		}

		_, err = io.Copy(opts.mainDocDst, file)
		return err
	}

	return nil
}

func validate(ctx context.Context, pipeline *bundler.Pipeline, fs fs.FS, schemaPath, ruleset string) error {
	logger := log.From(ctx)
	schemaPathDisplay := getSchemaPathDisplay(pipeline, schemaPath)
	logger.Info(fmt.Sprintf("Validating OpenAPI document %s...\n", schemaPathDisplay))

	schema, err := fs.Open(schemaPath)
	if err != nil {
		return fmt.Errorf("failed to open schema: %w", err)
	}
	schemaData, err := io.ReadAll(schema)
	if err != nil {
		return fmt.Errorf("failed to read schema: %w", err)
	}

	var out bytes.Buffer
	newCtx := log.With(ctx, logger.WithWriter(&out)) // Don't write every log to the console
	vErrs, vWarns, vInfos, err := validation.Validate(newCtx, schemaData, schemaPath, nil, false, ruleset, "")
	if err != nil {
		return err
	}

	prefixedLogger := logger.WithAssociatedFile(schemaPath).WithFormatter(log.PrefixedFormatter)

	for _, warn := range vWarns {
		prefixedLogger.Warn("", zap.Error(warn))
	}
	for _, err := range vErrs {
		prefixedLogger.Error("", zap.Error(err))
	}

	if len(vErrs) > 0 {
		errs := errors.Join(vErrs...).Error()
		errs = "\t" + strings.ReplaceAll(errs, "\n", "\n\t")
		return fmt.Errorf(fmt.Sprintf("OpenAPI document is invalid with %d error(s) - %s:\n%s", len(vErrs), schemaPathDisplay, errs))
	}

	suffix := ""
	if len(vInfos) > 0 || len(vWarns) > 0 {
		suffix = " (run `speakeasy validate openapi...` to examine them)"
	}
	logger.Success(fmt.Sprintf("OpenAPI document is valid with %d hint(s) and %d warning(s)%s - %s", len(vInfos), len(vWarns), suffix, schemaPathDisplay))

	return nil
}

func constructValidationSummary(pipeline *bundler.Pipeline, validationResults map[string]error) string {
	if len(validationResults) == 0 {
		return ""
	}
	errs := strings.Builder{}
	nonErrs := strings.Builder{}
	for path, err := range validationResults {
		schemaPathDisplay := getSchemaPathDisplay(pipeline, path)
		if err != nil {
			errS := strings.ReplaceAll(err.Error(), "\n", "\n\t\t")
			errs.WriteString(fmt.Sprintf("\tInput spec was invalid - %s:\n\t\t%s\n", schemaPathDisplay, errS))
		} else {
			nonErrs.WriteString(fmt.Sprintf("\tInput spec was valid - %s\n", schemaPathDisplay))
		}
	}

	return fmt.Sprintf("Input specs validation summary:\n%s%s", nonErrs.String(), errs.String())
}

func getSchemaPathDisplay(pipeline *bundler.Pipeline, path string) string {
	originalPath := ""
	if pipeline != nil {
		if p := pipeline.GetSourcePath(path); p != "" {
			originalPath = fmt.Sprintf(" (source: %s)", p)
		}
	}
	return fmt.Sprintf("%s%s", path, originalPath)
}
