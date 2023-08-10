package cmd

import (
	"fmt"
	"github.com/manifoldco/promptui"
	"github.com/speakeasy-api/openapi-generation/v2/pkg/errors"
	"github.com/speakeasy-api/speakeasy/internal/suggestions"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
	"strings"
)

var suggestCmd = &cobra.Command{
	Use:   "suggest",
	Short: "Validate an OpenAPI document and get fixes suggested by ChatGPT",
	Long: `The "suggest" command validates an OpenAPI spec and uses OpenAI's ChatGPT to suggest fixes to your spec.
You will need to set your OpenAI API key in a OPENAI_API_KEY environment variable. You will also need to authenticate with the Speakeasy API,
you must first create an API key via https://app.speakeasyapi.dev and then set the SPEAKEASY_API_KEY environment variable to the value of the API key.`,
	RunE: suggestFixesOpenAPI,
}

var severities = fmt.Sprintf("%s, %s, or %s", errors.SeverityError, errors.SeverityWarn, errors.SeverityHint)

func suggestInit() {
	suggestCmd.Flags().StringP("schema", "s", "", "path to the OpenAPI document")
	suggestCmd.Flags().BoolP("auto-approve", "a", false, "auto continue through all prompts")
	suggestCmd.Flags().StringP("output-file", "o", "", "output the modified file with suggested fixes applied to the specified path")
	suggestCmd.Flags().IntP("max-suggestions", "n", -1, "maximum number of llm suggestions to fetch, the default is no limit")
	suggestCmd.Flags().StringP("level", "l", "warn", fmt.Sprintf("%s. The minimum level of severity to request suggestions for", severities))
	suggestCmd.Flags().BoolP("serial", "", false, "do not parallelize requesting suggestions")
	suggestCmd.Flags().StringP("model", "m", "gpt-4-0613", "model to use when making llm suggestions (gpt-4-0613 recommended)")
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

	level, err := cmd.Flags().GetString("level")
	if err != nil {
		return err
	}

	severity := errors.Severity(level)
	if !slices.Contains([]errors.Severity{errors.SeverityError, errors.SeverityWarn, errors.SeverityHint}, severity) {
		return fmt.Errorf("level must be one of %s", severities)
	}

	outputFile, err := cmd.Flags().GetString("output-file")
	if err != nil {
		return err
	}

	if outputFile == "" {
		fmt.Println(promptui.Styler(promptui.FGWhite, promptui.FGItalic)("Specifying an output file with -o will allow you to automatically apply suggested fixes to the spec"))
		fmt.Println()
	}

	modelName, err := cmd.Flags().GetString("model")
	if err != nil {
		return err
	}

	if !strings.HasPrefix(modelName, "gpt-3.5") && !strings.HasPrefix(modelName, "gpt-4") {
		return fmt.Errorf("only gpt3.5 and gpt4 based models supported")
	}

	dontParallelize, err := cmd.Flags().GetBool("serial")
	if err != nil {
		return err
	}

	suggestionConfig := suggestions.Config{
		AutoContinue: autoApprove,
		Model:        modelName,
		OutputFile:   outputFile,
		Parallelize:  !dontParallelize,
		Level:        severity,
	}

	maxSuggestion, err := cmd.Flags().GetInt("max-suggestions")
	if err != nil {
		return err
	}

	if maxSuggestion != -1 {
		suggestionConfig.MaxSuggestions = &maxSuggestion
	}

	if err := suggestions.StartSuggest(cmd.Context(), schemaPath, &suggestionConfig, true); err != nil {
		rootCmd.SilenceUsage = true

		return err
	}

	uploadCommand := promptui.Styler(promptui.FGCyan, promptui.FGBold)("speakeasy api register-schema --schema=" + schemaPath)
	fmt.Printf("\nYou can upload your schema to Speakeasy using the following command:\n%s\n", uploadCommand)

	return nil
}
