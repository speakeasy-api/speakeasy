package api

import (
	"fmt"
	"io"

	"github.com/speakeasy-api/speakeasy/internal/log"

	"github.com/hexops/gotextdiff"
	"github.com/hexops/gotextdiff/myers"
	"github.com/hexops/gotextdiff/span"
	"github.com/speakeasy-api/speakeasy-client-sdk-go/v3/pkg/models/operations"
	"github.com/speakeasy-api/speakeasy/internal/sdk"
	"github.com/spf13/cobra"
)

func getApis(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	s, err := sdk.InitSDK()
	if err != nil {
		return err
	}

	res, err := s.Apis.GetApis(ctx, operations.GetApisRequest{})
	if err != nil {
		return err // TODO wrap
	}

	if res.StatusCode != 200 {
		return fmt.Errorf("statusCode: %d", res.StatusCode)
	}

	log.PrintArray(cmd, res.Apis, map[string]string{
		"APIID": "ApiID",
	})

	return nil
}

func getApiVersions(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	apiID, err := getStringFlag(cmd, "api-id")
	if err != nil {
		return err
	}

	s, err := sdk.InitSDK()
	if err != nil {
		return err
	}

	res, err := s.Apis.GetAllAPIVersions(ctx, operations.GetAllAPIVersionsRequest{
		APIID: apiID,
	})
	if err != nil {
		return err // TODO wrap
	}

	if res.StatusCode != 200 {
		return fmt.Errorf("statusCode: %d", res.StatusCode)
	}

	log.PrintArray(cmd, res.Apis, map[string]string{
		"APIID": "ApiID",
	})

	return nil
}

func generateOpenAPISpec(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	logger := log.From(ctx)

	apiID, err := getStringFlag(cmd, "api-id")
	if err != nil {
		return err
	}

	versionID, err := getStringFlag(cmd, "version-id")
	if err != nil {
		return err
	}

	diff, _ := cmd.Flags().GetBool("diff")

	s, err := sdk.InitSDK()
	if err != nil {
		return err
	}

	res, err := s.Apis.GenerateOpenAPISpec(ctx, operations.GenerateOpenAPISpecRequest{
		APIID:     apiID,
		VersionID: versionID,
	})
	if err != nil {
		return err // TODO wrap
	}

	if res.StatusCode != 200 {
		return fmt.Errorf("statusCode: %d", res.StatusCode)
	}

	specDiff := res.GenerateOpenAPISpecDiff

	if diff && specDiff.CurrentSchema != "" {
		edits := myers.ComputeEdits(span.URIFromPath("openapi"), specDiff.CurrentSchema, specDiff.NewSchema)
		logger.PrintlnUnstyled(gotextdiff.ToUnified("openapi", "openapi", specDiff.CurrentSchema, edits))
	} else {
		logger.PrintlnUnstyled(res.GenerateOpenAPISpecDiff.NewSchema)
	}

	return nil
}

func generatePostmanCollection(cmd *cobra.Command, args []string) error {
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

	res, err := s.Apis.GeneratePostmanCollection(ctx, operations.GeneratePostmanCollectionRequest{
		APIID:     apiID,
		VersionID: versionID,
	})
	if err != nil {
		return err // TODO wrap
	}

	if res.StatusCode != 200 {
		return fmt.Errorf("statusCode: %d", res.StatusCode)
	}

	collection, err := io.ReadAll(res.PostmanCollection)
	if err != nil {
		return err
	}
	log.From(ctx).Println(string(collection))

	return nil
}
