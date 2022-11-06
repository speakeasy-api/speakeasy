package cmd

import (
	"fmt"
	"strings"

	"github.com/speakeasy-api/openapi-generation/pkg/generate"
	"github.com/speakeasy-api/speakeasy/internal/sdkgen"
	"github.com/spf13/cobra"
)

var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate Client SDKs, OpenAPI specs from request logs (coming soon) and more",
	Long:  `The "generate" command provides a set of commands for generating client SDKs, OpenAPI specs (coming soon) and more (coming soon).`,
	RunE:  generateExec,
}

var genSDKCmd = &cobra.Command{
	Use:   "sdk",
	Short: fmt.Sprintf("Generating Client SDKs from OpenAPI specs (%s + more coming soon)", strings.Join(generate.SupportLangs, ", ")),
	Long: fmt.Sprintf(`Using the "speakeasy generate sdk" command you can generate client SDK packages for various languages
that are ready to use and publish to your favorite package registry.

The following languages are currently supported:
	- %s
	- more coming soon

By default the command will generate a Go SDK, but you can specify a different language using the --lang flag.
It will also use generic defaults for things such as package name (openapi), etc.

# Configuration

To configure the package of the generated SDKs you can config a "gen.yaml" file in the root of the output directory.

Example gen.yaml file for Go SDK:

`+"```"+`
go:
  packagename: github.com/speakeasy-api/speakeasy-client-sdk-go
# baseserverurl optional, if not specified it will use the server URL from the OpenAPI spec 
# this can also be provided via the --baseurl flag when calling the command line
baseserverurl: https://api.speakeasyapi.dev 
`+"```"+`

Example gen.yaml file for Python SDK:

`+"```"+`
python:
  packagename: speakeasy-client-sdk-python
  version: 0.1.0
  description: Speakeasy API Client SDK for Python
# baseserverurl optional, if not specified it will use the server URL from the OpenAPI spec 
# this can also be provided via the --baseurl flag when calling the command line
baseserverurl: https://api.speakeasyapi.dev 
`+"```"+`

## Configuring Comments

By default the generated SDKs will include comments for each operation and model. You can configure the comments by adding a `+"`comments`"+` section to the `+"`gen.yaml`"+` file.
 
Example gen.yaml file:

`+"```"+`
go:
  packagename: github.com/speakeasy-api/speakeasy-client-sdk-go
comments:
  disabled: true                         # disable all comments
  omitdescriptionifsummarypresent: true  # if true and comments enabled, the description will be omitted if a summary is present for an operation
# baseserverurl optional, if not specified it will use the server URL from the OpenAPI spec 
# this can also be provided via the --baseurl flag when calling the command line
baseserverurl: https://api.speakeasyapi.dev 
`+"```"+`

# Ignore Files

The SDK generator will clear the output directory before generating the SDKs, to ensure old files are removed. 
If you have any files you want to keep you can place a ".genignore" file in the root of the output directory.
The ".genignore" file follows the same syntax as a ".gitignore" file.

By default (without a .genignore file/folders) the SDK generator will ignore the following files:
	- gen.yaml
	- .genignore
	- .gitignore
	- .git
	- README.md
	- readme.md

`, strings.Join(generate.SupportLangs, "\n	- ")),
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
	return cmd.Help()
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

	if err := sdkgen.Generate(cmd.Context(), vCfg.GetString("id"), lang, schemaPath, outDir, baseURL); err != nil {
		rootCmd.SilenceUsage = true
		return err
	}

	return nil
}
