package run

import (
	"bytes"
	"context"
	"fmt"
	"github.com/sethvargo/go-githubactions"
	"github.com/speakeasy-api/speakeasy/internal/env"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/speakeasy-api/speakeasy/internal/charm/styles"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/utils"
	"golang.org/x/term"

	"github.com/speakeasy-api/openapi-generation/v2/pkg/generate"
	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/speakeasy-api/speakeasy/internal/config"
	"github.com/speakeasy-api/speakeasy/internal/download"
	"github.com/speakeasy-api/speakeasy/internal/overlay"
	"github.com/speakeasy-api/speakeasy/internal/sdkgen"
	"github.com/speakeasy-api/speakeasy/internal/validation"
	"github.com/speakeasy-api/speakeasy/pkg/merge"
)

func GetWorkflowAndDir() (*workflow.Workflow, string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, "", err
	}

	wf, workflowFileLocation, err := workflow.Load(wd)
	if err != nil {
		return nil, "", err
	}

	// Get the project directory which is the parent of the .speakeasy folder the workflow file is in
	projectDir := filepath.Dir(filepath.Dir(workflowFileLocation))
	if err := os.Chdir(projectDir); err != nil {
		return nil, "", err
	}

	return wf, projectDir, nil
}

func ParseSourcesAndTargets() ([]string, []string, error) {
	wf, _, err := GetWorkflowAndDir()
	if err != nil {
		return nil, nil, err
	}

	if err := wf.Validate(generate.GetSupportedLanguages()); err != nil {
		return nil, nil, err
	}

	targets := []string{}
	for targetID := range wf.Targets {
		targets = append(targets, targetID)
	}
	slices.Sort(targets)

	sources := []string{}
	for sourceID := range wf.Sources {
		sources = append(sources, sourceID)
	}
	slices.Sort(sources)

	return sources, targets, nil
}

func RunWithVisualization(ctx context.Context, target, source, genVersion, installationURL, repo, repoSubDir string, debug bool) error {
	updatesChannel := make(chan UpdateMsg)
	workflow := NewWorkflowStep("Workflow", updatesChannel)

	var logs bytes.Buffer
	var err, runErr error
	logger := log.From(ctx)

	runFnCli := func() error {
		l := logger.WithWriter(&logs) // Swallow logs other than the workflow display
		ctx := context.Background()
		ctx = log.With(ctx, l)
		err = Run(ctx, target, source, genVersion, installationURL, repo, repoSubDir, debug, workflow)

		workflow.Finalize(err == nil)
		if env.IsGithubAction() {
			githubactions.AddStepSummary(workflow.ToMermaidDiagram())
		}

		if err != nil {
			runErr = err
			return err
		}

		return nil
	}

	startTime := time.Now()
	err = workflow.RunWithVisualization(runFnCli, updatesChannel)
	endDuration := time.Since(startTime)
	if err != nil {
		logger.Errorf("Workflow failed with error: %s", err)
	}
	if runErr != nil {
		logger.Errorf("Workflow failed with error: %s\n", runErr)

		termWidth, _, _ := term.GetSize(int(os.Stdout.Fd()))
		style := styles.LeftBorder(styles.Dimmed.GetForeground()).Width(termWidth - 8) // -8 because of padding
		logsHeading := styles.Dimmed.Render("Workflow run logs")
		logger.PrintfStyled(style, "%s\n\n%s", logsHeading, strings.TrimSpace(logs.String()))
	}

	if err == nil && runErr == nil {
		t, err := getTarget(target)
		if err != nil {
			return err
		}
		tOut := "the current directory"
		if t.Output != nil && *t.Output != "" && *t.Output != "." {
			tOut = *t.Output
		}

		msg := styles.RenderSuccessMessage(
			t.Target+" SDK Generated Successfully",
			"✎ Output written to "+tOut,
			fmt.Sprintf("⏲ Generated in %.1f Seconds", endDuration.Seconds()),
		)
		logger.Println(msg)
	}

	return err
}

