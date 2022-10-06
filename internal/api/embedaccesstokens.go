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
		statusRes, ok := res.Responses[res.StatusCode]
		if !ok {
			return fmt.Errorf("unexpected status code: %d", res.StatusCode)
		}

		errorRes := statusRes[res.ContentType]
		return fmt.Errorf("error: %s, statusCode: %d", errorRes.Error.Message, res.StatusCode)
	}

	utils.PrintArray(cmd, res.Responses[res.StatusCode][res.ContentType].EmbedTokens, nil)

	return nil
}
