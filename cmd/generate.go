package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/speakeasy-api/sdk-generation/pkg/generate"
	"github.com/speakeasy-api/speakeasy/internal/sdkgen"
	"github.com/spf13/cobra"
)

var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate Client SDKs, OpenAPI specs (coming soon) and more (coming soon)",
	Long:  `TODO: add long description`,
	RunE:  generateExec,
}

var genSDKCmd = &cobra.Command{
	Use:   "sdk",
	Short: "Generating Client SDKs from OpenAPI specs (go, python, typescript(web/server), + more coming soon)",
	Long:  `TODO: add long description`,
}

func genInit() {
	rootCmd.AddCommand(generateCmd)
	genSDKInit()
}

//nolint:errcheck
func genSDKInit() {
	genSDKCmd.Flags().StringP("lang", "l", "go", fmt.Sprintf("language to generate sdk for (available options: [%s])", strings.Join(generate.SupportLangs, ", ")))

	genSDKCmd.Flags().StringP("schema", "s", "", "path to the openapi schema")
	genSDKCmd.MarkFlagRequired("schema")

	genSDKCmd.Flags().StringP("out", "o", "", "path to the output directory")
	genSDKCmd.MarkFlagRequired("out")

	genSDKCmd.Flags().StringP("baseurl", "b", "", "base URL for the api (only required if OpenAPI spec doesn't specify root server URLs")

	genSDKCmd.RunE = genSDKs

	generateCmd.AddCommand(genSDKCmd)
}

func generateExec(cmd *cobra.Command, args []string) error {
	return errors.New("no command provided")
}

func genSDKs(cmd *cobra.Command, args []string) error {
	lang, _ := cmd.Flags().GetString("lang")

	schemaPath, err := cmd.Flags().GetString("schema")
	if err != nil {
		return err
	}

	outDir, err := cmd.Flags().GetString("out")
	if err != nil {
		return err
	}

	baseURL, _ := cmd.Flags().GetString("baseurl")

	if err := sdkgen.Generate(cmd.Context(), lang, schemaPath, outDir, baseURL); err != nil {
		rootCmd.SilenceUsage = true
		return err
	}

	return nil
}
