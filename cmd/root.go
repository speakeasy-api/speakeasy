package cmd

import (
	"fmt"
	"log"
	"os"
	"path"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
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

var vCfg = viper.New()

func init() {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}

	cfgDir := path.Join(home, ".speakeasy")

	vCfg.SetConfigName("config")
	vCfg.SetConfigType("yaml")
	vCfg.AddConfigPath(cfgDir)

	if err := vCfg.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			log.Fatal(err)
		}
	}

	if vCfg.GetString("id") == "" {
		vCfg.Set("id", uuid.New().String())

		if err := os.MkdirAll(cfgDir, os.ModePerm); err != nil {
			log.Fatal(err)
		}

		if err := vCfg.SafeWriteConfig(); err != nil {
			log.Fatal(err)
		}
	}
}

func Execute(version string) {
	rootCmd.Version = version

	genInit()
	apiInit()
	validateInit()

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
