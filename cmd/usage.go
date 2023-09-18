package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/speakeasy-api/speakeasy/internal/usage"
	"github.com/speakeasy-api/speakeasy/internal/utils"
	"github.com/spf13/cobra"
)

const (
	defaultOutFile = "openapi.csv"
	outFlag        = "out"
)

func usageInit() {
	usageCmd := &cobra.Command{
		Use:   "usage",
		Short: "Output usage information for a given OpenAPI schema to a CSV",
		Long:  `Output usage information containing counts of OpenAPI features for a given OpenAPI schema to a CSV`,
		RunE:  genUsage,
	}
	usageCmd.Flags().StringP("schema", "s", "./openapi.yaml", "local filepath or URL for the OpenAPI schema")
	usageCmd.MarkFlagRequired("schema")

	usageCmd.Flags().StringP("header", "H", "", "header key to use if authentication is required for downloading schema from remote URL")
	usageCmd.Flags().String("token", "", "token value to use if authentication is required for downloading schema from remote URL")

	usageCmd.Flags().StringP("out", "o", "", "Path to output file")
	usageCmd.Flags().BoolP("debug", "d", false, "enable writing debug files with broken code")

	rootCmd.AddCommand(usageCmd)
}

func genUsage(cmd *cobra.Command, args []string) error {
	schemaPath, err := cmd.Flags().GetString("schema")
	if err != nil {
		return err
	}

	header, err := cmd.Flags().GetString("header")
	if err != nil {
		return err
	}

	token, err := cmd.Flags().GetString("token")
	if err != nil {
		return err
	}

	var out string
	if _, err := os.Stat(schemaPath); err == nil {
		out = filepath.Join(strings.TrimSuffix(schemaPath, filepath.Ext(schemaPath)) + ".csv")
	} else {
		out = defaultOutFile
	}

	if cmd.Flags().Lookup(outFlag).Changed {
		out, err = cmd.Flags().GetString(outFlag)
		if err != nil {
			return err
		}
	}

	debug, err := cmd.Flags().GetBool("debug")
	if err != nil {
		return err
	}

	if err = usage.OutputUsage(cmd, schemaPath, header, token, out, debug); err != nil {
		rootCmd.SilenceUsage = true
		return fmt.Errorf(utils.Red("%w"), err)
	}

	return nil
}
