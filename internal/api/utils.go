package api

import (
	"fmt"

	"github.com/spf13/cobra"
)

func getStringFlag(cmd *cobra.Command, flag string) (string, error) {
	val, err := cmd.Flags().GetString(flag)
	if err != nil {
		return "", err
	}
	if val == "" {
		return "", fmt.Errorf("%s not set", flag)
	}

	return val, nil
}
