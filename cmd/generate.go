package cmd

import (
	"fmt"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"strings"

	"github.com/speakeasy-api/speakeasy/internal/usagegen"
	"github.com/speakeasy-api/speakeasy/internal/utils"
	"golang.org/x/exp/slices"

	markdown "github.com/MichaelMure/go-term-markdown"
	changelog "github.com/speakeasy-api/openapi-generation/v2"
	"github.com/speakeasy-api/openapi-generation/v2/changelogs"
	"github.com/speakeasy-api/openapi-generation/v2/pkg/generate"
	"github.com/speakeasy-api/speakeasy/internal/auth"
	"github.com/speakeasy-api/speakeasy/internal/config"
	"github.com/speakeasy-api/speakeasy/internal/sdkgen"
	"github.com/spf13/cobra"
)

func SDKSupportedLanguageTargets() []string {
	languages := make([]string, 0)
	for _, lang := range generate.GetSupportedLanguages() {
		if lang == "docs" {
			continue
		}

		languages = append(languages, lang)
	}

	return languages
}

var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate client SDKs, docsites, and more",
	Long:  `The "generate" command provides a set of commands for generating client SDKs, OpenAPI specs (coming soon) and more (coming soon).`,
	RunE:  utils.InteractiveRunFn("What do you want to generate?"),
}

