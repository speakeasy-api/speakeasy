package cmd

import (
	"bytes"
	"fmt"
	"os"

	"github.com/pkg/errors"
	"github.com/speakeasy-api/openapi-generation/v2/pkg/generate"
	config "github.com/speakeasy-api/sdk-gen-config"
	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/speakeasy-api/speakeasy/charm"
	"github.com/speakeasy-api/speakeasy/internal/auth"
	"github.com/speakeasy-api/speakeasy/internal/run"
	"github.com/speakeasy-api/speakeasy/quickstart"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var quickstartCmd = &cobra.Command{
	Use:   "quickstart",
	Short: "Guided setup to help you create a new SDK in minutes.",
	Long:  `Guided setup to help you create a new SDK in minutes.`,
	RunE:  quickstartExec,
}

func quickstartInit() {
	quickstartCmd.Flags().BoolP("compile", "c", true, "run SDK validation and generation after quickstart")
	rootCmd.AddCommand(quickstartCmd)
}

func quickstartExec(cmd *cobra.Command, args []string) error {
	if err := auth.Authenticate(cmd.Context(), false); err != nil {
		return err
	}

	shouldCompile, err := cmd.Flags().GetBool("compile")
	if err != nil {
		return err
	}

	workingDir, err := os.Getwd()
	if err != nil {
		return err
	}

	if workflowFile, _, _ := workflow.Load(workingDir); workflowFile != nil {
		return fmt.Errorf("cannot run quickstart when a speakeasy workflow already exists")
	}

	fmt.Println(charm.FormatCommandTitle("Welcome to the Speakeasy!",
		"Speakeasy Quickstart guides you to build a generation workflow for any combination of sources and targets. \n"+
			"After completing these steps you will be ready to start customizing and generating your SDKs.") + "\n\n\n")

	quickstartObj := quickstart.Quickstart{
		WorkflowFile: &workflow.Workflow{
			Version: workflow.WorkflowVersion,
			Sources: make(map[string]workflow.Source),
			Targets: make(map[string]workflow.Target),
		},
		LanguageConfigs: make(map[string]*config.Configuration),
	}

	nextState := quickstart.SourceBase
	for nextState != quickstart.Complete {
		stateFunc := quickstart.StateMapping[nextState]
		state, err := stateFunc(&quickstartObj)
		if err != nil {
			return err
		}
		nextState = *state
	}

	if err := quickstartObj.WorkflowFile.Validate(generate.GetSupportedLanguages()); err != nil {
		return errors.Wrapf(err, "failed to validate workflow file")
	}

	if _, err := os.Stat(".speakeasy"); os.IsNotExist(err) {
		err = os.MkdirAll(".speakeasy", 0o755)
		if err != nil {
			return err
		}
	}

	if err := workflow.Save(workingDir, quickstartObj.WorkflowFile); err != nil {
		return errors.Wrapf(err, "failed to save workflow file")
	}

	for key, outConfig := range quickstartObj.LanguageConfigs {
		outDir := workingDir
		if quickstartObj.WorkflowFile.Targets[key].Output != nil {
			outDir = *quickstartObj.WorkflowFile.Targets[key].Output
		}

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

	if _, err := os.Stat(".github/workflows"); os.IsNotExist(err) {
		err = os.MkdirAll(".github/workflows", 0o755)
		if err != nil {
			return err
		}
	}

	if err = os.WriteFile(".github/workflows/speakeasy_sdk_generation.yaml", genWorkflowBuf.Bytes(), 0o644); err != nil {
		return errors.Wrapf(err, "failed to write github workflow file")
	}

	var initialTarget string
	for key := range quickstartObj.WorkflowFile.Targets {
		initialTarget = key
		break
	}

	if shouldCompile {
		if err = run.RunWithVisualization(cmd.Context(), initialTarget, "", genVersion, "", "", "", false); err != nil {
			return errors.Wrapf(err, "failed to run speakeasy generate")
		}
	}

	return nil
}
