package cmd

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/speakeasy-api/speakeasy/internal/usage"
	"github.com/speakeasy-api/speakeasy/internal/utils"
	"github.com/spf13/cobra"
)

const (
	fileFlag = "file"
	outFlag  = "out"
)

func usageInit() {
	usageCmd := &cobra.Command{
		Use:   "usage",
		Short: "Output usage information for a given OpenAPI schema to a CSV",
		Long:  `Output usage information containing counts of OpenAPI features for a given OpenAPI schema to a CSV`,
		RunE:  genUsage,
	}
	usageCmd.Flags().StringP("file", "f", "", "Path to file to generate usage information for")
	usageCmd.MarkFlagRequired("file")
	usageCmd.Flags().StringP("out", "o", "", "Path to output file")
	usageCmd.Flags().BoolP("debug", "d", false, "enable writing debug files with broken code")

	rootCmd.AddCommand(usageCmd)
}

func genUsage(cmd *cobra.Command, args []string) error {
	file, err := cmd.Flags().GetString(fileFlag)
	if err != nil {
		return err
	}
	if file == "" {
		return fmt.Errorf("%s not set", fileFlag)
	}

	out := filepath.Join(strings.TrimSuffix(file, filepath.Ext(file)) + ".csv")
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

	if err = usage.OutputUsage(cmd, file, out, debug); err != nil {
		rootCmd.SilenceUsage = true
		return fmt.Errorf(utils.Red("%w"), err)
	}

	return nil
}
