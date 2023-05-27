package cmd

import (
	"fmt"
	"math"
	"os"
	"strings"

	"github.com/hashicorp/go-version"
	"github.com/manifoldco/promptui"
	"github.com/speakeasy-api/speakeasy/internal/config"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/updates"
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
	RunE: rootExec,
}

var l = log.NewLogger("")

func init() {
	if err := config.Load(); err != nil {
		l.Error("", zap.Error(err))
		os.Exit(1)
	}
}

func Init(version, artifactArch string) {
	genInit()
	apiInit()
	validateInit()
	authInit()
	usageInit()
	mergeInit()
	updateInit(version, artifactArch)
	suggestInit()
}

func Execute(version, artifactArch string) {
	rootCmd.Version = version + "\n" + artifactArch
	rootCmd.SilenceErrors = true
	rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		if cmd.Name() != "update" {
			checkForUpdate(version, artifactArch)
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

func checkForUpdate(currVersion, artifactArch string) {
	latestVersion, err := updates.GetLatestVersion(artifactArch)
	if err != nil {
		return
	}

	if latestVersion == nil {
		return
	}

	curVer, err := version.NewVersion(currVersion)
	if err != nil {
		return
	}

	if latestVersion.GreaterThan(curVer) {
		versionString := fmt.Sprintf(" A new version of the Speakeasy CLI is available: v%s ", latestVersion.String())
		updateString := " Run `speakeasy update` to update to the latest version "
		padLength := int(math.Max(float64(len(versionString)), float64(len(updateString))))

		fmt.Println(utils.BackgroundYellow(strings.Repeat(" ", padLength)))
		fmt.Println(utils.BackgroundYellowBoldFG(padRight(versionString, padLength)))
		fmt.Println(utils.BackgroundYellowBoldFG(padRight(updateString, padLength)))
		fmt.Println(utils.BackgroundYellow(strings.Repeat(" ", padLength)))
		fmt.Println()
		return
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

func padRight(str string, width int) string {
	spaces := width - len(str)
	return str + strings.Repeat(" ", spaces)
}
