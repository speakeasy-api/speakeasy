package cmd

import (
	"fmt"
	"github.com/manifoldco/promptui"
	"github.com/speakeasy-api/speakeasy/internal/auth"
	"github.com/speakeasy-api/speakeasy/internal/env"
	"github.com/speakeasy-api/speakeasy/internal/run"
	"github.com/speakeasy-api/speakeasy/internal/utils"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
	"strings"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "run the workflow(s) defined in your `.speakeasy/workflow.yaml` file.",
	Long: "run the workflow(s) defined in your `.speakeasy/workflow.yaml` file." + `
A workflow can consist of multiple targets that define a source OpenAPI document that can be downloaded from a URL, exist as a local file, or be created via merging multiple OpenAPI documents together and/or overlaying them with an OpenAPI overlay document.
A full workflow is capable of running the following steps:
  - Downloading source OpenAPI documents from a URL
  - Merging multiple OpenAPI documents together
  - Overlaying OpenAPI documents with an OpenAPI overlay document
  - Generating one or many SDKs from the resulting OpenAPI document
  - Compiling the generated SDKs

` + "If `speakeasy run` is run without any arguments it will run either the first target in the workflow or the first source in the workflow if there are no other targets or sources, otherwise it will prompt you to select a target or source to run.",
	PreRunE: getMissingFlagVals,
	RunE:    runFunc,
}

func runInit() {
	runCmd.Flags().StringP("target", "t", "", "target to run. specify 'all' to run all targets")
	runCmd.Flags().StringP("source", "s", "", "source to run. specify 'all' to run all sources")
	runCmd.Flags().StringP("installationURL", "i", "", "the language specific installation URL for installation instructions if the SDK is not published to a package manager")
	runCmd.Flags().BoolP("debug", "d", false, "enable writing debug files with broken code")
	runCmd.Flags().StringP("repo", "r", "", "the repository URL for the SDK, if the published (-p) flag isn't used this will be used to generate installation instructions")
	runCmd.Flags().StringP("repo-subdir", "b", "", "the subdirectory of the repository where the SDK is located in the repo, helps with documentation generation")

	rootCmd.AddCommand(runCmd)
}

func getMissingFlagVals(cmd *cobra.Command, args []string) error {
	wf, _, err := run.GetWorkflowAndDir()
	if err != nil {
		return err
	}

	sources, targets, err := run.ParseSourcesAndTargets()
	if err != nil {
		return err
	}

	target, err := cmd.Flags().GetString("target")
	if err != nil {
		return err
	}

	source, err := cmd.Flags().GetString("source")
	if err != nil {
		return err
	}

	if target == "" && source == "" {
		if len(wf.Targets) == 1 {
			target = targets[0]
		} else if len(wf.Targets) == 0 && len(wf.Sources) == 1 {
			source = sources[0]
		} else {
			// TODO update to use our proper interactive code
			prompt := promptui.Prompt{
				Label: fmt.Sprintf("Select a target (%s or 'all')", strings.Join(targets, ", ")),
				Validate: func(input string) error {
					if input == "" {
						return fmt.Errorf("target cannot be empty")
					}

					if input != "all" && !slices.Contains(targets, input) {
						return fmt.Errorf("invalid target")
					}

					return nil
				},
			}

			result, err := prompt.Run()
			if err != nil {
				return err
			}

			target = result
		}
	}

	if err := cmd.Flags().Set("target", target); err != nil {
		return err
	}

	if err := cmd.Flags().Set("source", source); err != nil {
		return err
	}

	return nil
}

func runFunc(cmd *cobra.Command, args []string) error {
	if err := auth.Authenticate(cmd.Context(), false); err != nil {
		return err
	}

	target, err := cmd.Flags().GetString("target")
	if err != nil {
		return err
	}

	source, err := cmd.Flags().GetString("source")
	if err != nil {
		return err
	}

	installationURL, err := cmd.Flags().GetString("installationURL")
	if err != nil {
		return err
	}

	debug, err := cmd.Flags().GetBool("debug")
	if err != nil {
		return err
	}

	repo, err := cmd.Flags().GetString("repo")
	if err != nil {
		return err
	}

	repoSubDir, err := cmd.Flags().GetString("repo-subdir")
	if err != nil {
		return err
	}

	if !utils.IsInteractive() || env.IsGithubAction() {
		return run.Run(cmd.Context(), target, source, genVersion, installationURL, repo, repoSubDir, debug, nil)
	} else {
		return run.RunWithVisualization(cmd.Context(), target, source, genVersion, installationURL, repo, repoSubDir, debug)
	}

	return nil
}
