package cmd

import (
	"github.com/speakeasy-api/speakeasy/internal/api"
	"github.com/spf13/cobra"
)

var apiCmd = &cobra.Command{
	Use:   "api",
	Short: "TBD",
	Long:  `TBD`,
	RunE:  apiExec,
}

func apiInit() {
	api.RegisterAPICommands(apiCmd)
	rootCmd.AddCommand(apiCmd)
}

func apiExec(cmd *cobra.Command, args []string) error {
	return cmd.Help()
}
