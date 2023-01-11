package cmd

import (
	"fmt"
	"os"

	"github.com/speakeasy-api/speakeasy/internal/config"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "speakeasy",
	Short: "The speakeasy cli tool provides access to the speakeasyapi.dev toolchain",
	Long: ` A cli tool for interacting with the Speakeasy https://www.speakeasyapi.dev/ platform and its various functions including:
	- Generating Client SDKs from OpenAPI specs (go, python, typescript(web/server), + more coming soon)
	- Interacting with the Speakeasy API to create and manage your API workspaces	(coming soon)
	- Generating OpenAPI specs from your API traffic 								(coming soon)
	- Validating OpenAPI specs 														(coming soon)
	- Generating Postman collections from OpenAPI Specs 							(coming soon)
`,
	RunE: rootExec,
}

func init() {
	if err := config.Load(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func Init() {
	genInit()
	apiInit()
	validateInit()
	authInit()
}

func Execute(version string) {
	rootCmd.Version = version

	Init()

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func GetRootCommand() *cobra.Command {
	return rootCmd
}

func rootExec(cmd *cobra.Command, args []string) error {
	return cmd.Help()
}
