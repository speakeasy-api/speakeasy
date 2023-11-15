package cmd

import (
	"fmt"
	"os"

	"github.com/manifoldco/promptui"
	"github.com/speakeasy-api/speakeasy/internal/config"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/sdkgen"
	"github.com/speakeasy-api/speakeasy/internal/utils"
	"github.com/speakeasy-api/speakeasy/internal/validation"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
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
	validateOpenAPICmd.Flags().StringP("schema", "s", "", "local filepath or URL for the OpenAPI schema")
	_ = validateOpenAPICmd.MarkFlagRequired("schema")

	validateOpenAPICmd.Flags().StringP("header", "H", "", "header key to use if authentication is required for downloading schema from remote URL")
	validateOpenAPICmd.Flags().String("token", "", "token value to use if authentication is required for downloading schema from remote URL")

	validateOpenAPICmd.Flags().Int("max-validation-warnings", 1000, "limit the number of warnings to output (default 1000, 0 = no limit)")
	validateOpenAPICmd.Flags().Int("max-validation-errors", 1000, "limit the number of errors to output (default 1000, 0 = no limit)")

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

	header, err := cmd.Flags().GetString("header")
	if err != nil {
		return err
	}

	token, err := cmd.Flags().GetString("token")
	if err != nil {
		return err
	}

	outputHints, err := cmd.Flags().GetBool("output-hints")
	if err != nil {
		return err
	}

	maxWarns, err := cmd.Flags().GetInt("max-validation-warnings")
	if err != nil {
		return err
	}

	maxErrs, err := cmd.Flags().GetInt("max-validation-errors")
	if err != nil {
		return err
	}

	limits := &validation.OutputLimits{
		MaxErrors:   maxErrs,
		MaxWarns:    maxWarns,
		OutputHints: outputHints,
	}

	logProxyFields := collectValidationLoggingFields(schemaPath)
	log.NewLogger("").InfoProxy("Attempted Spec Validation", logProxyFields...)

	if err := validation.ValidateOpenAPI(cmd.Context(), schemaPath, header, token, limits); err != nil {
		rootCmd.SilenceUsage = true
		logProxyFields = append(logProxyFields, zap.Error(err))
		// TODO: Move Upstream work in validation summary.
		log.NewLogger("").ErrorProxy("Failed Spec Validation", logProxyFields...)
		return err
	}

	log.NewLogger("").InfoProxy("Successful Spec Validation", logProxyFields...)

	uploadCommand := promptui.Styler(promptui.FGCyan, promptui.FGBold)("speakeasy api register-schema --schema=" + schemaPath)
	fmt.Printf("\nYou can upload your schema to Speakeasy using the following command:\n%s\n", uploadCommand)

	return nil
}

func collectValidationLoggingFields(schemaPath string) []zap.Field {
	runLocation := os.Getenv("SPEAKEASY_RUN_LOCATION")
	if runLocation == "" {
		runLocation = "cli"
	}
	// TODO: find some way for the cli to detect version in use.
	logProxyFields := []zap.Field{
		zap.String("run_location", runLocation),
		zap.String("schema_path", schemaPath),
		zap.String("customer_id", config.GetCustomerID()),
		zap.String("command", "validate_openapi"),
	}

	return logProxyFields
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
