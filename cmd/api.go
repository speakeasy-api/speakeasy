package cmd

import (
	"github.com/speakeasy-api/speakeasy/internal/api"
	"github.com/speakeasy-api/speakeasy/internal/utils"
	"github.com/spf13/cobra"
)

var apiCmd = &cobra.Command{
	Use:   "api",
	Short: "Access the Speakeasy API via the CLI",
	Long: `Provides access to the Speakeasy API via the CLI.
To authenticate with the Speakeasy API, you must first create an API key via https://app.speakeasyapi.dev
and then set the SPEAKEASY_API_KEY environment variable to the value of the API key.`,
	RunE: utils.InteractiveRunFn("Choose an API endpoint:"),
}

func apiInit() {
	api.RegisterAPICommands(apiCmd)
	rootCmd.AddCommand(apiCmd)
}