var genSDKCmd = &cobra.Command{
	Use:   "sdk",
	Short: fmt.Sprintf("Generating Client SDKs from OpenAPI specs (%s + more coming soon)", strings.Join(SDKSupportedLanguageTargets(), ", ")),
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

Example gen.yaml file for C# SDK:

`+"```"+`
csharp:
  version: 0.1.0
  author: Speakeasy
  maxMethodParams: 0
  packageName: SpeakeasySDK
generate:
  # baseServerUrl is optional, if not specified it will use the server URL from the OpenAPI document
  baseServerUrl: https://api.speakeasyapi.dev 
`+"```"+`

For additional documentation visit: https://docs.speakeasyapi.dev/docs/using-speakeasy/create-client-sdks/intro
`, strings.Join(SDKSupportedLanguageTargets(), "\n	- ")),
	PreRunE: utils.GetMissingFlagsPreRun,
	RunE:    genSDKs,
}

var genUsageSnippetCmd = &cobra.Command{
	Use:   "usage",
	Short: fmt.Sprintf("Generate standalone usage snippets for SDKs in (%s)", strings.Join(usagegen.SupportedLanguagesUsageSnippets, ", ")),
	Long: fmt.Sprintf(`Using the "speakeasy generate usage" command you can generate usage snippets for various SDKs.

The following languages are currently supported:
	- %s
	- more coming soon

You can generate usage snippets by OperationID or by Namespace. By default this command will write to stdout.

You can also select to write to a file or write to a formatted output directory.
`, strings.Join(usagegen.SupportedLanguagesUsageSnippets, "\n	- ")),
	RunE: genUsageSnippets,
}

var genSDKVersionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of the SDK generator",
	Long:  `Print the version number of the SDK generator including the latest changelog entry`,
	RunE:  getLatestVersionInfo,
}

var genSDKChangelogCmd = &cobra.Command{
	Use:   "changelog",
	Short: "Prints information about changes to the SDK generator",
	Long:  `Prints information about changes to the SDK generator with the ability to filter by version and format the output for the terminal or parsing. By default it will print the latest changelog entry.`,
	RunE:  getChangelogs,
}

var genSDKDocsCmd = &cobra.Command{
	Use:   "docs",
	Short: "Use this command to generate content for the SDK docs directory.",
	Long:  "Use this command to generate content for the SDK docs directory.",
	RunE:  genSDKDocsContent,
}

var genVersion string

func genInit() {
	rootCmd.AddCommand(generateCmd)

	genVersion = rootCmd.Version

	genSDKInit()
}

//nolint:errcheck
func genSDKInit() {
	genSDKCmd.Flags().StringP("lang", "l", "go", fmt.Sprintf("language to generate sdk for (available options: [%s])", strings.Join(SDKSupportedLanguageTargets(), ", ")))

	genSDKCmd.Flags().StringP("schema", "s", "./openapi.yaml", "local filepath or URL for the OpenAPI schema")
	genSDKCmd.MarkFlagRequired("schema")

	genSDKCmd.Flags().StringP("header", "H", "", "header key to use if authentication is required for downloading schema from remote URL")
	genSDKCmd.Flags().String("token", "", "token value to use if authentication is required for downloading schema from remote URL")

	genSDKCmd.Flags().StringP("out", "o", "", "path to the output directory")
	genSDKCmd.MarkFlagRequired("out")

	genSDKCmd.Flags().BoolP("debug", "d", false, "enable writing debug files with broken code")

	genSDKCmd.Flags().BoolP("auto-yes", "y", false, "auto answer yes to all prompts")

	genSDKCmd.Flags().StringP("installationURL", "i", "", "the language specific installation URL for installation instructions if the SDK is not published to a package manager")
	genSDKCmd.Flags().BoolP("published", "p", false, "whether the SDK is published to a package manager or not, determines the type of installation instructions to generate")

	genSDKCmd.Flags().StringP("repo", "r", "", "the repository URL for the SDK, if the published (-p) flag isn't used this will be used to generate installation instructions")
	genSDKCmd.Flags().StringP("repo-subdir", "b", "", "the subdirectory of the repository where the SDK is located in the repo, helps with documentation generation")

	genSDKCmd.Flags().BoolP("output-tests", "t", false, "output internal tests for internal speakeasy use cases")
	genSDKCmd.Flags().MarkHidden("output-tests")

	genSDKChangelogCmd.Flags().StringP("target", "t", "", "the version(s) to get changelogs from, if not specified the latest version(s) will be used")
	genSDKChangelogCmd.Flags().StringP("previous", "p", "", "the version(s) to get changelogs between this and the target version(s)")
	genSDKChangelogCmd.Flags().StringP("specific", "s", "", "the version to get changelogs for, not used if language is specified")
	genSDKChangelogCmd.Flags().StringP("language", "l", "", "the language to get changelogs for, if not specified the changelog for the generator itself will be returned")
	genSDKChangelogCmd.Flags().BoolP("raw", "r", false, "don't format the output for the terminal")

	genSDKVersionCmd.Flags().StringP("language", "l", "", "if language is set to one of the supported languages it will print version numbers for that languages features and the changelog for that language")

	genUsageSnippetCmd.Flags().StringP("lang", "l", "go", fmt.Sprintf("language to generate sdk for (available options: [%s])", strings.Join(SDKSupportedLanguageTargets(), ", ")))
	genUsageSnippetCmd.Flags().StringP("schema", "s", "./openapi.yaml", "path to the openapi schema")
	genUsageSnippetCmd.MarkFlagRequired("schema")
	genUsageSnippetCmd.Flags().StringP("header", "H", "", "header key to use if authentication is required for downloading schema from remote URL")
	genUsageSnippetCmd.Flags().String("token", "", "token value to use if authentication is required for downloading schema from remote URL")
	genUsageSnippetCmd.Flags().StringP("operation-id", "i", "", "The OperationID to generate usage snippet for")
	genUsageSnippetCmd.Flags().StringP("namespace", "n", "", "The namespace to generate multiple usage snippets for. This could correspond to a tag or a x-speakeasy-group-name in your OpenAPI spec.")
	genUsageSnippetCmd.Flags().StringP("out", "o", "", `By default this command will write to stdout. If a filepath is provided results will be written into that file.
	If the path to an existing directory is provided, all results will be formatted into that directory with each operation getting its own sub folder.`)
	genUsageSnippetCmd.Flags().StringP("config-path", "c", ".", "An optional argument to pass in the path to a directory that holds the gen.yaml configuration file.")

	genSDKDocsCmd.Flags().StringP("out", "o", "", "path to the output directory")
	genSDKDocsCmd.MarkFlagRequired("out")
	genSDKDocsCmd.Flags().StringP("schema", "s", "./openapi.yaml", "local filepath or URL for the OpenAPI schema")
	genSDKDocsCmd.MarkFlagRequired("schema")
	genSDKDocsCmd.Flags().StringP("langs", "l", "", "a list of languages to include in SDK Doc generation. Example usage -l go,python,typescript")
	genSDKDocsCmd.Flags().StringP("header", "H", "", "header key to use if authentication is required for downloading schema from remote URL")
	genSDKDocsCmd.Flags().String("token", "", "token value to use if authentication is required for downloading schema from remote URL")
	genSDKDocsCmd.Flags().BoolP("debug", "d", false, "enable writing debug files with broken code")
	genSDKDocsCmd.Flags().BoolP("auto-yes", "y", false, "auto answer yes to all prompts")
	genSDKDocsCmd.Flags().BoolP("compile", "c", false, "automatically compile SDK docs content for a single page doc site")
	genSDKDocsCmd.Flags().StringP("repo", "r", "", "the repository URL for the SDK Docs repo")
	genSDKDocsCmd.Flags().StringP("repo-subdir", "b", "", "the subdirectory of the repository where the SDK Docs are located in the repo, helps with documentation generation")

	genSDKCmd.AddCommand(genSDKVersionCmd, genSDKChangelogCmd)
	generateCmd.AddCommand(genSDKCmd, genUsageSnippetCmd, genSDKDocsCmd)
}

func genSDKs(cmd *cobra.Command, args []string) error {
	if err := auth.Authenticate(cmd.Context(), false); err != nil {
		return err
	}

	lang, _ := cmd.Flags().GetString("lang")

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

	repo, err := cmd.Flags().GetString("repo")
	if err != nil {
		return err
	}

	repoSubdir, err := cmd.Flags().GetString("repo-subdir")
	if err != nil {
		return err
	}

	outputTests, err := cmd.Flags().GetBool("output-tests")
	if err != nil {
		return err
	}

	if err := sdkgen.Generate(cmd.Context(), config.GetCustomerID(), config.GetWorkspaceID(), lang, schemaPath, header, token, outDir, genVersion, installationURL, debug, autoYes, published, outputTests, repo, repoSubdir, false); err != nil {
		rootCmd.SilenceUsage = true

		return err
	}

	return nil
}

func genUsageSnippets(cmd *cobra.Command, args []string) error {
	lang, _ := cmd.Flags().GetString("lang")

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

	out, _ := cmd.Flags().GetString("out")
	configPath, _ := cmd.Flags().GetString("config-path")
	operation, _ := cmd.Flags().GetString("operation-id")
	namespace, _ := cmd.Flags().GetString("namespace")

	if err := usagegen.Generate(cmd.Context(), config.GetCustomerID(), lang, schemaPath, header, token, out, operation, namespace, configPath); err != nil {
		rootCmd.SilenceUsage = true

		return err
	}

	return nil
}

func getLatestVersionInfo(cmd *cobra.Command, args []string) error {
	version := changelog.GetLatestVersion()
	var changeLog string

	logger := log.From(cmd.Context())

	logger.Printf("Version: %s", version)

	lang, err := cmd.Flags().GetString("language")
	if err != nil {
		lang = ""
	}

	if lang != "" {
		if !slices.Contains(generate.GetSupportedLanguages(), lang) {
			return fmt.Errorf("unsupported language %s", lang)
		}

		latestVersions, err := changelogs.GetLatestVersions(lang)
		if err != nil {
			return fmt.Errorf("failed to get latest versions for language %s: %w", lang, err)
		}

		logger.Printf("Features:")

		for feature, version := range latestVersions {
			logger.Printf("  %s: %s", feature, version)
		}

		if len(latestVersions) > 0 {
			logger.Printf("\n")
		}

		changeLog, err = changelogs.GetChangeLog(lang, latestVersions, nil)
		if err != nil {
			return fmt.Errorf("failed to get changelog for language %s: %w", lang, err)
		}
	} else {
		changeLog = changelog.GetChangeLog(changelog.WithSpecificVersion(version))
	}

	logger.Printf(string(markdown.Render("# CHANGELOG\n\n"+changeLog, 100, 0)))

	return nil
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

	lang, err := cmd.Flags().GetString("language")
	if err != nil {
		return err
	}

	raw, err := cmd.Flags().GetBool("raw")
	if err != nil {
		return err
	}
	raw = raw || !utils.IsInteractive()

	opts := []changelog.Option{}

	var changeLog string

	if lang != "" {
		if !slices.Contains(generate.GetSupportedLanguages(), lang) {
			return fmt.Errorf("unsupported language %s", lang)
		}

		targetVersions := map[string]string{}

		if targetVersion == "" {
			targetVersions, err = changelogs.GetLatestVersions(lang)
			if err != nil {
				return err
			}
		} else {
			pairs := strings.Split(targetVersion, ",")
			for i := 0; i < len(pairs); i += 2 {
				targetVersions[pairs[i]] = pairs[i+1]
			}
		}

		var previousVersions map[string]string

		if previousVersion != "" {
			previousVersions = map[string]string{}

			pairs := strings.Split(previousVersion, ",")
			for i := 0; i < len(pairs); i += 2 {
				previousVersions[pairs[i]] = pairs[i+1]
			}
		}

		changeLog, err = changelogs.GetChangeLog(lang, targetVersions, previousVersions)
		if err != nil {
			return fmt.Errorf("failed to get changelog for language %s: %w", lang, err)
		}
	} else {
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

		changeLog = changelog.GetChangeLog(opts...)
	}

	logger := log.From(cmd.Context())

	if raw {
		logger.Printf(changeLog)
		return nil
	}

	logger.Printf(string(markdown.Render("# CHANGELOG\n\n"+changeLog, 100, 0)))
	return nil
}
