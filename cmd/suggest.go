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
	suggestCmd.Flags().BoolP("auto-approve", "a", false, "auto continue through all prompts")
	suggestCmd.Flags().IntP("max-suggestions", "n", -1, "maximum number of llm suggestions to fetch, the default is no limit")
	_ = suggestCmd.MarkFlagRequired("schema")
	rootCmd.AddCommand(suggestCmd)
}

func suggestFixesOpenAPI(cmd *cobra.Command, args []string) error {
	// no authentication required for validating specs

	schemaPath, err := cmd.Flags().GetString("schema")
	if err != nil {
		return err
	}

	autoApprove, err := cmd.Flags().GetBool("auto-approve")
	if err != nil {
		return err
	}

	suggestionConfig := validation.SuggestionsConfig{
		AutoContinue: autoApprove,
	}

	maxSuggestion, err := cmd.Flags().GetInt("max-suggestions")
	if err != nil {
		return err
	}

	if maxSuggestion != -1 {
		suggestionConfig.MaxSuggestions = &maxSuggestion
	}

	if err := validation.ValidateOpenAPI(cmd.Context(), schemaPath, &suggestionConfig); err != nil {
		rootCmd.SilenceUsage = true

		return err
	}

	uploadCommand := promptui.Styler(promptui.FGCyan, promptui.FGBold)("speakeasy api register-schema --schema=" + schemaPath)
	fmt.Printf("\nYou can upload your schema to Speakeasy using the following command:\n%s\n", uploadCommand)

	return nil
}
