package cmd

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/speakeasy-api/speakeasy/internal/model"

	"github.com/pkg/errors"
	"github.com/speakeasy-api/openapi-generation/v2/pkg/generate"
	config "github.com/speakeasy-api/sdk-gen-config"
	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/speakeasy-api/speakeasy/internal/charm"
	"github.com/speakeasy-api/speakeasy/internal/run"
	"github.com/speakeasy-api/speakeasy/prompts"
	"gopkg.in/yaml.v3"
)

type QuickstartFlags struct {
	SkipCompile bool   `json:"skip-compile"`
	Schema      string `json:"schema"`
	OutDir      string `json:"out-dir"`
	TargetType  string `json:"target"`
}

var quickstartCmd = &model.ExecutableCommand[QuickstartFlags]{
	Usage:        "quickstart",
	Short:        "Guided setup to help you create a new SDK in minutes.",
	Long:         `Guided setup to help you create a new SDK in minutes.`,
	Run:          quickstartExec,
	RequiresAuth: true,
	Flags: []model.Flag{
		model.BooleanFlag{
			Name:        "skip-compile",
			Description: "skip compilation during generation after setup",
		},
		model.StringFlag{
			Name:        "schema",
			Shorthand:   "s",
			Description: "local filepath or URL for the OpenAPI schema",
		},
		model.StringFlag{
			Name:        "out-dir",
			Shorthand:   "o",
			Description: "output directory for the quickstart command",
		},
		model.StringFlag{
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
		return fmt.Errorf("you cannot run quickstart when a speakeasy workflow already exists, try speakeasy configure instead")
	}

	if prompts.HasExistingGeneration(workingDir) {
		return fmt.Errorf("you cannot run quickstart when an existing sdk already exists, try speakeasy configure instead")
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
	var targetType string
	for _, target := range quickstartObj.WorkflowFile.Targets {
		targetType = target.Target
		break
	}

	var isUncleanDir bool
	if entry, err := os.ReadDir(workingDir); err == nil {
		for _, e := range entry {
			if !strings.HasPrefix(e.Name(), ".") && !strings.HasSuffix(e.Name(), ".yaml") && !strings.HasSuffix(e.Name(), ".yml") && !strings.HasSuffix(e.Name(), ".json") {
				isUncleanDir = true
				break
			}
		}
	}

	if (isUncleanDir && outDir == workingDir) || (targetType == "terraform" && !strings.HasPrefix(filepath.Base(outDir), "terraform-provider")) {
		promptedDir := "."
		if outDir != workingDir {
			promptedDir = outDir
		}
		description := "The default option we have provided maps to the current root directory."
		if targetType == "terraform" {
			description = "Terraform providers must be placed in a directory structured in the following format terraform-provider-*."
		}

		if _, err := tea.NewProgram(charm.NewForm(huh.NewForm(huh.NewGroup(charm.NewInput().
			Title("What directory should quickstart files be written too?").
			Description(description+"\n").
			Validate(func(s string) error {
				if targetType == "terraform" {
					if !strings.HasPrefix(s, "terraform-provider") && !strings.HasPrefix(filepath.Base(filepath.Join(workingDir, s)), "terraform-provider") {
						return errors.New("a terraform provider directory must start with 'terraform-provider'")
					}
				}
				return nil
			}).
			Inline(false).Prompt("").Value(&promptedDir))),
			"Let's pick an output directory for your newly created files.")).
			Run(); err != nil {
			return err
		}
		outDir = filepath.Join(workingDir, promptedDir)
	}

	var resolvedSchema string
	var sourceName string
	for name, source := range quickstartObj.WorkflowFile.Sources {
		sourceName = name
		resolvedSchema = source.Inputs[0].Location
	}

	absoluteOurDir, err := filepath.Abs(outDir)
	if err != nil {
		return err
	}

	// If we are referencing a local schema, set a relative path for the new out directory
	if _, err := os.Stat(resolvedSchema); err == nil && absoluteOurDir != workingDir {
		absPath, err := filepath.Abs(resolvedSchema)
		if err != nil {
			return err
		}

		referencePath, err := filepath.Rel(outDir, absPath)
		if err != nil {
			return err
		}
		quickstartObj.WorkflowFile.Sources[sourceName].Inputs[0].Location = referencePath
	}

	speakeasyFolderPath := outDir + "/" + ".speakeasy"
	if _, err := os.Stat(speakeasyFolderPath); os.IsNotExist(err) {
		err = os.MkdirAll(speakeasyFolderPath, 0o755)
		if err != nil {
			return err
		}
	}

	if err := workflow.Save(outDir, quickstartObj.WorkflowFile); err != nil {
		return errors.Wrapf(err, "failed to save workflow file")
	}

	for key, outConfig := range quickstartObj.LanguageConfigs {
		if err := config.SaveConfig(outDir, outConfig); err != nil {
			return errors.Wrapf(err, "failed to save config file for target %s", key)
		}
	}

	// Write a github workflow file.
	var genWorkflowBuf bytes.Buffer
	yamlEncoder := yaml.NewEncoder(&genWorkflowBuf)
	yamlEncoder.SetIndent(2)
	if err := yamlEncoder.Encode(quickstartObj.GithubWorkflow); err != nil {
		return errors.Wrapf(err, "failed to encode workflow file")
	}

	if _, err := os.Stat(outDir + "/" + ".github/workflows"); os.IsNotExist(err) {
		err = os.MkdirAll(outDir+"/"+".github/workflows", 0o755)
		if err != nil {
			return err
		}
	}

	if err = os.WriteFile(outDir+"/"+".github/workflows/speakeasy_sdk_generation.yaml", genWorkflowBuf.Bytes(), 0o644); err != nil {
		return errors.Wrapf(err, "failed to write github workflow file")
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

	workflow, err := run.NewWorkflow("Workflow", initialTarget, "", genVersion, "", nil, nil, false, !flags.SkipCompile)
	if err != nil {
		return err
	}

	if err = workflow.RunWithVisualization(ctx); err != nil {
		return errors.Wrapf(err, "failed to run generation workflow")
	}

	return nil
}
