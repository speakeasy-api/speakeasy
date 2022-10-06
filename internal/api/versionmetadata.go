package api

import (
	"fmt"

	"github.com/speakeasy-api/speakeasy-client-sdk-go/pkg/models/operations"
	"github.com/speakeasy-api/speakeasy/internal/utils"
	"github.com/spf13/cobra"
)

func getVersionMetadata(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	apiID, err := getStringFlag(cmd, "api-id")
	if err != nil {
		return err
	}

	versionID, err := getStringFlag(cmd, "version-id")
	if err != nil {
		return err
	}

	s, err := initSDK()
	if err != nil {
		return err
	}

	res, err := s.GetVersionMetadata(ctx, operations.GetVersionMetadataRequest{
		PathParams: operations.GetVersionMetadataPathParams{
			APIID:     apiID,
			VersionID: versionID,
		},
	})
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

	utils.PrintArray(cmd, res.Responses[res.StatusCode][res.ContentType].VersionMetadata, map[string]string{
		"APIID":     "ApiID",
		"MetaKey":   "Key",
		"MetaValue": "Value",
	})

	return nil
}
