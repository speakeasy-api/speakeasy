package cmd

import (
	"context"
	_ "embed"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/browser"
	"github.com/samber/lo"
	"github.com/speakeasy-api/speakeasy/internal/config"
	"github.com/speakeasy-api/speakeasy/internal/env"
	"github.com/speakeasy-api/speakeasy/internal/interactivity"
	"github.com/speakeasy-api/speakeasy/internal/studio"
	"github.com/speakeasy-api/speakeasy/internal/utils"
	"golang.org/x/exp/maps"

	"github.com/speakeasy-api/speakeasy/internal/charm/styles"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/model/flag"

	"github.com/speakeasy-api/huh"
	"github.com/speakeasy-api/speakeasy/internal/model"

	"github.com/iancoleman/strcase"
	"github.com/pkg/errors"
	"github.com/speakeasy-api/openapi-generation/v2/pkg/generate"
	sdkGenConfig "github.com/speakeasy-api/sdk-gen-config"
	"github.com/speakeasy-api/sdk-gen-config/workflow"
	speakeasyErrors "github.com/speakeasy-api/speakeasy-core/errors"
	"github.com/speakeasy-api/speakeasy-core/suggestions"
	"github.com/speakeasy-api/speakeasy/internal/charm"
	"github.com/speakeasy-api/speakeasy/internal/run"
	"github.com/speakeasy-api/speakeasy/prompts"
)

type QuickstartFlags struct {
	SkipCompile bool   `json:"skip-compile"`
	Schema      string `json:"schema"`
	OutDir      string `json:"out-dir"`
	TargetType  string `json:"target"`
}

//go:embed sample_openapi.yaml
var sampleSpec string

var quickstartCmd = &model.ExecutableCommand[QuickstartFlags]{
	Usage:        "quickstart",
	Short:        "Guided setup to help you create a new SDK in minutes.",
	Long:         `Guided setup to help you create a new SDK in minutes.`,
	Run:          quickstartExec,
	RequiresAuth: true,
	Flags: []flag.Flag{
		flag.BooleanFlag{
			Name:        "skip-compile",
			Description: "skip compilation during generation after setup",
		},
		flag.StringFlag{
			Name:                       "schema",
			Shorthand:                  "s",
			Description:                "local filepath or URL for the OpenAPI schema",
			AutocompleteFileExtensions: charm.OpenAPIFileExtensions,
		},
		flag.StringFlag{
			Name:        "out-dir",
			Shorthand:   "o",
			Description: "output directory for the quickstart command",
		},
		flag.StringFlag{
			Name:        "target",
			Shorthand:   "t",
			Description: fmt.Sprintf("language to generate sdk for (available options: [%s])", strings.Join(prompts.GetSupportedTargets(), ", ")),
		},
	},
}

const ErrWorkflowExists = speakeasyErrors.Error("You cannot run quickstart when a speakeasy workflow already exists. \n" +
	"To create a brand _new_ SDK directory: `cd ..` and then `speakeasy quickstart`. \n" +
	"To add an additional SDK to this workflow: `speakeasy configure`. \n" +
	"To regenerate the current workflow: `speakeasy run`.")

