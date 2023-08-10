package cmd

import (
	"fmt"
	"github.com/manifoldco/promptui"
	"github.com/speakeasy-api/speakeasy/internal/sdkgen"
	"github.com/speakeasy-api/speakeasy/internal/utils"
	"github.com/speakeasy-api/speakeasy/internal/validation"
	"github.com/spf13/cobra"
)

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate OpenAPI documents + more (coming soon)",
	Long:  `The "validate" command provides a set of commands for validating OpenAPI docs and more (coming soon).`,
	RunE:  utils.InteractiveRunFn("What do you want to validate?"),
}

var validateOpenAPICmd = &cobra.Command{
	Use:   "openapi",
	Short: "Validate an OpenAPI document",
	Long:  `Validates an OpenAPI document is valid and conforms to the Speakeasy OpenAPI specification.`,
	RunE:  validateOpenAPI,
}

var validateConfigCmd = &cobra.Command{
	Use:   "config",
	Short: "Validates a Speakeasy configuration file for SDK generation",
	Long:  `Validates a Speakeasy configuration file for SDK generation.`,
	RunE:  validateConfig,
}

func validateInit() {
	rootCmd.AddCommand(validateCmd)
	validateOpenAPIInit()
	validateConfigInit()
}

//nolint:errcheck
func validateOpenAPIInit() {
	validateOpenAPICmd.Flags().BoolP("output-hints", "o", false, "output validation hints in addition to warnings/errors")
	validateOpenAPICmd.Flags().StringP("schema", "s", "", "path to the OpenAPI document")
	_ = validateOpenAPICmd.MarkFlagRequired("schema")

	validateCmd.AddCommand(validateOpenAPICmd)
}

func validateConfigInit() {
	validateConfigCmd.Flags().StringP("dir", "d", "", "path to the directory containing the Speakeasy configuration file")
	_ = validateConfigCmd.MarkFlagRequired("dir")

	validateCmd.AddCommand(validateConfigCmd)
}

func validateOpenAPI(cmd *cobra.Command, args []string) error {
	// no authentication required for validating specs

	schemaPath, err := cmd.Flags().GetString("schema")
	if err != nil {
		return err
	}

	outputHints, err := cmd.Flags().GetBool("output-hints")
	if err != nil {
		return err
	}

	if err := validation.ValidateOpenAPI(cmd.Context(), schemaPath, outputHints); err != nil {
		rootCmd.SilenceUsage = true

		return err
	}

	uploadCommand := promptui.Styler(promptui.FGCyan, promptui.FGBold)("speakeasy api register-schema --schema=" + schemaPath)
	fmt.Printf("\nYou can upload your schema to Speakeasy using the following command:\n%s\n", uploadCommand)

	return nil
}

func validateConfig(cmd *cobra.Command, args []string) error {
	// no authentication required for validating configs

	dir, err := cmd.Flags().GetString("dir")
	if err != nil {
		return err
	}

	if err := sdkgen.ValidateConfig(cmd.Context(), dir); err != nil {
		rootCmd.SilenceUsage = true

		return fmt.Errorf(utils.Red("%s"), err)
	}

	fmt.Printf("%s\n", utils.Green("Config valid ✓"))

	return nil
}
