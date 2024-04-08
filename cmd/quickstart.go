package cmd

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/speakeasy-api/speakeasy/internal/charm/styles"
	"github.com/speakeasy-api/speakeasy/internal/model/flag"

	"github.com/speakeasy-api/huh"
	"github.com/speakeasy-api/speakeasy/internal/model"

	"github.com/iancoleman/strcase"
	"github.com/pkg/errors"
	"github.com/speakeasy-api/openapi-generation/v2/pkg/generate"
	config "github.com/speakeasy-api/sdk-gen-config"
	"github.com/speakeasy-api/sdk-gen-config/workflow"
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

func quickstartExec(ctx context.Context, flags QuickstartFlags) error {
	workingDir, err := os.Getwd()
	if err != nil {
		return err
	}

	if workflowFile, _, _ := workflow.Load(workingDir); workflowFile != nil {
		return fmt.Errorf("You cannot run quickstart when a speakeasy workflow already exists. \n" +
			"To create a brand new SDK directory: `cd ..` and then `speakeasy quickstart`. \n" +
			"To add an additional SDK to this workflow: `speakeasy configure`. \n" +
			"To regenerate the current workflow: `speakeasy run`.")
	}

	if prompts.HasExistingGeneration(workingDir) {
		return fmt.Errorf("You cannot run quickstart when a speakeasy workflow already exists. \n" +
			"To create a brand new SDK directory: cd .. and then `speakeasy quickstart`. \n" +
			"To add an additional SDK to this workflow: `speakeasy configure`. \n" +
			"To regenerate the current workflow: `speakeasy run`.")
	}

	fmt.Println(charm.FormatCommandTitle("Welcome to the Speakeasy!",
		"Speakeasy Quickstart guides you to build a generation workflow for any combination of sources and targets. \n"+
			"After completing these steps you will be ready to start customizing and generating your SDKs.") + "\n\n\n")

	quickstartObj := prompts.Quickstart{
		WorkflowFile: &workflow.Workflow{
			Version: workflow.WorkflowVersion,
			Sources: make(map[string]workflow.Source),
			Targets: make(map[string]workflow.Target),
		},
		LanguageConfigs: make(map[string]*config.Configuration),
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
		state, err := stateFunc(&quickstartObj)
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

	promptedDir := filepath.Join(workingDir, strcase.ToKebab(sdkClassName)+"-"+targetType)
	if outDir != workingDir {
		promptedDir = outDir
	}
	description := "We recommend a git repo per SDK. To use the current directory, leave empty."
	if targetType == "terraform" {
		description = "Terraform providers must be placed in a directory named in the following format terraform-provider-*. according to Hashicorp conventions"
		outDir = "terraform-provider"
	}

	if _, err := charm.NewForm(huh.NewForm(huh.NewGroup(charm.NewInput().
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
		Inline(false).Prompt("").Value(&promptedDir))),
		"Pick an output directory for your newly created files.").
		ExecuteForm(); err != nil {
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

		fmt.Println(
			styles.RenderInfoMessage(
				"The OpenAPI sample document will be used",
				"It can be found here, you can edit it at anytime:",
				absSchemaPath,
			),
		)

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
		if err := config.SaveConfig(outDir, outConfig); err != nil {
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

	workflow, err := run.NewWorkflow("Workflow", initialTarget, "", "", nil, nil, false, !flags.SkipCompile, false)
	if err != nil {
		return err
	}
	workflow.FromQuickstart = true

	if err = workflow.RunWithVisualization(ctx); err != nil {
		return errors.Wrapf(err, "failed to run generation workflow")
	}

	return nil
}