func quickstartExec(ctx context.Context, flags QuickstartFlags) error {
	workingDir, err := os.Getwd()
	if err != nil {
		return err
	}

	if workflowFile, _, _ := workflow.Load(workingDir); workflowFile != nil {
		return ErrWorkflowExists
	}

	if prompts.HasExistingGeneration(workingDir) {
		return ErrWorkflowExists
	}

	log.From(ctx).PrintfStyled(styles.DimmedItalic, "\nYour first SDK is a few short questions away...\n")

	quickstartObj := prompts.Quickstart{
		WorkflowFile: &workflow.Workflow{
			Version: workflow.WorkflowVersion,
			Sources: make(map[string]workflow.Source),
			Targets: make(map[string]workflow.Target),
		},
		LanguageConfigs: make(map[string]*sdkGenConfig.Configuration),
	}

	if flags.Schema != "" {
		quickstartObj.Defaults.SchemaPath = &flags.Schema
	}

	if flags.TargetType != "" {
		quickstartObj.Defaults.TargetType = &flags.TargetType
	}

	nextState := prompts.SourceBase
	for nextState != prompts.Complete {
		stateFunc := prompts.StateMapping[nextState]
		state, err := stateFunc(ctx, &quickstartObj)
		if err != nil {
			return err
		}
		nextState = *state
	}

	if err := quickstartObj.WorkflowFile.Validate(generate.GetSupportedLanguages()); err != nil {
		return errors.Wrapf(err, "failed to validate workflow file")
	}

	outDir := workingDir
	if flags.OutDir != "" {
		outDir = flags.OutDir
	}

	// Pull the target type and sdk class name from the first target
	// Assume just one target possible during quickstart
	var targetType string
	var sdkClassName string
	for _, target := range quickstartObj.WorkflowFile.Targets {
		targetType = target.Target
		break
	}
	for _, config := range quickstartObj.LanguageConfigs {
		sdkClassName = config.Generation.SDKClassName
		break
	}

	// use the sdkClassName as an improved workflow target name
	if sdkClassName != "" {
		targetName := strcase.ToKebab(sdkClassName)
		quickstartObj.WorkflowFile.Targets[targetName] = quickstartObj.WorkflowFile.Targets[prompts.TargetNameDefault]
		delete(quickstartObj.WorkflowFile.Targets, prompts.TargetNameDefault)
	}

	promptedDir := setDefaultOutDir(workingDir, sdkClassName, targetType)
	if outDir != workingDir {
		promptedDir = outDir
	}
	description := "We recommend a git repo per SDK. To use the current directory, leave empty."
	if targetType == "terraform" {
		description = "Terraform providers must be placed in a directory named in the following format terraform-provider-*. according to Hashicorp conventions"
		outDir = "terraform-provider"
	}

	if !currentDirectoryEmpty() {
		_, err = charm.NewForm(huh.NewForm(huh.NewGroup(charm.NewInput().
			Title("What directory should the "+targetType+" files be written to?").
			Description(description+"\n").
			Suggestions(charm.DirsInCurrentDir(promptedDir)).
			SetSuggestionCallback(charm.SuggestionCallback(charm.SuggestionCallbackConfig{IsDirectories: true})).
			Validate(func(s string) error {
				if targetType == "terraform" {
					if !strings.HasPrefix(s, "terraform-provider") && !strings.HasPrefix(filepath.Base(filepath.Join(workingDir, s)), "terraform-provider") {
						return errors.New("a terraform provider directory must start with 'terraform-provider'")
					}
				}
				return nil
			}).
			Value(&promptedDir))),
			charm.WithTitle("Pick an output directory for your newly created files.")).
			ExecuteForm()
	} else {
		promptedDir = "."
	}

	if err != nil {
		return err
	}
	if !filepath.IsAbs(promptedDir) {
		promptedDir = filepath.Join(workingDir, promptedDir)
	}

	outDir, err = filepath.Abs(promptedDir)
	if err != nil {
		return err
	}

	speakeasyFolderPath := filepath.Join(outDir, ".speakeasy")
	if _, err := os.Stat(speakeasyFolderPath); os.IsNotExist(err) {
		err = os.MkdirAll(speakeasyFolderPath, 0o755)
		if err != nil {
			return err
		}
	}

	var resolvedSchema string
	var sourceName string
	for name, source := range quickstartObj.WorkflowFile.Sources {
		sourceName = name
		resolvedSchema = source.Inputs[0].Location
	}

	// If we are referencing a local schema, set a relative path for the new out directory
	if _, err := os.Stat(resolvedSchema); err == nil && outDir != workingDir {
		absSchemaPath, err := filepath.Abs(resolvedSchema)
		if err != nil {
			return err
		}

		referencePath, err := filepath.Rel(outDir, absSchemaPath)
		if err != nil {
			return err
		}
		quickstartObj.WorkflowFile.Sources[sourceName].Inputs[0].Location = referencePath
	}

	if quickstartObj.IsUsingSampleOpenAPISpec {
		absSchemaPath := filepath.Join(outDir, "openapi.yaml")
		if err := os.WriteFile(absSchemaPath, []byte(sampleSpec), 0o644); err != nil {
			return errors.Wrapf(err, "failed to write sample OpenAPI spec")
		}

		printSampleSpecMessage(absSchemaPath)

		referencePath, err := filepath.Rel(outDir, absSchemaPath)
		if err != nil {
			return err
		}
		quickstartObj.WorkflowFile.Sources[sourceName].Inputs[0].Location = referencePath
	}

	if err := workflow.Save(outDir, quickstartObj.WorkflowFile); err != nil {
		return errors.Wrapf(err, "failed to save workflow file")
	}

	for key, outConfig := range quickstartObj.LanguageConfigs {
		if err := sdkGenConfig.SaveConfig(outDir, outConfig); err != nil {
			return errors.Wrapf(err, "failed to save config file for target %s", key)
		}
	}

	var initialTarget string
	for key := range quickstartObj.WorkflowFile.Targets {
		initialTarget = key
		break
	}

	// Change working directory to our output directory
	if err := os.Chdir(outDir); err != nil {
		return errors.Wrapf(err, "failed to run speakeasy generate")
	}

	wf, err := run.NewWorkflow(
		ctx,
		run.WithTarget(initialTarget),
		run.WithShouldCompile(!flags.SkipCompile),
		run.WithSkipCleanup(), // The studio won't work if we clean up before it launches
	)

	defer func() {
		wf.Cleanup()
	}()

	if err != nil {
		return err
	}
	wf.FromQuickstart = true

	logger := log.From(ctx)
	var changeDirMsg string
	relPath, _ := filepath.Rel(workingDir, outDir)
	if workingDir != outDir && relPath != "" {
		changeDirMsg = fmt.Sprintf("`cd %s` before moving forward with your SDK", relPath)
	}

	if err = wf.RunWithVisualization(ctx); err != nil {
		if strings.Contains(err.Error(), "document invalid") {
			if retry, newErr := retryWithSampleSpec(ctx, quickstartObj.WorkflowFile, initialTarget, outDir, flags.SkipCompile); newErr != nil {
				return errors.Wrapf(err, "failed to run generation workflow")
			} else if retry {
				if changeDirMsg != "" {
					logger.Println(styles.RenderWarningMessage("! ATTENTION DO THIS !", changeDirMsg))
				}

				return nil
			}
		}

		return errors.Wrapf(err, "failed to run generation workflow")
	}

	if len(wf.OperationsRemoved) > 0 {
		// If we have modified the workflow with a minimum viable spec we should make sure that is saved
		if err := workflow.Save(outDir, wf.GetWorkflowFile()); err != nil {
			return errors.Wrapf(err, "failed to save workflow file")
		}
	}

	if changeDirMsg != "" {
		logger.Println(styles.RenderWarningMessage("! ATTENTION DO THIS !", changeDirMsg))
	}

	if shouldLaunchStudio(ctx, wf, true) {
		err = studio.LaunchStudio(ctx, wf)
	} else if len(wf.SDKOverviewURLs) == 1 { // There should only be one target after quickstart
		overviewURL := wf.SDKOverviewURLs[initialTarget]
		browser.OpenURL(overviewURL)
	}

	return nil
}

