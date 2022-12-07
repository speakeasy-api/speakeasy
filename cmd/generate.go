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
  version: 0.1.0
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
  author: Speakeasy API
# baseserverurl optional, if not specified it will use the server URL from the OpenAPI spec 
# this can also be provided via the --baseurl flag when calling the command line
baseserverurl: https://api.speakeasyapi.dev 
`+"```"+`

Example gen.yaml file for Typescript SDK:

`+"```"+`
typescript:
  packagename: speakeasy-client-sdk-typescript
  version: 0.1.0
  author: Speakeasy API
# baseserverurl optional, if not specified it will use the server URL from the OpenAPI spec
# this can also be provided via the --baseurl flag when calling the command line
baseserverurl: https://api.speakeasyapi.dev
`+"```"+`

Example gen.yaml file for Java SDK:

`+"```"+`
java:
  packagename: dev.speakeasyapi.javasdk
  projectname: speakeasy-client-sdk-java
  version: 0.1.0
# baseserverurl optional, if not specified it will use the server URL from the OpenAPI spec
# this can also be provided via the --baseurl flag when calling the command line
baseserverurl: https://api.speakeasyapi.dev
`+"```"+`

For additional documentation visit: https://docs.speakeasyapi.dev/docs/using-speakeasy/create-client-sdks/intro

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
	- LICENSE

`, strings.Join(generate.SupportLangs, "\n	- ")),
}

var genVersion string

func genInit() {
	rootCmd.AddCommand(generateCmd)

	genVersion = rootCmd.Version

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

	genSDKCmd.Flags().BoolP("debug", "d", false, "enable writing debug files with broken code")

	genSDKCmd.Flags().BoolP("auto-yes", "y", false, "auto answer yes to all prompts")

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

	baseURL, err := cmd.Flags().GetString("baseurl")
	if err != nil {
		return err
	}

	debug, err := cmd.Flags().GetBool("debug")
	if err != nil {
		return err
	}

	autoYes, err := cmd.Flags().GetBool("auto-yes")
	if err != nil {
		return err
	}

	if err := sdkgen.Generate(cmd.Context(), vCfg.GetString("id"), lang, schemaPath, outDir, baseURL, genVersion, debug, autoYes); err != nil {
		rootCmd.SilenceUsage = true
		return err
	}

	return nil
}
