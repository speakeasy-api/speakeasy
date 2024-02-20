package run

import (
	"bytes"
	"context"
	"fmt"
	"math"
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

type Workflow struct {
	Target           string
	Source           string
	GenVersion       string
	Repo             string
	RepoSubDirs      map[string]string
	InstallationURLs map[string]string
	Debug            bool
	ShouldCompile    bool

	RootStep            *WorkflowStep
	workflow            *workflow.Workflow
	projectDir          string
	validatedDocuments  []string
	hasGenerationAccess bool
}

func NewWorkflow(name, target, source, genVersion, repo string, repoSubDirs, installationURLs map[string]string, debug, shouldCompile bool) (*Workflow, error) {
	wf, projectDir, err := GetWorkflowAndDir()
	if err != nil {
		return nil, err
	}

	rootStep := NewWorkflowStep(name, nil)

	return &Workflow{
		Target:           target,
		Source:           source,
		GenVersion:       genVersion,
		Repo:             repo,
		RepoSubDirs:      repoSubDirs,
		InstallationURLs: installationURLs,
		Debug:            debug,
		ShouldCompile:    shouldCompile,
		workflow:         wf,
		projectDir:       projectDir,
		RootStep:         rootStep,
	}, nil
}

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

func (w *Workflow) RunWithVisualization(ctx context.Context) error {
	updatesChannel := make(chan UpdateMsg)
	w.RootStep = NewWorkflowStep("Workflow", updatesChannel)

	var logs bytes.Buffer
	var err, runErr error
	logger := log.From(ctx)

	runFnCli := func() error {
		l := logger.WithWriter(&logs) // Swallow logs other than the workflow display
		runCtx := log.With(ctx, l)
		err = w.Run(runCtx)

		w.RootStep.Finalize(err == nil)

		if err != nil {
			runErr = err
			return err
		}

		return nil
	}

	startTime := time.Now()
	err = w.RootStep.RunWithVisualization(runFnCli, updatesChannel)
	endDuration := time.Since(startTime)

	// Display error logs if the workflow failed
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

	// Display success message if the workflow succeeded
	if err == nil && runErr == nil {
		t, err := getTarget(w.Target)
		if err != nil {
			return err
		}
		tOut := "the current directory"
		if t.Output != nil && *t.Output != "" && *t.Output != "." {
			tOut = *t.Output
		}
		if w.Target == "all" {
			tOut = "the paths specified in workflow.yaml"
		}

		msg := styles.RenderSuccessMessage(
			t.Target+" SDK Generated Successfully",
			"✎ Output written to "+tOut,
			fmt.Sprintf("⏲ Generated in %.1f Seconds", endDuration.Seconds()),
		)
		logger.Println(msg)

		if !w.hasGenerationAccess {
			warningDate := time.Date(2024, time.March, 22, 0, 0, 0, 0, time.UTC)
			daysToLimit := int(math.Round(warningDate.Sub(time.Now().Truncate(24*time.Hour)).Hours() / 24))
			msg := styles.RenderInfoMessage(
				"🚀 Time to Upgrade 🚀",
				"\nYou have exceeded the limit of one free generated SDK.",
				"Upgrade your account if you intend to generate multiple SDKs!",
				fmt.Sprintf("Please reach out to the Speakeasy team in the next %d days to ensure continued access.", daysToLimit),
				"\nhttps://calendly.com/d/5dm-wvm-2mx/chat-with-speakeasy-team",
			)
			logger.Println("\n\n" + msg)
		}
	}

	return err
}

func (w *Workflow) Run(ctx context.Context) error {
	if w.Source != "" && w.Target != "" {
		return fmt.Errorf("cannot specify both a target and a source")
	}

	if w.Target == "all" {
		for t := range w.workflow.Targets {
			err := w.runTarget(ctx, t)
			if err != nil {
				return err
			}
		}
	} else if w.Source == "all" {
		for id := range w.workflow.Sources {
			_, err := w.runSource(ctx, w.RootStep, id)
			if err != nil {
				return err
			}
		}
	} else if w.Target != "" {
		if _, ok := w.workflow.Targets[w.Target]; !ok {
			return fmt.Errorf("target %s not found", w.Target)
		}

		err := w.runTarget(ctx, w.Target)
		if err != nil {
			return err
		}
	} else if w.Source != "" {
		if _, ok := w.workflow.Sources[w.Source]; !ok {
			return fmt.Errorf("source %s not found", w.Source)
		}

		_, err := w.runSource(ctx, w.RootStep, w.Source)
		if err != nil {
			return err
		}
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

func (w *Workflow) runTarget(ctx context.Context, target string) error {
	rootStep := w.RootStep.NewSubstep(fmt.Sprintf("Target: %s", target))

	t := w.workflow.Targets[target]

	log.From(ctx).Infof("Running target %s (%s)...\n", target, t.Target)

	source, sourcePath, err := w.workflow.GetTargetSource(target)
	if err != nil {
		return err
	}

	if source != nil {
		sourcePath, err = w.runSource(ctx, rootStep, t.Source)

		if err != nil {
			return err
		}
	} else {
		if err := w.validateDocument(ctx, rootStep, sourcePath); err != nil {
			return err
		}
	}

	var outDir string
	if t.Output != nil {
		outDir = *t.Output
	} else {
		outDir = w.projectDir
	}

	published := t.Publishing != nil && t.Publishing.IsPublished(target)

	genStep := rootStep.NewSubstep(fmt.Sprintf("Generating %s SDK", utils.CapitalizeFirst(t.Target)))

	logListener := make(chan log.Msg)
	logger := log.From(ctx).WithListener(logListener)
	ctx = log.With(ctx, logger)
	go genStep.ListenForSubsteps(logListener)

	hasGenerationAccess, err := sdkgen.Generate(
		ctx,
		config.GetCustomerID(),
		config.GetWorkspaceID(),
		t.Target,
		sourcePath,
		"",
		"",
		outDir,
		w.GenVersion,
		w.InstallationURLs[target],
		w.Debug,
		true,
		published,
		false,
		w.Repo,
		w.RepoSubDirs[target],
		w.ShouldCompile,
	)
	if err != nil {
		return err
	}
	w.hasGenerationAccess = hasGenerationAccess

	rootStep.NewSubstep("Cleaning up")

	// Clean up temp files on success
	os.RemoveAll(workflow.GetTempDir())

	rootStep.SucceedWorkflow()

	return nil
}

func (w *Workflow) runSource(ctx context.Context, parentStep *WorkflowStep, id string) (string, error) {
	rootStep := parentStep.NewSubstep(fmt.Sprintf("Source: %s", id))
	source := w.workflow.Sources[id]

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

	if err := w.validateDocument(ctx, rootStep, outputLocation); err != nil {
		return "", err
	}

	rootStep.SucceedWorkflow()

	return outputLocation, nil
}

func (w *Workflow) validateDocument(ctx context.Context, parentStep *WorkflowStep, schemaPath string) error {
	step := parentStep.NewSubstep("Validating document")

	if slices.Contains(w.validatedDocuments, schemaPath) {
		step.Skip("already validated")
		return nil
	}

	limits := &validation.OutputLimits{
		MaxErrors:   1000,
		MaxWarns:    1000,
		OutputHints: false,
	}

	res := validation.ValidateOpenAPI(ctx, schemaPath, "", "", limits)

	w.validatedDocuments = append(w.validatedDocuments, schemaPath)

	return res
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
