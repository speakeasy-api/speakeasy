package api

import (
	"fmt"

	"github.com/speakeasy-api/speakeasy/internal/utils"
	"github.com/spf13/cobra"
)

func getValidEmbedAccessTokens(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	s, err := initSDK()
	if err != nil {
		return err
	}

	res, err := s.GetValidEmbedAccessTokens(ctx)
	if err != nil {
		return err // TODO wrap
	}

	if res.StatusCode != 200 {
		return fmt.Errorf("error: %s, statusCode: %d", res.Error.Message, res.StatusCode)
	}

	utils.PrintArray(cmd, res.EmbedTokens, nil)

	return nil
}
