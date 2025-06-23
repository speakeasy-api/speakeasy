package generate

import (
	"context"
	"fmt"
	"slices"
	"strings"

	charm_internal "github.com/speakeasy-api/speakeasy/internal/charm"
	"github.com/speakeasy-api/speakeasy/internal/model/flag"

	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/model"
	"github.com/speakeasy-api/speakeasy/internal/utils"

	markdown "github.com/MichaelMure/go-term-markdown"
	changelog "github.com/speakeasy-api/openapi-generation/v2"
	"github.com/speakeasy-api/openapi-generation/v2/changelogs"
	"github.com/speakeasy-api/openapi-generation/v2/pkg/generate"
)

func GeneratorSupportedTargetNames() []string {
	return generate.GetSupportedTargetNames()
}

var (
	headerFlag = flag.StringFlag{
		Name:        "header",
		Shorthand:   "H",
		Description: "header key to use if authentication is required for downloading schema from remote URL",
	}
	tokenFlag = flag.StringFlag{
		Name:        "token",
		Description: "token value to use if authentication is required for downloading schema from remote URL",
	}
	schemaFlag = flag.StringFlag{
		Name:                       "schema",
		Shorthand:                  "s",
		Description:                "local filepath or URL for the OpenAPI schema",
		Required:                   true,
		DefaultValue:               "./openapi.yaml",
		AutocompleteFileExtensions: charm_internal.OpenAPIFileExtensions,
	}
	outFlag = flag.StringFlag{
		Name:        "out",
		Shorthand:   "o",
		Description: "path to the output directory",
		Required:    true,
	}
	debugFlag = flag.BooleanFlag{
		Name:        "debug",
		Shorthand:   "d",
		Description: "enable writing debug files with broken code",
	}
	autoYesFlag = flag.BooleanFlag{
		Name:        "auto-yes",
		Shorthand:   "y",
		Description: "auto answer yes to all prompts",
	}
	repoFlag = flag.StringFlag{
		Name:        "repo",
		Shorthand:   "r",
		Description: "the repository URL for the SDK, if the published (-p) flag isn't used this will be used to generate installation instructions",
	}
	repoSubdirFlag = flag.StringFlag{
		Name:        "repo-subdir",
		Shorthand:   "b",
		Description: "the subdirectory of the repository where the SDK is located in the repo, helps with documentation generation",
	}
)

var GenerateCmd = &model.CommandGroup{
	Usage:          "generate",
	Short:          "One off Generations for client SDKs and more",
	Long:           `The "generate" command provides a set of commands for one off generations of client SDKs and Terraform providers`,
	InteractiveMsg: "What do you want to generate?",
	Commands:       []model.Command{genSDKCmd, genUsageSnippetCmd, codeSamplesCmd, genSDKVersionCmd, genSDKChangelogCmd, suportedTargetsCmd},
	Hidden:         true,
}

type GenerateSDKVersionFlags struct {
	Language string `json:"language"`
}

var genSDKVersionCmd = &model.ExecutableCommand[GenerateSDKVersionFlags]{
	Usage: "version",
	Short: "Print the version number of the SDK generator",
	Long:  `Print the version number of the SDK generator including the latest changelog entry`,
	Run:   getLatestVersionInfo,
	Flags: []flag.Flag{
		flag.StringFlag{
			Name:        "language",
			Shorthand:   "l",
			Description: "if language is set to one of the supported languages it will print version numbers for that languages features and the changelog for that language",
		},
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
	Flags: []flag.Flag{
		flag.StringFlag{
			Name:        "target",
			Shorthand:   "t",
			Description: "the version(s) to get changelogs from, if not specified the latest version(s) will be used",
		},
		flag.StringFlag{
			Name:        "previous",
			Shorthand:   "p",
			Description: "the version(s) to get changelogs between this and the target version(s)",
		},
		flag.StringFlag{
			Name:        "specific",
			Shorthand:   "s",
			Description: "the version to get changelogs for, not used if language is specified",
		},
		flag.StringFlag{
			Name:        "language",
			Shorthand:   "l",
			Description: "the language to get changelogs for, if not specified the changelog for the generator itself will be returned",
		},
		flag.BooleanFlag{
			Name:        "raw",
			Shorthand:   "r",
			Description: "don't format the output for the terminal",
		},
	},
}

func getLatestVersionInfo(ctx context.Context, flags GenerateSDKVersionFlags) error {
	version := changelog.GetLatestVersion()
	var changeLog string

	logger := log.From(ctx)

	logger.Printf("Version: %s", version)

	lang := flags.Language
	if lang != "" {
		if !slices.Contains(generate.GetSupportedTargetNames(), lang) {
			return fmt.Errorf("unsupported language %s", lang)
		}

		latestVersions, err := changelogs.GetLatestVersions(lang)
		if err != nil {
			return fmt.Errorf("failed to get latest versions for language %s: %w", lang, err)
		}

		logger.Print("Features:")

		for feature, version := range latestVersions {
			logger.Printf("  %s: %s", feature, version)
		}

		if len(latestVersions) > 0 {
			logger.Print("\n")
		}

		changeLog, err = changelogs.GetChangeLog(lang, latestVersions, nil)
		if err != nil {
			return fmt.Errorf("failed to get changelog for language %s: %w", lang, err)
		}
	} else {
		changeLog = changelog.GetChangeLog(changelog.WithSpecificVersion(version))
	}

	logger.Print(string(markdown.Render("# CHANGELOG...\n\n"+changeLog, 100, 0)))

	return nil
}

func getChangelogs(ctx context.Context, flags GenerateSDKChangelogFlags) error {
	raw := flags.Raw || !utils.IsInteractive()

	opts := []changelog.Option{}

	var err error
	var changeLog string

	lang := flags.Language
	if lang != "" {
		if !slices.Contains(generate.GetSupportedTargetNames(), lang) {
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
		logger.Print(changeLog)
		return nil
	}

	logger.Print(string(markdown.Render("# CHANGELOG&&&\n\n"+changeLog, 100, 0)))
	return nil
}
