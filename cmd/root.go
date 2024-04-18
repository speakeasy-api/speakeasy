package cmd

import (
	"context"
	"fmt"
	"github.com/speakeasy-api/speakeasy/internal/env"
	"os"
	"slices"
	"strings"

	"github.com/speakeasy-api/speakeasy/cmd/generate"

	"github.com/speakeasy-api/speakeasy-core/events"

	"github.com/speakeasy-api/speakeasy/internal/model"

	"github.com/speakeasy-api/speakeasy/internal/charm/styles"
	"github.com/speakeasy-api/speakeasy/internal/interactivity"
	"github.com/speakeasy-api/speakeasy/internal/updates"
	"github.com/speakeasy-api/speakeasy/internal/utils"

	_ "embed"

	"github.com/speakeasy-api/speakeasy/internal/config"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

//go:embed root_examples.md
var examples string

const rootLong = `# Speakeasy CLI

A CLI tool for interacting with the [Speakeasy platform](https://www.speakeasyapi.dev/) and its various functions including:

- Generating Client SDKs from OpenAPI specs
	(go, python, typescript, java, php, c#, swift, ruby, terraform)
- Validating OpenAPI specs
- Interacting with the Speakeasy API to create and manage your API workspaces
- Generating OpenAPI specs from your API traffic
- Generating Postman collections from OpenAPI Specs
`

var rootCmd = &cobra.Command{
	Use:     "speakeasy",
	Short:   log.RenderMarkdown("The Speakeasy CLI provides access to the [Speakeasy](speakeasyapi.dev) toolchain"),
	Long:    log.RenderMarkdown(rootLong),
	Example: log.RenderMarkdown(examples),
	RunE:    rootExec,
}

var l = log.New().WithLevel(log.LevelInfo)

func init() {
	// We want our commands to be sorted in defined order, not alphabetically
	cobra.EnableCommandSorting = false
	if err := config.Load(); err != nil {
		l.Error("", zap.Error(err))
		os.Exit(1)
	}
}

func Init(version, artifactArch string) {
	rootCmd.PersistentFlags().String("logLevel", string(log.LevelInfo), fmt.Sprintf("the log level (available options: [%s])", strings.Join(log.Levels, ", ")))

	// TODO: migrate this file to use model.CommandGroup once all subcommands have been refactored
	addCommand(rootCmd, quickstartCmd)
	addCommand(rootCmd, runCmd)
	addCommand(rootCmd, configureCmd)
	addCommand(rootCmd, generate.GenerateCmd)
	addCommand(rootCmd, lintCmd)
	addCommand(rootCmd, openapiCmd)
	addCommand(rootCmd, migrateCmd)

	authInit()
	mergeInit()
	addCommand(rootCmd, overlayCmd)
	addCommand(rootCmd, transformCmd)
	suggestInit()
	updateInit(version, artifactArch)
	proxyInit()
	apiInit()
	languageServerInit(version)
	bumpInit()
}

func addCommand(cmd *cobra.Command, command model.Command) {
	c, err := command.Init()
	if err != nil {
		l.Error("", zap.Error(err))
		os.Exit(1)
	}
	cmd.AddCommand(c)
}

func CmdForTest(version, artifactArch string) *cobra.Command {
	setupRootCmd(version, artifactArch)

	return rootCmd
}

func Execute(version, artifactArch string) {
	setupRootCmd(version, artifactArch)

	if err := rootCmd.Execute(); err != nil {
		l.Error("", zap.Error(err))
		os.Exit(1)
	}
}

func setupRootCmd(version, artifactArch string) {
	rootCmd.Version = version + "\n" + artifactArch
	rootCmd.SilenceErrors = true
	rootCmd.SilenceUsage = true
	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		ctx := context.WithValue(cmd.Context(), updates.ArtifactArchContextKey, artifactArch)
		ctx = events.SetSpeakeasyVersionInContext(ctx, version)
		cmd.SetContext(ctx)

		if !slices.Contains([]string{"update", "language-server"}, cmd.Name()) {
			checkForUpdate(ctx, version, artifactArch)
		}

		return setLogLevel(cmd)
	}

	Init(version, artifactArch)
}

func GetRootCommand() *cobra.Command {
	return rootCmd
}

func checkForUpdate(ctx context.Context, currentVersion, artifactArch string) {
	// Don't display if piping to a file for example or running locally during development
	if !utils.IsInteractive() || env.IsLocalDev() {
		return
	}

	newerVersion, err := updates.GetNewerVersion(artifactArch, currentVersion)
	if err != nil {
		return // Don't display error to user
	}

	if newerVersion == nil {
		return
	}

	versionString := fmt.Sprintf("A new version of the Speakeasy CLI is available: v%s", newerVersion.String())
	updateString := "Run `speakeasy update` to update to the latest version"

	l := log.From(ctx)
	style := styles.Emphasized.Copy().Background(styles.Colors.SpeakeasyPrimary).Foreground(styles.Colors.SpeakeasySecondary).Padding(1, 2)
	l.PrintfStyled(style, "%s\n%s", versionString, updateString)
	l.Println("\n")

	return
}

func setLogLevel(cmd *cobra.Command) error {
	logLevel, err := cmd.Flags().GetString("logLevel")
	if err != nil {
		return err
	}
	if !slices.Contains(log.Levels, logLevel) {
		return fmt.Errorf("log level must be one of: %s", strings.Join(log.Levels, ", "))
	}

	l = l.WithLevel(log.Level(logLevel))
	ctx := log.With(cmd.Context(), l)
	cmd.SetContext(ctx)

	return nil
}

func rootExec(cmd *cobra.Command, args []string) error {
	if !utils.IsInteractive() {
		return cmd.Help()
	}

	l := log.From(cmd.Context()).WithInteractiveOnly()
	l.PrintfStyled(styles.HeavilyEmphasized, "Welcome to the Speakeasy CLI!\n")
	l.PrintfStyled(styles.DimmedItalic, "This is interactive mode. For usage, run speakeasy -h instead.\n")

	return interactivity.InteractiveExec(cmd, args, "Select a command to run")
}
