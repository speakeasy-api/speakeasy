package cmd

import (
	"github.com/speakeasy-api/speakeasy/internal/validation"
	"github.com/spf13/cobra"
)

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate OpenAPI documents + more (coming soon)",
	Long:  `The "validate" command provides a set of commands for validating OpenAPI docs and more (coming soon).`,
	RunE:  validateExec,
}

var validateOpenAPICmd = &cobra.Command{
	Use:   "openapi",
	Short: "Validate an OpenAPI document",
	Long:  `Validates an OpenAPI document is valid and conforms to the Speakeasy OpenAPI specification.`,
}

func validateInit() {
	rootCmd.AddCommand(validateCmd)
	validateOpenInit()
}

//nolint:errcheck
func validateOpenInit() {
	validateOpenAPICmd.Flags().StringP("schema", "s", "", "path to the OpenAPI document")
	validateOpenAPICmd.MarkFlagRequired("schema")

	validateOpenAPICmd.RunE = validateOpenAPI

	validateCmd.AddCommand(validateOpenAPICmd)
}

func validateExec(cmd *cobra.Command, args []string) error {
	return cmd.Help()
}

func validateOpenAPI(cmd *cobra.Command, args []string) error {
	schemaPath, err := cmd.Flags().GetString("schema")
	if err != nil {
		return err
	}

	if errs := validation.ValidateOpenAPI(cmd.Context(), schemaPath); len(errs) > 0 {
		rootCmd.SilenceUsage = true
		return errs[0]
	}

	return nil
}
