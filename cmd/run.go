package cmd

import (
	"github.com/speakeasy-api/speakeasy/internal/auth"
	"github.com/speakeasy-api/speakeasy/internal/run"
	"github.com/spf13/cobra"
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
	RunE: runFunc,
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

	if err := run.Run(cmd.Context(), target, source, genVersion, installationURL, repo, repoSubDir, debug); err != nil {
		return err
	}

	return nil
}
