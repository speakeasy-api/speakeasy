package cmd

import (
	goerr "errors"
	"fmt"
	"github.com/manifoldco/promptui"
	"github.com/speakeasy-api/openapi-generation/v2/pkg/errors"
	"github.com/speakeasy-api/speakeasy/internal/suggestions"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
	"os"
	"strings"
)

var suggestCmd = &cobra.Command{
	Use:   "suggest",
	Short: "Validate an OpenAPI document and get fixes suggested by ChatGPT",
	Long: `The "suggest" command validates an OpenAPI spec and uses OpenAI's ChatGPT to suggest fixes to your spec.
You can use the Speakeasy OpenAI key within our platform limits, or you may set your own using the OPENAI_API_KEY environment variable. You will also need to authenticate with the Speakeasy API,
you must first create an API key via https://app.speakeasyapi.dev and then set the SPEAKEASY_API_KEY environment variable to the value of the API key.`,
	RunE: suggestFixesOpenAPI,
}

var severities = fmt.Sprintf("%s, %s, or %s", errors.SeverityError, errors.SeverityWarn, errors.SeverityHint)

func suggestInit() {
	suggestCmd.Flags().StringP("header", "H", "", "header key to use if authentication is required for downloading schema from remote URL")
	suggestCmd.Flags().String("token", "", "token value to use if authentication is required for downloading schema from remote URL")
	suggestCmd.Flags().StringP("schema", "s", "./openapi.yaml", "local path to a directory containing OpenAPI schema(s), or a single OpenAPI schema, or a remote URL to an OpenAPI schema")
	suggestCmd.Flags().BoolP("auto-approve", "a", false, "auto continue through all prompts")
	suggestCmd.Flags().StringP("output-file", "o", "", "output the modified file with suggested fixes applied to the specified path")
	suggestCmd.Flags().IntP("max-suggestions", "n", -1, "maximum number of llm suggestions to fetch, the default is no limit")
	suggestCmd.Flags().StringP("level", "l", "warn", fmt.Sprintf("%s. The minimum level of severity to request suggestions for", severities))
	suggestCmd.Flags().BoolP("serial", "", false, "do not parallelize requesting suggestions")
	suggestCmd.Flags().StringP("model", "m", "gpt-4-0613", "model to use when making llm suggestions (gpt-4-0613 recommended)")
	suggestCmd.Flags().BoolP("summary", "y", false, "show a summary of the remaining validation errors and their counts")
	suggestCmd.Flags().IntP("validation-loops", "v", -1, "number of times to run the validation loop, the default is no limit (only used in parallelized implementation)")
	suggestCmd.Flags().IntP("num-specs", "c", -1, "number of specs to run suggest on, the default is no limit")
	suggestCmd.Flags().StringP("cache-folder", "", "", "caches computations into a given folder")
	suggestCmd.Flags().BoolP("example-experiment", "", false, "enables the example experiment for the suggest command, generating an updated document with examples for all primitives.")
	_ = suggestCmd.MarkFlagRequired("schema")
	rootCmd.AddCommand(suggestCmd)
}

func suggestFixesOpenAPI(cmd *cobra.Command, args []string) error {
	// no authentication required for validating specs

	schemaPath, err := cmd.Flags().GetString("schema")
	if err != nil {
		return err
	}

	header, err := cmd.Flags().GetString("header")
	if err != nil {
		return err
	}

	token, err := cmd.Flags().GetString("token")
	if err != nil {
		return err
	}

	schemaPathFileInfo, _ := os.Stat(schemaPath)

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

	if schemaPathFileInfo != nil && schemaPathFileInfo.IsDir() && outputFile != "" {
		return goerr.New("cannot specify an output file when running suggest on a directory of specs")
	}

	summary, err := cmd.Flags().GetBool("summary")
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
		Summary:      summary,
	}

	maxSuggestions, err := cmd.Flags().GetInt("max-suggestions")
	if err != nil {
		return err
	}

	if maxSuggestions != -1 {
		suggestionConfig.MaxSuggestions = &maxSuggestions
	}

	numSpecs, err := cmd.Flags().GetInt("num-specs")
	if err != nil {
		return err
	}

	if numSpecs != -1 {
		suggestionConfig.NumSpecs = &numSpecs
	}

	validationLoops, err := cmd.Flags().GetInt("validation-loops")
	if err != nil {
		return err
	}

	if validationLoops != -1 {
		suggestionConfig.ValidationLoops = &validationLoops
	}
	cacheFolder, err := cmd.Flags().GetString("cache-folder")
	if err != nil {
		return err
	}

	if exampleExperiment, _ := cmd.Flags().GetBool("example-experiment"); exampleExperiment {
		if len(cacheFolder) == 0 {
			return goerr.New("cache-folder is required for example-experiment")
		}
		// check its a valid directory
		// if it doesn't exist, create it
		err = os.MkdirAll(cacheFolder, os.ModePerm)
		if err != nil {
			return err
		}
		cacheFolderFileInfo, err := os.Stat(cacheFolder)
		if err != nil {
			return err
		}
		if !cacheFolderFileInfo.IsDir() {
			return goerr.New("cache-folder must be a directory")
		}
		return suggestions.StartExampleExperiment(cmd.Context(), schemaPath, cacheFolder, outputFile)
	}

	isDir := schemaPathFileInfo != nil && schemaPathFileInfo.IsDir()
	err = suggestions.StartSuggest(cmd.Context(), schemaPath, header, token, isDir, &suggestionConfig)
	if err != nil {
		rootCmd.SilenceUsage = true
		return err
	}

	uploadCommand := promptui.Styler(promptui.FGCyan, promptui.FGBold)("speakeasy api register-schema --schema=" + schemaPath)
	fmt.Printf("\nYou can upload your schema to Speakeasy using the following command:\n%s\n", uploadCommand)

	return nil
}
