package cmd

import (
	"fmt"
	"strings"

	markdown "github.com/MichaelMure/go-term-markdown"
	changelog "github.com/speakeasy-api/openapi-generation/v2"
	"github.com/speakeasy-api/openapi-generation/v2/pkg/generate"
	"github.com/speakeasy-api/speakeasy/internal/auth"
	"github.com/speakeasy-api/speakeasy/internal/config"
	"github.com/speakeasy-api/speakeasy/internal/merge"
	"github.com/speakeasy-api/speakeasy/internal/sdkgen"
	"github.com/speakeasy-api/speakeasy/internal/utils"
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
	Short: fmt.Sprintf("Generating Client SDKs from OpenAPI specs (%s + more coming soon)", strings.Join(generate.GetSupportedLanguages(), ", ")),
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
  packageName: github.com/speakeasy-api/speakeasy-client-sdk-go
  version: 0.1.0
generate:
  # baseServerUrl is optional, if not specified it will use the server URL from the OpenAPI document 
  baseServerUrl: https://api.speakeasyapi.dev 
`+"```"+`

Example gen.yaml file for Python SDK:

`+"```"+`
python:
  packageName: speakeasy-client-sdk-python
  version: 0.1.0
  description: Speakeasy API Client SDK for Python
  author: Speakeasy API
generate:
  # baseServerUrl is optional, if not specified it will use the server URL from the OpenAPI document 
  baseServerUrl: https://api.speakeasyapi.dev 
`+"```"+`

Example gen.yaml file for Typescript SDK:

`+"```"+`
typescript:
  packageName: speakeasy-client-sdk-typescript
  version: 0.1.0
  author: Speakeasy API
generate:
  # baseServerUrl is optional, if not specified it will use the server URL from the OpenAPI document 
  baseServerUrl: https://api.speakeasyapi.dev 
`+"```"+`

Example gen.yaml file for Java SDK:

`+"```"+`
java:
  groupID: dev.speakeasyapi
  artifactID: javasdk
  projectName: speakeasy-client-sdk-java
  version: 0.1.0
generate:
  # baseServerUrl is optional, if not specified it will use the server URL from the OpenAPI document 
  baseServerUrl: https://api.speakeasyapi.dev 
`+"```"+`

Example gen.yaml file for PHP SDK:

`+"```"+`
php:
  packageName: speakeasy-client-sdk-php
  namespace: "speakeasyapi\\sdk"
  version: 0.1.0
generate:
  # baseServerUrl is optional, if not specified it will use the server URL from the OpenAPI document 
  baseServerUrl: https://api.speakeasyapi.dev 
`+"```"+`

For additional documentation visit: https://docs.speakeasyapi.dev/docs/using-speakeasy/create-client-sdks/intro
`, strings.Join(generate.GetSupportedLanguages(), "\n	- ")),
	RunE: genSDKs,
}

var mergeCmd = &cobra.Command{
	Use:   "merge",
	Short: "Merge multiple OpenAPI documents into a single document",
	Long: `Merge multiple OpenAPI documents into a single document, useful for merging multiple OpenAPI documents into a single document for generating a client SDK.
Note: That any duplicate operations, components, etc. will be overwritten by the next document in the list.`,
	RunE: mergeExec,
}

var genSDKVersionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of the SDK generator",
	Long:  `Print the version number of the SDK generator including the latest changelog entry`,
	Run:   getLatestVersionInfo,
}

var genSDKChangelogCmd = &cobra.Command{
	Use:   "changelog",
	Short: "Prints information about changes to the SDK generator",
	Long:  `Prints information about changes to the SDK generator with the ability to filter by version and format the output for the terminal or parsing. By default it will print the latest changelog entry.`,
	RunE:  getChangelogs,
}

var genVersion string

func genInit() {
	rootCmd.AddCommand(generateCmd)

	genVersion = rootCmd.Version

	genSDKInit()
}

//nolint:errcheck
func genSDKInit() {
	genSDKCmd.Flags().StringP("lang", "l", "go", fmt.Sprintf("language to generate sdk for (available options: [%s])", strings.Join(generate.GetSupportedLanguages(), ", ")))

	genSDKCmd.Flags().StringP("schema", "s", "./openapi.yaml", "path to the openapi schema")
	genSDKCmd.MarkFlagRequired("schema")

	genSDKCmd.Flags().StringP("out", "o", "", "path to the output directory")
	genSDKCmd.MarkFlagRequired("out")

	genSDKCmd.Flags().BoolP("debug", "d", false, "enable writing debug files with broken code")

	genSDKCmd.Flags().BoolP("auto-yes", "y", false, "auto answer yes to all prompts")

	genSDKCmd.Flags().StringP("installationURL", "i", "", "the language specific installation URL for installation instructions if the SDK is not published to a package manager")
	genSDKCmd.Flags().BoolP("published", "p", false, "whether the SDK is published to a package manager or not, determines the type of installation instructions to generate")

	genSDKChangelogCmd.Flags().StringP("target", "t", "", "target version to get changelog from (default: the latest change)")
	genSDKChangelogCmd.Flags().StringP("previous", "p", "", "the version to get changelogs between this and the target version")
	genSDKChangelogCmd.Flags().StringP("specific", "s", "", "the version to get changelogs for")
	genSDKChangelogCmd.Flags().BoolP("raw", "r", false, "don't format the output for the terminal")

	mergeCmd.Flags().StringArrayP("schemas", "s", []string{}, "paths to the openapi schemas to merge")
	mergeCmd.MarkFlagRequired("schemas")
	mergeCmd.Flags().StringP("out", "o", "", "path to the output file")
	mergeCmd.MarkFlagRequired("out")

	genSDKCmd.AddCommand(genSDKVersionCmd)
	genSDKCmd.AddCommand(genSDKChangelogCmd)
	generateCmd.AddCommand(genSDKCmd)
	generateCmd.AddCommand(mergeCmd)
}

func generateExec(cmd *cobra.Command, args []string) error {
	return cmd.Help()
}

func genSDKs(cmd *cobra.Command, args []string) error {
	if err := auth.Authenticate(false); err != nil {
		return err
	}

	lang, _ := cmd.Flags().GetString("lang")

	schemaPath, err := cmd.Flags().GetString("schema")
	if err != nil {
		return err
	}

	outDir, err := cmd.Flags().GetString("out")
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

	installationURL, err := cmd.Flags().GetString("installationURL")
	if err != nil {
		return err
	}

	published, err := cmd.Flags().GetBool("published")
	if err != nil {
		return err
	}

	if err := sdkgen.Generate(cmd.Context(), config.GetCustomerID(), lang, schemaPath, outDir, genVersion, installationURL, debug, autoYes, published); err != nil {
		rootCmd.SilenceUsage = true

		return err
	}

	return nil
}

func getLatestVersionInfo(cmd *cobra.Command, args []string) {
	version := changelog.GetLatestVersion()

	changeLog := changelog.GetChangeLog(changelog.WithSpecificVersion(version))

	fmt.Printf("Version: %s\n\n", version)
	fmt.Println(string(markdown.Render("# CHANGELOG\n\n"+changeLog, 100, 0)))
}

func getChangelogs(cmd *cobra.Command, args []string) error {
	targetVersion, err := cmd.Flags().GetString("target")
	if err != nil {
		return err
	}

	previousVersion, err := cmd.Flags().GetString("previous")
	if err != nil {
		return err
	}

	specificVersion, err := cmd.Flags().GetString("specific")
	if err != nil {
		return err
	}

	raw, err := cmd.Flags().GetBool("raw")
	if err != nil {
		return err
	}

	opts := []changelog.Option{}

	if targetVersion != "" {
		opts = append(opts, changelog.WithTargetVersion(targetVersion))

		if previousVersion != "" {
			opts = append(opts, changelog.WithPreviousVersion(previousVersion))
		}
	} else if specificVersion != "" {
		opts = append(opts, changelog.WithSpecificVersion(specificVersion))
	} else {
		opts = append(opts, changelog.WithSpecificVersion(changelog.GetLatestVersion()))
	}

	changeLog := changelog.GetChangeLog(opts...)

	if raw {
		fmt.Println(changeLog)
		return nil
	}

	fmt.Println(string(markdown.Render("# CHANGELOG\n\n"+changeLog, 100, 0)))
	return nil
}

func mergeExec(cmd *cobra.Command, args []string) error {
	inSchemas, err := cmd.Flags().GetStringArray("schemas")
	if err != nil {
		return err
	}

	outFile, err := cmd.Flags().GetString("out")
	if err != nil {
		return err
	}

	if err := merge.MergeOpenAPIDocuments(inSchemas, outFile); err != nil {
		return err
	}

	fmt.Println(utils.Green(fmt.Sprintf("Successfully merged %d schemas into %s", len(inSchemas), outFile)))

	return nil
}