func retryWithSampleSpec(ctx context.Context, workflowFile *workflow.Workflow, initialTarget, outDir string, skipCompile bool) (bool, error) {
	retrySampleSpec := true
	_, err := charm.NewForm(huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[bool]().
				Title("\n\nYour OpenAPI spec seems to have some validation issues.\nWould you like to retry with a sample spec?\n").
				Options(
					huh.NewOption("Yes", true),
					huh.NewOption("No", false),
				).
				Value(&retrySampleSpec),
		),
	)).ExecuteForm()
	if err != nil {
		return false, err
	}

	if !retrySampleSpec {
		return false, nil
	}

	absSchemaPath := filepath.Join(outDir, "openapi.yaml")
	if err := os.WriteFile(absSchemaPath, []byte(sampleSpec), 0o644); err != nil {
		return true, errors.Wrapf(err, "failed to write sample OpenAPI spec")
	}

	workflowFile.Sources[workflowFile.Targets[initialTarget].Source].Inputs[0].Location = "openapi.yaml"
	if err := workflow.Save(outDir, workflowFile); err != nil {
		return true, errors.Wrapf(err, "failed to save workflow file")
	}

	printSampleSpecMessage(absSchemaPath)

	wf, err := run.NewWorkflow(
		ctx,
		run.WithTarget(initialTarget),
		run.WithShouldCompile(!skipCompile),
	)

	err = wf.RunWithVisualization(ctx)

	return true, err
}

func shouldLaunchStudio(ctx context.Context, wf *run.Workflow, fromQuickstart bool, commandLineFlagToLaunchStudio *bool) bool {
	// TODO: Remove this when ready to launch for everyone
	if !config.IsAdminUnsafe() {
		return false
	}

	if len(wf.SourceResults) != 1 {
		// Only one source at a time is supported in the studio at the moment
		return false
	}
	sourceResult := maps.Values(wf.SourceResults)[0]

	if commandLineFlagToLaunchStudio != nil {
		// User explicitly chose whether to launch the studio or not
		return *commandLineFlagToLaunchStudio
	}

	if !utils.IsInteractive() || env.IsGithubAction() {
		return false
	}

	if sourceResult.LintResult == nil {
		// No lint result indicates the spec wasn't even loaded successfully, the studio can't help with that
		return false
	}

	// TODO: include anyDiagnostics into here if we want to launch the studio after run when we detect issues
	numDiagnostics := lo.SumBy(lo.Values(sourceResult.Diagnosis), func(x []suggestions.Diagnostic) int {
		return len(x)
	})

	if numDiagnostics == 0 {
		// No interesting diagnostics to show in the studio
		return false
	}

	offerDeclineOption := !fromQuickstart && config.SeenStudio()

	message := fmt.Sprintf("We've detected %d potential improvements for your SDK. Would you like to launch the studio?", numDiagnostics)

	if offerDeclineOption {
		return interactivity.SimpleConfirm(message)
	}

	interactivity.SimpleConfirmWithOnlyAccept(message)
	return true

}

func printSampleSpecMessage(absSchemaPath string) {
	fmt.Println(
		styles.RenderInfoMessage(
			"A sample OpenAPI document will be used",
			"You can edit it anytime here:",
			absSchemaPath,
		) + "\n",
	)
}

func setDefaultOutDir(workingDir string, sdkClassName string, targetType string) string {
	subDirectory := strcase.ToKebab(sdkClassName) + "-" + targetType
	if targetType == "terraform" {
		if strings.HasPrefix(filepath.Base(workingDir), "terraform-provider") {
			return "."
		}

		subDirectory = fmt.Sprintf("terraform-provider-%s", subDirectory)
	}

	return filepath.Join(workingDir, subDirectory)
}

func currentDirectoryEmpty() bool {
	dir, err := os.Open(".")
	if err != nil {
		fmt.Println("Error opening directory:", err)
		return false
	}
	defer dir.Close()

	_, err = dir.Readdirnames(1)
	return err == io.EOF
}
