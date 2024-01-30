package cmd

import (
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/pkg/errors"
	"github.com/speakeasy-api/openapi-generation/v2/pkg/generate"
	config "github.com/speakeasy-api/sdk-gen-config"
	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/speakeasy-api/speakeasy/internal/auth"
	"github.com/speakeasy-api/speakeasy/internal/charm"
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
	Short:   "Configure new or existing sources.",
	Long:    "Guided prompts to configure a new or existing source in your speakeasy workflow.",
	PreRunE: interactivity.GetMissingFlagsPreRun,
	RunE:    configureSources,
}

var configureTargetCmd = &cobra.Command{
	Use:     "targets",
	Short:   "Configure new target.",
	Long:    "Guided prompts to configure a new target in your speakeasy workflow.",
	PreRunE: interactivity.GetMissingFlagsPreRun,
	RunE:    configureTarget,
}

func configureInit() {
	rootCmd.AddCommand(configureCmd)
	configureSourcesInit()
	configureTargetInit()
}

func configureSourcesInit() {
	configureSourcesCmd.Flags().StringP("id", "i", "", "the name of an existing target to configure")
	configureSourcesCmd.Flags().BoolP("new", "n", false, "configure a new target")

	configureCmd.AddCommand(configureSourcesCmd)
}

func configureTargetInit() {
	configureTargetCmd.Flags().StringP("id", "i", "", "the name of an existing target to configure")
	configureTargetCmd.Flags().BoolP("new", "n", false, "configure a new target")
	configureCmd.AddCommand(configureTargetCmd)
}

func configureSources(cmd *cobra.Command, args []string) error {
	if err := auth.Authenticate(cmd.Context(), false); err != nil {
		return err
	}

	id, err := cmd.Flags().GetString("id")
	if err != nil {
		return err
	}

	newSource, err := cmd.Flags().GetBool("new")
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

	var existingSourceName string
	var existingSource *workflow.Source
	if source, ok := workflowFile.Sources[id]; ok {
		existingSourceName = id
		existingSource = &source
	}

	var sourceOptions []string
	for sourceName := range workflowFile.Sources {
		sourceOptions = append(sourceOptions, sourceName)
	}
	sourceOptions = append(sourceOptions, "new")

	if !newSource && existingSource == nil {
		prompt := charm.NewSelectPrompt("What source would you like to configure?", "You may choose an existing source or create a new source.", sourceOptions, &existingSourceName)
		if _, err := tea.NewProgram(charm.NewForm(huh.NewForm(prompt),
			"Let's configure a source for your workflow.")).
			Run(); err != nil {
			return err
		}
		if existingSourceName == "new" {
			existingSourceName = ""
		} else {
			if source, ok := workflowFile.Sources[existingSourceName]; ok {
				existingSource = &source
			}
		}
	}

	if existingSource != nil {
		source, err := prompts.AddToSource(existingSourceName, existingSource)
		if err != nil {
			return errors.Wrapf(err, "failed to add to source %s", existingSourceName)
		}
		workflowFile.Sources[existingSourceName] = *source
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

func configureTarget(cmd *cobra.Command, args []string) error {
	if err := auth.Authenticate(cmd.Context(), false); err != nil {
		return err
	}

	id, err := cmd.Flags().GetString("id")
	if err != nil {
		return err
	}

	newTarget, err := cmd.Flags().GetBool("new")
	if err != nil {
		return err
	}

	workingDir, err := os.Getwd()
	if err != nil {
		return err
	}

	var workflowFile *workflow.Workflow
	if workflowFile, _, err = workflow.Load(workingDir); err != nil || workflowFile == nil || len(workflowFile.Sources) == 0 {
		return errors.New("you must have a source to configure a target try speakeasy quickstart")
	}

	existingTarget := ""
	if _, ok := workflowFile.Targets[id]; ok {
		existingTarget = id
	}

	var existingTargets []string
	for targetName := range workflowFile.Targets {
		existingTargets = append(existingTargets, targetName)
	}
	targetOptions := append(existingTargets, "new")

	if !newTarget && existingTarget == "" {
		prompt := charm.NewSelectPrompt("What target would you like to configure?", "You may choose an existing target or create a new target.", targetOptions, &existingTarget)
		if _, err := tea.NewProgram(charm.NewForm(huh.NewForm(prompt),
			"Let's configure a target for your workflow.")).
			Run(); err != nil {
			return err
		}
		if existingTarget == "new" {
			existingTarget = ""
		}
	}

	var targetName string
	var target *workflow.Target
	if existingTarget == "" {
		// If we add multiple targets to one workflow file the out dir of a target cannot be the root dir
		if err := prompts.MoveOutDir(workflowFile, existingTargets); err != nil {
			return err
		}

		targetName, target, err = prompts.PromptForNewTarget(workflowFile, "", "", "")
		if err != nil {
			return err
		}

		workflowFile.Targets[targetName] = *target
	}

	targetConfig, err := prompts.PromptForTargetConfig(targetName, target)
	if err != nil {
		return err
	}

	outDir := workingDir
	if target.Output != nil {
		outDir = *target.Output
	}

	if err := config.SaveConfig(outDir, targetConfig); err != nil {
		return errors.Wrapf(err, "failed to save config file for target %s", targetName)
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
