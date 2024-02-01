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
	"github.com/speakeasy-api/speakeasy/internal/utils"
	"github.com/speakeasy-api/speakeasy/prompts"
	"gopkg.in/yaml.v3"
)

type QuickstartFlags struct {
	ShouldCompile bool   `json:"compile"`
	Schema        string `json:"schema"`
	OutDir        string `json:"out-dir"`
	TargetType    string `json:"target"`
}

var quickstartCmd = &model.ExecutableCommand[QuickstartFlags]{
	Usage:        "quickstart",
	Short:        "Guided setup to help you create a new SDK in minutes.",
	Long:         `Guided setup to help you create a new SDK in minutes.`,
	Run:          quickstartExec,
	RequiresAuth: true,
	Flags: []model.Flag{
		model.BooleanFlag{
			Name:         "compile",
			Shorthand:    "c",
			Description:  "run SDK validation and generation after quickstart",
			DefaultValue: true,
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

	var targetType string
	chosenDir := flags.OutDir
	for _, target := range quickstartObj.WorkflowFile.Targets {
		if chosenDir == "" {
			chosenDir = defaultOutDir(target.Target)
			targetType = target.Target
			break
		}
	}

	if _, err := tea.NewProgram(charm.NewForm(huh.NewForm(huh.NewGroup(charm.NewInput().
		Title("What directory should quickstart files be written too?").
		Description("A sensible default has been provided for you. An empty entry will map to the current root directory.\n").
		Validate(func(s string) error {
			if targetType == "terraform" {
				if !strings.HasPrefix(s, "terraform-provider") && !strings.HasPrefix(filepath.Base(workingDir), "terraform-provider") {
					return errors.New("a terraform provider directory must start with 'terraform-provider'")
				}
			}
			return nil
		}).
		Inline(false).Prompt("").Value(&chosenDir))),
		"Let's pick an output directory for your newly created files.")).
		Run(); err != nil {
		return err
	}

	outDir := workingDir
	if chosenDir != "" {
		outDir = filepath.Join(workingDir, chosenDir)
	}

	var resolvedSchema string
	for _, source := range quickstartObj.WorkflowFile.Sources {
		resolvedSchema = source.Inputs[0].Location
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

	// If we are referencing a local schema, copy it to the output directory
	if _, err := os.Stat(resolvedSchema); err == nil {
		if err := utils.CopyFile(resolvedSchema, outDir+"/"+resolvedSchema); err != nil {
			return errors.Wrapf(err, "failed to copy schema file")
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

	if flags.ShouldCompile {
		// Change working directory to our output directory
		if err := os.Chdir(outDir); err != nil {
			return errors.Wrapf(err, "failed to run speakeasy generate")
		}

		if err = run.RunWithVisualization(ctx, initialTarget, "", genVersion, "", "", "", false); err != nil {
			return errors.Wrapf(err, "failed to run speakeasy generate")
		}
	}

	return nil
}

func defaultOutDir(target string) string {
	switch target {
	case "terraform":
		return "terraform-provider-speakeasy"
	default:
	}

	return target
}
