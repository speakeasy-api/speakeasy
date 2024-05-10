package cmd

import (
	"fmt"

	"github.com/speakeasy-api/speakeasy/internal/updates"
	"github.com/spf13/cobra"
)

func updateInit(version, artifactArch string) {
	updateCmd := &cobra.Command{
		Use:   "update",
		Short: "Update the Speakeasy CLI to the latest version",
		Long:  `Updates the Speakeasy CLI in-place to the latest version available by downloading from Github and replacing the current binary`,
		RunE:  update(version, artifactArch),
	}

	updateCmd.Flags().IntP("timeout", "t", 30, "timeout in seconds for the update to complete")

	rootCmd.AddCommand(updateCmd)
}

func update(version, artifactArch string) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		timeout, err := cmd.Flags().GetInt("timeout")
		if err != nil {
			return err
		}

		newVersion, err := updates.Update(cmd.Context(), version, artifactArch, timeout)
		if err != nil {
			return err
		}

		if newVersion == "" {
			fmt.Println("Already up to date")
		} else {
			fmt.Println("Updated to version", newVersion)
		}

		return nil
	}
}
