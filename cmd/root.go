package cmd

import (
	"fmt"
	"github.com/hashicorp/go-version"
	"github.com/speakeasy-api/speakeasy/internal/interactivity"
	"github.com/speakeasy-api/speakeasy/internal/styles"
	"github.com/speakeasy-api/speakeasy/internal/updates"
	"github.com/speakeasy-api/speakeasy/internal/utils"
	"os"
	"slices"
	"strings"

	"github.com/speakeasy-api/speakeasy/internal/config"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var rootCmd = &cobra.Command{
	Use:   "speakeasy",
	Short: "The speakeasy cli tool provides access to the speakeasyapi.dev toolchain",
	Long: ` A cli tool for interacting with the Speakeasy https://www.speakeasyapi.dev/ platform and its various functions including:
	- Generating Client SDKs from OpenAPI specs (go, python, typescript, java, php + more coming soon)
	- Validating OpenAPI specs
	- Interacting with the Speakeasy API to create and manage your API workspaces
	- Generating OpenAPI specs from your API traffic 								(coming soon)
	- Generating Postman collections from OpenAPI Specs 							(coming soon)
`,
	RunE: rootExec,
}

var l = log.New().WithLevel(log.LevelInfo)

func init() {
	if err := config.Load(); err != nil {
		l.Error("", zap.Error(err))
		os.Exit(1)
	}
}

func Init(version, artifactArch string) {
	rootCmd.PersistentFlags().String("logLevel", string(log.LevelInfo), fmt.Sprintf("the log level (available options: [%s])", strings.Join(log.Levels, ", ")))

	genInit()
	apiInit()
	validateInit()
	authInit()
	mergeInit()
	updateInit(version, artifactArch)
	suggestInit()
	proxyInit()
	docsInit()
	overlayInit()
	quickstartInit()
	runInit()
}

func Execute(version, artifactArch string) {
	rootCmd.Version = version + "\n" + artifactArch
	rootCmd.SilenceErrors = true
	rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		if cmd.Name() != "update" {
			checkForUpdate(cmd, version, artifactArch)
		}

		if err := setLogLevel(cmd); err != nil {
			return
		}
	}

	Init(version, artifactArch)

	if err := rootCmd.Execute(); err != nil {
		l.Error("", zap.Error(err))
		os.Exit(1)
	}
}

func GetRootCommand() *cobra.Command {
	return rootCmd
}

func checkForUpdate(cmd *cobra.Command, currentVersion, artifactArch string) {
	// Don't display if piping to a file for example
	if !utils.IsInteractive() {
		return
	}

	latestVersion, err := updates.GetLatestVersion(artifactArch)
	if err != nil {
		return
	}

	if latestVersion == nil {
		return
	}

	curVer, err := version.NewVersion(currentVersion)
	if err != nil {
		return
	}

	if latestVersion.GreaterThan(curVer) {
		versionString := fmt.Sprintf("A new version of the Speakeasy CLI is available: v%s", latestVersion.String())
		updateString := "Run `speakeasy update` to update to the latest version"

		l := log.From(cmd.Context())
		style := styles.Emphasized.Copy().Background(styles.Colors.DimYellow).Foreground(styles.Colors.Brown).Padding(1, 2)
		l.PrintfStyled(style, "%s\n%s", versionString, updateString)
		l.Println("\n")

		return
	}
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
	l.WithStyle(styles.HeavilyEmphasized).Println("Welcome to the Speakeasy CLI!")
	l.WithStyle(styles.DimmedItalic).Println("This is interactive mode. For usage, run speakeasy -h instead.")

	return interactivity.InteractiveExec(cmd, args, "Select a command to run")
}
