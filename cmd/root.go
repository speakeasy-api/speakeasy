package cmd

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/speakeasy-api/speakeasy/cmd/lint"

	"github.com/speakeasy-api/speakeasy/cmd/generate"
	"github.com/speakeasy-api/speakeasy/cmd/openapi"

	"github.com/speakeasy-api/speakeasy-core/events"

	"github.com/speakeasy-api/speakeasy/internal/env"
	"github.com/speakeasy-api/speakeasy/internal/model"

	"github.com/speakeasy-api/speakeasy/internal/charm/styles"
	"github.com/speakeasy-api/speakeasy/internal/interactivity"
	"github.com/speakeasy-api/speakeasy/internal/updates"
	"github.com/speakeasy-api/speakeasy/internal/utils"

	"github.com/speakeasy-api/speakeasy/internal/config"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

const rootLong = `# Speakeasy 

A CLI tool for interacting with the [Speakeasy platform](https://www.speakeasy.com/) and its APIs.

Use this CLI to:
- Lint and validate OpenAPI specs
- Create, manage, and run Speakeasy workflows
- Configure GitHub Actions for Speakeasy workflows
- Suggest improvements to OpenAPI specs

Generate from OpenAPI Specs:
- Client and Server SDKs in GO, Python, TypeScript, Java, PHP, C#, Ruby
- Postman collections
- Terraform providers
- MCP Servers

[Quickstart guide](https://www.speakeasy.com/docs/create-client-sdks)

Visit [Speakeasy](https://www.speakeasy.com/) for more information
`

var rootCmd = &cobra.Command{
	Use:   "speakeasy",
	Short: "The Speakeasy CLI tool provides access to the Speakeasy.com platform",
	Long:  utils.RenderMarkdown(rootLong),
	RunE:  rootExec,
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
	addCommand(rootCmd, statusCmd)
	addCommand(rootCmd, quickstartCmd)
	addCommand(rootCmd, billingCmd)
	addCommand(rootCmd, runCmd)
	addCommand(rootCmd, configureCmd)
	addCommand(rootCmd, generate.GenerateCmd)
	addCommand(rootCmd, lint.LintCmd)
	addCommand(rootCmd, openapi.OpenAPICmd)
	addCommand(rootCmd, migrateCmd)

	authInit()
	mergeInit()
	addCommand(rootCmd, overlayCmd)
	addCommand(rootCmd, suggestCmd)
	addCommand(rootCmd, testCmd)
	addCommand(rootCmd, defaultCodeSamplesCmd)
	updateInit(version, artifactArch)
	proxyInit()
	languageServerInit(version)
	bumpInit()
	addCommand(rootCmd, tagCmd)
	addCommand(rootCmd, cleanCmd)

	addCommand(rootCmd, AskCmd)
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
			checkForUpdate(ctx, version, artifactArch, cmd)
		}

		return setLogLevel(cmd)
	}

	Init(version, artifactArch)
}

func GetRootCommand() *cobra.Command {
	return rootCmd
}

func checkForUpdate(ctx context.Context, currentVersion, artifactArch string, cmd *cobra.Command) {
	// Don't display if piping to a file for example
	if !utils.IsInteractive() {
		return
	}

	if env.IsLocalDev() {
		return
	}

	// When using the --pinned flag, don't display update notifications
	if flag := cmd.Flag("pinned"); flag != nil && flag.Value.String() == "true" {
		return
	}

	newerVersion, err := updates.GetNewerVersion(ctx, artifactArch, currentVersion)
	if err != nil {
		return // Don't display error to user
	}

	if newerVersion == nil {
		return
	}

	mainStyle := styles.Emphasized.Background(styles.Colors.SpeakeasyPrimary).Foreground(styles.Colors.SpeakeasySecondary)
	inverseStyle := styles.Emphasized.Background(styles.Colors.SpeakeasySecondary).Foreground(styles.Colors.SpeakeasyPrimary)

	versionString := fmt.Sprintf("A new version of the Speakeasy CLI is available: v%s", newerVersion.String())
	updateString := fmt.Sprintf("Run %s%s", inverseStyle.Render("speakeasy update"), mainStyle.Render(" to update to the latest version"))

	l := log.From(ctx)
	l.PrintfStyled(mainStyle.Padding(1, 2), "%s\n%s", versionString, updateString)
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
