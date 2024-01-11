package cmd

import (
	"fmt"
	"os"

	"github.com/pkg/errors"
	"github.com/speakeasy-api/openapi-generation/v2/pkg/generate"
	config "github.com/speakeasy-api/sdk-gen-config"
	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/speakeasy-api/speakeasy/charm"
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
	rootCmd.AddCommand(quickstartCmd)
}

func quickstartExec(cmd *cobra.Command, args []string) error {
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

	// quickstartObj.WorkflowFile.Sources = make(map[string]workflow.Source)

	// TODO: Replace with workflow.Save once some pending PRs are merged
	yamlData, err := yaml.Marshal(quickstartObj.WorkflowFile)
	if err != nil {
		return err
	}
	if _, err := os.Stat(".speakeasy"); os.IsNotExist(err) {
		err = os.MkdirAll(".speakeasy", 0o755)
		if err != nil {
			return err
		}
	}
	err = os.WriteFile(".speakeasy/workflow.yaml", yamlData, 0o644)
	if err != nil {
		return err
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

	return nil
}
