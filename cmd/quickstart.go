package cmd

import (
	"fmt"
	"os"

	"github.com/pkg/errors"
	"github.com/speakeasy-api/openapi-generation/v2/pkg/generate"
	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/speakeasy-api/speakeasy/quickstart"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var quickstartCmd = &cobra.Command{
	Use:   "quickstart",
	Short: "Guided setup for speakeasy workflow files to start generating SDK targets on day 1.",
	Long:  `Guided setup for speakeasy workflow files to start generating SDK targets on day 1.`,
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
	workflowFile := workflow.Workflow{
		Version: workflow.WorkflowVersion,
		Sources: make(map[string]workflow.Source),
		Targets: make(map[string]workflow.Target),
	}

	nextState := quickstart.SourceBase
	for nextState != quickstart.Complete {
		stateFunc := quickstart.StateMapping[nextState]
		state, err := stateFunc(&workflowFile)
		if err != nil {
			return err
		}
		nextState = *state
	}

	if err := workflowFile.Validate(generate.GetSupportedLanguages()); err != nil {
		return errors.Wrapf(err, "failed to validate workflow file")
	}

	// TODO: Replace this with write file from sdk-gen-config
	yamlData, err := yaml.Marshal(&workflowFile)
	if err != nil {
		return err
	}
	// Create the directory if it doesn't exist
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

	//if _, err := os.Stat(".git"); err == nil {
	//	githubAddOn := false
	//	huh.NewConfirm().
	//		Title("Looks like you are in a github repo. Would you like to continue with setting up target generation in Github?").
	//		Affirmative("Yes.").
	//		Negative("No.").
	//		Value(&githubAddOn).Run()
	//	if githubAddOn {
	//		// TODO: Execute the speakeasy configure github logic.
	//	}
	//}

	return nil
}
