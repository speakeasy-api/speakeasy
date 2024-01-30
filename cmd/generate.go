package cmd

import (
	"context"
	"fmt"
	"github.com/speakeasy-api/speakeasy/internal/docsgen"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/model"
	"github.com/speakeasy-api/speakeasy/internal/utils"
	"strings"

	"github.com/speakeasy-api/speakeasy/internal/usagegen"
	"golang.org/x/exp/slices"

	markdown "github.com/MichaelMure/go-term-markdown"
	changelog "github.com/speakeasy-api/openapi-generation/v2"
	"github.com/speakeasy-api/openapi-generation/v2/changelogs"
	"github.com/speakeasy-api/openapi-generation/v2/pkg/generate"
	"github.com/speakeasy-api/speakeasy/internal/config"
	"github.com/speakeasy-api/speakeasy/internal/sdkgen"
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

var (
	headerFlag = model.StringFlag{
		Name:        "header",
		Shorthand:   "H",
		Description: "header key to use if authentication is required for downloading schema from remote URL",
	}
	tokenFlag = model.StringFlag{
		Name:        "token",
		Description: "token value to use if authentication is required for downloading schema from remote URL",
	}
	schemaFlag = model.StringFlag{
		Name:         "schema",
		Shorthand:    "s",
		Description:  "local filepath or URL for the OpenAPI schema",
		Required:     true,
		DefaultValue: "./openapi.yaml",
	}
	outFlag = model.StringFlag{
		Name:        "out",
		Shorthand:   "o",
		Description: "path to the output directory",
		Required:    true,
	}
	debugFlag = model.BooleanFlag{
		Name:        "debug",
		Shorthand:   "d",
		Description: "enable writing debug files with broken code",
	}
	autoYesFlag = model.BooleanFlag{
		Name:        "auto-yes",
		Shorthand:   "y",
		Description: "auto answer yes to all prompts",
	}
	repoFlag = model.StringFlag{
		Name:        "repo",
		Shorthand:   "r",
		Description: "the repository URL for the SDK, if the published (-p) flag isn't used this will be used to generate installation instructions",
	}
	repoSubdirFlag = model.StringFlag{
		Name:        "repo-subdir",
		Shorthand:   "b",
		Description: "the subdirectory of the repository where the SDK is located in the repo, helps with documentation generation",
	}
)

var generateCmd = &model.CommandGroup{
	Usage:          "generate",
	Short:          "Generate client SDKs, docsites, and more",
	Long:           `The "generate" command provides a set of commands for generating client SDKs, OpenAPI specs (coming soon) and more (coming soon).`,
	InteractiveMsg: "What do you want to generate?",
	Commands:       []model.Command{genSDKCmd, genSDKDocsCmd, genUsageSnippetCmd, genSDKVersionCmd, genSDKChangelogCmd},
}

type GenerateFlags struct {
	Lang            string `json:"lang"`
	SchemaPath      string `json:"schema"`
	OutDir          string `json:"out"`
	Header          string `json:"header"`
	Token           string `json:"token"`
	Debug           bool   `json:"debug"`
	AutoYes         bool   `json:"auto-yes"`
	InstallationURL string `json:"installationURL"`
	Published       bool   `json:"published"`
	Repo            string `json:"repo"`
	RepoSubdir      string `json:"repo-subdir"`
	OutputTests     bool   `json:"output-tests"`
}

var genSDKCmd = &model.ExecutableCommand[GenerateFlags]{
	Usage:        "sdk",
	Short:        fmt.Sprintf("Generating Client SDKs from OpenAPI specs (%s + more coming soon)", strings.Join(SDKSupportedLanguageTargets(), ", ")),
	Long:         generateLongDesc,
	Run:          genSDKs,
	RequiresAuth: true,
	Flags: []model.Flag{
		model.StringFlag{
			Name:        "lang",
			Shorthand:   "l",
			Description: fmt.Sprintf("language to generate sdk for (available options: [%s])", strings.Join(SDKSupportedLanguageTargets(), ", ")),
		},
		schemaFlag,
		outFlag,
		headerFlag,
		tokenFlag,
		debugFlag,
		autoYesFlag,
		model.StringFlag{
			Name:        "installationURL",
			Shorthand:   "i",
			Description: "the language specific installation URL for installation instructions if the SDK is not published to a package manager",
		},
		model.BooleanFlag{
			Name:        "published",
			Shorthand:   "p",
			Description: "whether the SDK is published to a package manager or not, determines the type of installation instructions to generate",
		},
		repoFlag,
		repoSubdirFlag,
		model.BooleanFlag{
			Name:        "output-tests",
			Shorthand:   "t",
			Description: "output internal tests for internal speakeasy use cases",
			Hidden:      true,
		},
	},
}

type GenerateUsageSnippetFlags struct {
	Lang       string `json:"lang"`
	SchemaPath string `json:"schema"`
	Header     string `json:"header"`
	Token      string `json:"token"`
	Operation  string `json:"operation-id"`
	Namespace  string `json:"namespace"`
	Out        string `json:"out"`
	ConfigPath string `json:"config-path"`
}

var genUsageSnippetCmd = &model.ExecutableCommand[GenerateUsageSnippetFlags]{
	Usage: "usage",
	Short: fmt.Sprintf("Generate standalone usage snippets for SDKs in (%s)", strings.Join(usagegen.SupportedLanguagesUsageSnippets, ", ")),
	Long: fmt.Sprintf(`Using the "speakeasy generate usage" command you can generate usage snippets for various SDKs.

The following languages are currently supported:
	- %s
	- more coming soon

You can generate usage snippets by OperationID or by Namespace. By default this command will write to stdout.

You can also select to write to a file or write to a formatted output directory.
`, strings.Join(usagegen.SupportedLanguagesUsageSnippets, "\n	- ")),
	Run: genUsageSnippets,
	Flags: []model.Flag{
		model.StringFlag{
			Name:         "lang",
			Shorthand:    "l",
			Description:  fmt.Sprintf("language to generate sdk for (available options: [%s])", strings.Join(usagegen.SupportedLanguagesUsageSnippets, ", ")),
			DefaultValue: "go",
		},
		schemaFlag,
		headerFlag,
		tokenFlag,
		model.StringFlag{
			Name:        "operation-id",
			Shorthand:   "i",
			Description: "The OperationID to generate usage snippet for",
		},
		model.StringFlag{
			Name:        "namespace",
			Shorthand:   "n",
			Description: "The namespace to generate multiple usage snippets for. This could correspond to a tag or a x-speakeasy-group-name in your OpenAPI spec.",
		},
		model.StringFlag{
			Name:      "out",
			Shorthand: "o",
			Description: `By default this command will write to stdout. If a filepath is provided results will be written into that file.
	If the path to an existing directory is provided, all results will be formatted into that directory with each operation getting its own sub folder.`,
		},
		model.StringFlag{
			Name:         "config-path",
			Shorthand:    "c",
			DefaultValue: ".",
			Description:  "An optional argument to pass in the path to a directory that holds the gen.yaml configuration file.",
		},
	},
}

type GenerateSDKVersionFlags struct {
	Language string `json:"language"`
}

var genSDKVersionCmd = &model.ExecutableCommand[GenerateSDKVersionFlags]{
	Usage: "version",
	Short: "Print the version number of the SDK generator",
	Long:  `Print the version number of the SDK generator including the latest changelog entry`,
	Run:   getLatestVersionInfo,
	Flags: []model.Flag{
		model.StringFlag{
			Name:        "language",
			Shorthand:   "l",
			Description: "if language is set to one of the supported languages it will print version numbers for that languages features and the changelog for that language",
		},
	},
}

type GenerateSDKDocsFlags struct {
	Langs      string `json:"langs"`
	SchemaPath string `json:"schema"`
	OutDir     string `json:"out"`
	Header     string `json:"header"`
	Token      string `json:"token"`
	Debug      bool   `json:"debug"`
	AutoYes    bool   `json:"auto-yes"`
	Compile    bool   `json:"compile"`
	Repo       string `json:"repo"`
	RepoSubdir string `json:"repo-subdir"`
}

var genSDKDocsCmd = &model.ExecutableCommand[GenerateSDKDocsFlags]{
	Usage: "docs",
	Short: "Use this command to generate content for the SDK docs directory.",
	Long:  "Use this command to generate content for the SDK docs directory.",
	Run:   genSDKDocsContent,
	Flags: []model.Flag{
		outFlag,
		schemaFlag,
		model.StringFlag{
			Name:        "langs",
			Shorthand:   "l",
			Description: "a list of languages to include in SDK Docs generation. Example usage -l go,python,typescript",
		},
		headerFlag,
		tokenFlag,
		debugFlag,
		autoYesFlag,
		model.BooleanFlag{
			Name:        "compile",
			Shorthand:   "c",
			Description: "automatically compile SDK docs content for a single page doc site",
		},
		repoFlag,
		repoSubdirFlag,
	},
}

type GenerateSDKChangelogFlags struct {
	TargetVersion   string `json:"target"`
	PreviousVersion string `json:"previous"`
	SpecificVersion string `json:"specific"`
	Language        string `json:"language"`
	Raw             bool   `json:"raw"`
}

var genSDKChangelogCmd = &model.ExecutableCommand[GenerateSDKChangelogFlags]{
	Usage: "changelog",
	Short: "Prints information about changes to the SDK generator",
	Long:  `Prints information about changes to the SDK generator with the ability to filter by version and format the output for the terminal or parsing. By default it will print the latest changelog entry.`,
	Run:   getChangelogs,
	Flags: []model.Flag{
		model.StringFlag{
			Name:        "target",
			Shorthand:   "t",
			Description: "the version(s) to get changelogs from, if not specified the latest version(s) will be used",
		},
		model.StringFlag{
			Name:        "previous",
			Shorthand:   "p",
			Description: "the version(s) to get changelogs between this and the target version(s)",
		},
		model.StringFlag{
			Name:        "specific",
			Shorthand:   "s",
			Description: "the version to get changelogs for, not used if language is specified",
		},
		model.StringFlag{
			Name:        "language",
			Shorthand:   "l",
			Description: "the language to get changelogs for, if not specified the changelog for the generator itself will be returned",
		},
		model.BooleanFlag{
			Name:        "raw",
			Shorthand:   "r",
			Description: "don't format the output for the terminal",
		},
	},
}

var genVersion string

func genSDKs(ctx context.Context, flags GenerateFlags) error {
	if err := sdkgen.Generate(
		ctx,
		config.GetCustomerID(),
		config.GetWorkspaceID(),
		flags.Lang,
		flags.SchemaPath,
		flags.Header,
		flags.Token,
		flags.OutDir,
		genVersion,
		flags.InstallationURL,
		flags.Debug,
		flags.AutoYes,
		flags.Published,
		flags.OutputTests,
		flags.Repo,
		flags.RepoSubdir,
		false,
	); err != nil {
		rootCmd.SilenceUsage = true

		return err
	}

	return nil
}

func genUsageSnippets(ctx context.Context, flags GenerateUsageSnippetFlags) error {
	if err := usagegen.Generate(
		ctx,
		config.GetCustomerID(),
		flags.Lang,
		flags.SchemaPath,
		flags.Header,
		flags.Token,
		flags.Out,
		flags.Operation,
		flags.Namespace,
		flags.ConfigPath,
	); err != nil {
		rootCmd.SilenceUsage = true

		return err
	}

	return nil
}

func genSDKDocsContent(ctx context.Context, flags GenerateSDKDocsFlags) error {
	languages := make([]string, 0)
	if flags.Langs != "" {
		for _, lang := range strings.Split(flags.Langs, ",") {
			languages = append(languages, strings.TrimSpace(lang))
		}
	}

	if err := docsgen.GenerateContent(
		ctx,
		languages,
		config.GetCustomerID(),
		flags.SchemaPath,
		flags.Header,
		flags.Token,
		flags.OutDir,
		flags.Repo,
		flags.RepoSubdir,
		flags.Debug,
		flags.AutoYes,
		flags.Compile,
	); err != nil {
		rootCmd.SilenceUsage = true

		return err
	}

	return nil
}

func getLatestVersionInfo(ctx context.Context, flags GenerateSDKVersionFlags) error {
	version := changelog.GetLatestVersion()
	var changeLog string

	logger := log.From(ctx)

	logger.Printf("Version: %s", version)

	lang := flags.Language
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

func getChangelogs(ctx context.Context, flags GenerateSDKChangelogFlags) error {
	raw := flags.Raw || !utils.IsInteractive()

	opts := []changelog.Option{}

	var err error
	var changeLog string

	lang := flags.Language
	if lang != "" {
		if !slices.Contains(generate.GetSupportedLanguages(), lang) {
			return fmt.Errorf("unsupported language %s", lang)
		}

		targetVersions := map[string]string{}

		if flags.TargetVersion == "" {
			targetVersions, err = changelogs.GetLatestVersions(lang)
			if err != nil {
				return err
			}
		} else {
			pairs := strings.Split(flags.TargetVersion, ",")
			for i := 0; i < len(pairs); i += 2 {
				targetVersions[pairs[i]] = pairs[i+1]
			}
		}

		var previousVersions map[string]string

		if flags.PreviousVersion != "" {
			previousVersions = map[string]string{}

			pairs := strings.Split(flags.PreviousVersion, ",")
			for i := 0; i < len(pairs); i += 2 {
				previousVersions[pairs[i]] = pairs[i+1]
			}
		}

		changeLog, err = changelogs.GetChangeLog(lang, targetVersions, previousVersions)
		if err != nil {
			return fmt.Errorf("failed to get changelog for language %s: %w", lang, err)
		}
	} else {
		if flags.TargetVersion != "" {
			opts = append(opts, changelog.WithTargetVersion(flags.TargetVersion))

			if flags.PreviousVersion != "" {
				opts = append(opts, changelog.WithPreviousVersion(flags.PreviousVersion))
			}
		} else if flags.SpecificVersion != "" {
			opts = append(opts, changelog.WithSpecificVersion(flags.SpecificVersion))
		} else {
			opts = append(opts, changelog.WithSpecificVersion(changelog.GetLatestVersion()))
		}

		changeLog = changelog.GetChangeLog(opts...)
	}

	logger := log.From(ctx)

	if raw {
		logger.Printf(changeLog)
		return nil
	}

	logger.Printf(string(markdown.Render("# CHANGELOG\n\n"+changeLog, 100, 0)))
	return nil
}

var generateLongDesc = fmt.Sprintf(`Using the "speakeasy generate sdk" command you can generate client SDK packages for various languages
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
`, strings.Join(SDKSupportedLanguageTargets(), "\n	- "))
