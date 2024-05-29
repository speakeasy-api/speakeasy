package api

import (
	"fmt"
	"github.com/speakeasy-api/speakeasy/internal/log"

	"github.com/speakeasy-api/speakeasy-client-sdk-go/v3/pkg/models/operations"
	"github.com/speakeasy-api/speakeasy/internal/sdk"
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

	s, err := sdk.InitSDK()
	if err != nil {
		return err
	}

	res, err := s.Metadata.GetVersionMetadata(ctx, operations.GetVersionMetadataRequest{
		APIID:     apiID,
		VersionID: versionID,
	})
	if err != nil {
		return err // TODO wrap
	}

	if res.StatusCode != 200 {
		return fmt.Errorf("error: %s, statusCode: %d", res.Error.Message, res.StatusCode)
	}

	log.PrintArray(cmd, res.VersionMetadata, map[string]string{
		"APIID":     "ApiID",
		"MetaKey":   "Key",
		"MetaValue": "Value",
	})

	return nil
}
