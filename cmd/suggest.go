package cmd

import (
	"fmt"
	"github.com/manifoldco/promptui"
	"github.com/speakeasy-api/speakeasy/internal/validation"
	"github.com/spf13/cobra"
)

var suggestCmd = &cobra.Command{
	Use:   "suggest",
	Short: "Validate an OpenAPI document and get fixes suggested by ChatGPT",
	Long: `The "suggest" command validates an OpenAPI spec and uses OpenAI's ChatGPT to suggest fixes to your spec.
You will need to set your OpenAI API key in a OPENAI_API_KEY environment variable. You will also need to authenticate with the Speakeasy API,
you must first create an API key via https://app.speakeasyapi.dev and then set the SPEAKEASY_API_KEY environment variable to the value of the API key.`,
	RunE: suggestFixesOpenAPI,
}

func suggestInit() {
	suggestCmd.Flags().StringP("schema", "s", "", "path to the OpenAPI document")
	_ = suggestCmd.MarkFlagRequired("schema")
	rootCmd.AddCommand(suggestCmd)
}

func suggestFixesOpenAPI(cmd *cobra.Command, args []string) error {
	// no authentication required for validating specs

	schemaPath, err := cmd.Flags().GetString("schema")
	if err != nil {
		return err
	}

	if err := validation.ValidateOpenAPI(cmd.Context(), schemaPath, true); err != nil {
		rootCmd.SilenceUsage = true

		return err
	}

	uploadCommand := promptui.Styler(promptui.FGCyan, promptui.FGBold)("speakeasy api register-schema --schema=" + schemaPath)
	fmt.Printf("\nYou can upload your schema to Speakeasy using the following command:\n%s\n", uploadCommand)

	return nil
}
