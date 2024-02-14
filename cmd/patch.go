package cmd

import (
	"fmt"
	"os"
	"slices"

	"github.com/speakeasy-api/openapi-generation/v2/pkg/generate"
	"github.com/speakeasy-api/openapi-generation/v2/pkg/patch"
	"github.com/spf13/cobra"
)

var patchCommand = &cobra.Command{
	Use:    "patch",
	Short:  "Creates a patch file for adding dependencies to a generated SDK",
	Long:   `Creates a patch file for adding dependencies to a generated SDK, by creating a dependencies.patch file from the difference between local changes and the original dependency files in an SDK.`,
	Hidden: true,
}

func patchInit() {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	patchCommand.Flags().StringP("target", "t", "", "the target to patch, e.g. go, python, typescript, etc")
	patchCommand.MarkFlagRequired("target")
	patchCommand.Flags().StringP("working-dir", "w", wd, "override for working directory to use for the patch command")

	patchCommand.RunE = patchExec
	rootCmd.AddCommand(patchCommand)
}

func patchExec(cmd *cobra.Command, args []string) error {
	workingDir, err := cmd.Flags().GetString("working-dir")
	if err != nil {
		return err
	}

	target, err := cmd.Flags().GetString("target")
	if err != nil {
		return err
	}

	if !slices.Contains(generate.GetSupportedLanguages(), target) {
		return fmt.Errorf("unsupported target %s", target)
	}

	return patch.CreatePatch(workingDir, target)
}