func Run(ctx context.Context, target, source, genVersion, installationURL, repo, repoSubDir string, debug bool, rootStep *WorkflowStep) error {
	if rootStep == nil {
		rootStep = NewWorkflowStep("ignored", nil)
	}

	wf, projectDir, err := GetWorkflowAndDir()
	if err != nil {
		return err
	}

	if source != "" && target != "" {
		return fmt.Errorf("cannot specify both a target and a source")
	}

	if target == "all" {
		for t := range wf.Targets {
			err := runTarget(ctx, t, wf, projectDir, genVersion, installationURL, repo, repoSubDir, debug, rootStep)
			if err != nil {
				return err
			}
		}
	} else if source == "all" {
		for id, s := range wf.Sources {
			_, err := runSource(ctx, id, &s, rootStep)
			if err != nil {
				return err
			}
		}
	} else if target != "" {
		if _, ok := wf.Targets[target]; !ok {
			return fmt.Errorf("target %s not found", target)
		}

		err := runTarget(ctx, target, wf, projectDir, genVersion, installationURL, repo, repoSubDir, debug, rootStep)
		if err != nil {
			return err
		}
	} else if source != "" {
		s, ok := wf.Sources[source]
		if !ok {
			return fmt.Errorf("source %s not found", source)
		}

		_, err := runSource(ctx, source, &s, rootStep)
		if err != nil {
			return err
		}
	}

	if env.IsGithubAction() {
		githubactions.AddStepSummary(rootStep.ToMermaidDiagram())
	}

	return nil
}

func getTarget(target string) (*workflow.Target, error) {
	wf, _, err := GetWorkflowAndDir()
	if err != nil {
		return nil, err
	}
	t := wf.Targets[target]
	return &t, nil
}

func runTarget(ctx context.Context, target string, wf *workflow.Workflow, projectDir, genVersion, installationURL, repo, repoSubDir string, debug bool, rootStep *WorkflowStep) error {
	rootStep = rootStep.NewSubstep(fmt.Sprintf("Target: %s", target))

	t := wf.Targets[target]

	log.From(ctx).Infof("Running target %s (%s)...\n", target, t.Target)

	source, sourcePath, err := wf.GetTargetSource(target)
	if err != nil {
		return err
	}

	if source != nil {
		sourcePath, err = runSource(ctx, t.Source, source, rootStep)

		if err != nil {
			return err
		}
	} else {
		rootStep.NewSubstep("Validating document")
		if err := validateDocument(ctx, sourcePath); err != nil {
			return err
		}
	}

	var outDir string
	if t.Output != nil {
		outDir = *t.Output
	} else {
		outDir = projectDir
	}

	published := t.Publishing != nil && t.Publishing.IsPublished(target)

	genStep := rootStep.NewSubstep(fmt.Sprintf("Generating %s SDK", utils.CapitalizeFirst(t.Target)))

	logListener := make(chan log.Msg)
	logger := log.From(ctx).WithListener(logListener)
	ctx = log.With(ctx, logger)
	go genStep.ListenForSubsteps(logListener)

	if err := sdkgen.Generate(
		ctx,
		config.GetCustomerID(),
		config.GetWorkspaceID(),
		t.Target,
		sourcePath,
		"",
		"",
		outDir,
		genVersion,
		installationURL,
		debug,
		true,
		published,
		false,
		repo,
		repoSubDir,
		true,
	); err != nil {
		return err
	}

	rootStep.NewSubstep("Cleaning up")

	// Clean up temp files on success
	os.RemoveAll(workflow.GetTempDir())

	rootStep.SucceedWorkflow()

	return nil
}

