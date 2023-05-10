package cmd

import (
	"fmt"
	"github.com/manifoldco/promptui"
	"github.com/speakeasy-api/speakeasy/internal/config"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/utils"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"os"
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

func Execute(version string) {
	rootCmd.Version = version
	rootCmd.SilenceErrors = true

	Init()

	if err := rootCmd.Execute(); err != nil {
		l.Error("", zap.Error(err))
		os.Exit(1)
	}
}

func GetRootCommand() *cobra.Command {
	return rootCmd
}

func rootExec(cmd *cobra.Command, args []string) error {
	welcomeString := promptui.Styler(promptui.FGYellow, promptui.FGBold)("Welcome to the Speakeasy CLI!")
	helpString := promptui.Styler(promptui.FGFaint, promptui.FGItalic)("This is interactive mode. For usage, run speakeasy -h instead")
	println(fmt.Sprintf("%s\n%s\n", welcomeString, helpString))

	return utils.InteractiveExec(cmd, args, "What do you want to do?")
}
