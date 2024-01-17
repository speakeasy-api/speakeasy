package cmd

import (
	"os"

	"github.com/pkg/errors"
	"github.com/speakeasy-api/openapi-generation/v2/pkg/generate"
	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/speakeasy-api/speakeasy/internal/auth"
	"github.com/speakeasy-api/speakeasy/internal/interactivity"
	"github.com/speakeasy-api/speakeasy/prompts"
	"github.com/spf13/cobra"
)

var configureCmd = &cobra.Command{
	Use:   "configure",
	Short: "Configure your Speakeasy SDK Setup.",
	Long:  `Configure your Speakeasy SDK Setup.`,
	RunE:  interactivity.InteractiveRunFn("What do you want to configure?"),
}

var configureSourcesCmd = &cobra.Command{
	Use:     "sources",
	Short:   "Guided prompts to configure a new or existing source in your speakeasy workflow.",
	Long:    "Guided prompts to configure a new or existing source in your speakeasy workflow.",
	PreRunE: interactivity.GetMissingFlagsPreRun,
	RunE:    configureSources,
}

func configureInit() {
	rootCmd.AddCommand(configureCmd)
	configureSourcesInit()
}

func configureSourcesInit() {
	configureSourcesCmd.Flags().StringP("name", "n", "", "the name of the source to configure")

	configureCmd.AddCommand(configureSourcesCmd)
}

func configureSources(cmd *cobra.Command, args []string) error {
	if err := auth.Authenticate(cmd.Context(), false); err != nil {
		return err
	}

	name, err := cmd.Flags().GetString("name")
	if err != nil {
		return err
	}

	workingDir, err := os.Getwd()
	if err != nil {
		return err
	}

	var workflowFile *workflow.Workflow
	if workflowFile, _, _ = workflow.Load(workingDir); workflowFile == nil {
		workflowFile = &workflow.Workflow{
			Version: workflow.WorkflowVersion,
			Sources: make(map[string]workflow.Source),
			Targets: make(map[string]workflow.Target),
		}
	}

	var existingSource *workflow.Source
	if source, ok := workflowFile.Sources[name]; ok {
		existingSource = &source
	}

	if existingSource != nil {
		source, err := prompts.AddToSource(name, existingSource)
		if err != nil {
			return errors.Wrapf(err, "failed to add to source %s", name)
		}
		workflowFile.Sources[name] = *source
	} else {
		newName, source, err := prompts.PromptForNewSource(workflowFile)
		if err != nil {
			return errors.Wrap(err, "failed to prompt for a new source")
		}

		workflowFile.Sources[newName] = *source
	}

	if err := workflowFile.Validate(generate.GetSupportedLanguages()); err != nil {
		return errors.Wrapf(err, "failed to validate workflow file")
	}

	if _, err := os.Stat(".speakeasy"); os.IsNotExist(err) {
		err = os.MkdirAll(".speakeasy", 0o755)
		if err != nil {
			return err
		}
	}

	if err := workflow.Save(workingDir, workflowFile); err != nil {
		return errors.Wrapf(err, "failed to save workflow file")
	}

	return nil
}