func runSource(ctx context.Context, id string, source *workflow.Source, rootStep *WorkflowStep) (string, error) {
	rootStep = rootStep.NewSubstep(fmt.Sprintf("Source: %s", id))

	logger := log.From(ctx)
	logger.Infof("Running source %s...", id)

	outputLocation, err := source.GetOutputLocation()
	if err != nil {
		return "", err
	}

	var currentDocument string

	if len(source.Inputs) == 1 {
		if source.Inputs[0].IsRemote() {
			rootStep.NewSubstep("Downloading document")

			downloadLocation := outputLocation
			if len(source.Overlays) > 0 {
				downloadLocation = source.Inputs[0].GetTempDownloadPath(workflow.GetTempDir())
			}

			currentDocument, err = resolveRemoteDocument(ctx, source.Inputs[0], downloadLocation)
			if err != nil {
				return "", err
			}
		} else {
			currentDocument = source.Inputs[0].Location
		}
	} else {
		mergeStep := rootStep.NewSubstep("Merge documents")

		mergeLocation := source.GetTempMergeLocation()
		if len(source.Overlays) == 0 {
			mergeLocation = outputLocation
		}

		logger.Infof("Merging %d schemas into %s...", len(source.Inputs), mergeLocation)

		inSchemas := []string{}
		for _, input := range source.Inputs {
			if input.IsRemote() {
				mergeStep.NewSubstep(fmt.Sprintf("Download document from %s", input.Location))

				downloadedPath, err := resolveRemoteDocument(ctx, input, input.GetTempDownloadPath(workflow.GetTempDir()))
				if err != nil {
					return "", err
				}

				inSchemas = append(inSchemas, downloadedPath)
			} else {
				inSchemas = append(inSchemas, input.Location)
			}
		}

		mergeStep.NewSubstep(fmt.Sprintf("Merge %d documents", len(source.Inputs)))

		if err := mergeDocuments(ctx, inSchemas, mergeLocation); err != nil {
			return "", err
		}

		currentDocument = mergeLocation
	}

	if len(source.Overlays) > 0 {
		overlayStep := rootStep.NewSubstep("Applying overlays")

		overlayLocation := outputLocation

		logger.Infof("Applying %d overlays into %s...", len(source.Overlays), overlayLocation)

		overlaySchemas := []string{}
		for _, overlay := range source.Overlays {
			if overlay.IsRemote() {
				overlayStep.NewSubstep(fmt.Sprintf("Download document from %s", overlay.Location))

				downloadedPath, err := resolveRemoteDocument(ctx, overlay, workflow.GetTempDir())
				if err != nil {
					return "", err
				}

				overlaySchemas = append(overlaySchemas, downloadedPath)
			} else {
				overlaySchemas = append(overlaySchemas, overlay.Location)
			}
		}

		overlayStep.NewSubstep(fmt.Sprintf("Apply %d overlay(s)", len(source.Overlays)))

		if err := overlayDocument(ctx, currentDocument, overlaySchemas, overlayLocation); err != nil {
			return "", err
		}
	}

	rootStep.NewSubstep("Validating document")

	if err := validateDocument(ctx, outputLocation); err != nil {
		return "", err
	}

	rootStep.SucceedWorkflow()

	return outputLocation, nil
}

func resolveRemoteDocument(ctx context.Context, d workflow.Document, outPath string) (string, error) {
	log.From(ctx).Infof("Downloading %s... to %s\n", d.Location, outPath)

	if err := os.MkdirAll(filepath.Dir(outPath), os.ModePerm); err != nil {
		return "", err
	}

	var token, header string
	if d.Auth != nil {
		header = d.Auth.Header
		token = os.Getenv(strings.TrimPrefix(d.Auth.Secret, "$"))
	}

	if err := download.DownloadFile(d.Location, outPath, header, token); err != nil {
		return "", err
	}

	return outPath, nil
}

func mergeDocuments(ctx context.Context, inSchemas []string, outFile string) error {
	if err := os.MkdirAll(filepath.Dir(outFile), os.ModePerm); err != nil {
		return err
	}

	if err := merge.MergeOpenAPIDocuments(ctx, inSchemas, outFile); err != nil {
		return err
	}

	log.From(ctx).Printf("Successfully merged %d schemas into %s", len(inSchemas), outFile)

	return nil
}

func overlayDocument(ctx context.Context, schema string, overlayFiles []string, outFile string) error {
	currentBase := schema

	if err := os.MkdirAll(filepath.Dir(outFile), os.ModePerm); err != nil {
		return err
	}

	f, err := os.Create(outFile)
	if err != nil {
		return err
	}

	for _, overlayFile := range overlayFiles {
		if err := overlay.Apply(currentBase, overlayFile, f); err != nil {
			return err
		}

		currentBase = outFile
	}

	log.From(ctx).Successf("Successfully applied %d overlays into %s", len(overlayFiles), outFile)

	return nil
}

func validateDocument(ctx context.Context, schemaPath string) error {
	limits := &validation.OutputLimits{
		MaxErrors:   1000,
		MaxWarns:    1000,
		OutputHints: false,
	}

	return validation.ValidateOpenAPI(ctx, schemaPath, "", "", limits)
}
