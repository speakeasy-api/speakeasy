package cmd

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/go-github/v52/github"
	"github.com/manifoldco/promptui"
	"github.com/speakeasy-api/speakeasy/internal/config"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/utils"
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
	RunE:              rootExec,
	PersistentPreRunE: utils.GetMissingFlagsPreRun,
}

var l = log.NewLogger("")

func init() {
	if err := config.Load(); err != nil {
		l.Error("", zap.Error(err))
		os.Exit(1)
	}
}

func Init() {
	genInit()
	apiInit()
	validateInit()
	authInit()
	usageInit()
	mergeInit()
}

func Execute(version, artifactArch string) {
	rootCmd.Version = version + "\n" + artifactArch
	rootCmd.SilenceErrors = true

	Init()

	checkForUpdate(version, artifactArch)

	if err := rootCmd.Execute(); err != nil {
		l.Error("", zap.Error(err))
		os.Exit(1)
	}
}

func GetRootCommand() *cobra.Command {
	return rootCmd
}

func checkForUpdate(version, artifactArch string) {
	client := github.NewClient(&http.Client{
		Timeout: 1 * time.Second,
	})

	releases, _, err := client.Repositories.ListReleases(context.Background(), "speakeasy-api", "speakeasy", nil)
	if err != nil {
		return
	}

	if len(releases) == 0 {
		return
	}

	for _, release := range releases {
		for _, asset := range release.Assets {
			if strings.HasSuffix(strings.ToLower(asset.GetName()), strings.ToLower(artifactArch)+".tar.gz") {
				versionString := fmt.Sprintf(" A new version of the Speakeasy CLI is available: %s ", release.GetTagName())

				fmt.Println(utils.BackgroundYellow(strings.Repeat(" ", len(versionString))))
				fmt.Println(utils.BackgroundYellowBoldFG(versionString))
				fmt.Println(utils.BackgroundYellow(strings.Repeat(" ", len(versionString))))
				fmt.Println()
				return
			}
		}
	}
}

func rootExec(cmd *cobra.Command, args []string) error {
	if !utils.IsInteractive() {
		return cmd.Help()
	}

	welcomeString := promptui.Styler(promptui.FGYellow, promptui.FGBold)("Welcome to the Speakeasy CLI!")
	helpString := promptui.Styler(promptui.FGFaint, promptui.FGItalic)("This is interactive mode. For usage, run speakeasy -h instead")
	println(fmt.Sprintf("%s\n%s\n", welcomeString, helpString))

	return utils.InteractiveExec(cmd, args, "What do you want to do?")
}
