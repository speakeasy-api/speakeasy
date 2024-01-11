package api

import (
	"fmt"
	"github.com/speakeasy-api/speakeasy/internal/log"

	"github.com/hexops/gotextdiff"
	"github.com/hexops/gotextdiff/myers"
	"github.com/hexops/gotextdiff/span"
	"github.com/speakeasy-api/speakeasy-client-sdk-go/pkg/models/operations"
	"github.com/speakeasy-api/speakeasy/internal/sdk"
	"github.com/spf13/cobra"
)

func getAllAPIEndpoints(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	apiID, err := getStringFlag(cmd, "api-id")
	if err != nil {
		return err
	}

	s, err := sdk.InitSDK("")
	if err != nil {
		return err
	}

	res, err := s.APIEndpoints.GetAllAPIEndpoints(ctx, operations.GetAllAPIEndpointsRequest{
		APIID: apiID,
	})
	if err != nil {
		return err // TODO wrap
	}

	if res.StatusCode != 200 {
		return fmt.Errorf("error: %s, statusCode: %d", res.Error.Message, res.StatusCode)
	}

	log.PrintArray(cmd, res.APIEndpoints, map[string]string{
		"APIID":         "ApiID",
		"APIEndpointID": "ApiEndpointID",
	})

	return nil
}

func getAllAPIEndpointsForVersion(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	apiID, err := getStringFlag(cmd, "api-id")
	if err != nil {
		return err
	}

	versionID, err := getStringFlag(cmd, "version-id")
	if err != nil {
		return err
	}

	s, err := sdk.InitSDK("")
	if err != nil {
		return err
	}

	res, err := s.APIEndpoints.GetAllForVersionAPIEndpoints(ctx, operations.GetAllForVersionAPIEndpointsRequest{
		APIID:     apiID,
		VersionID: versionID,
	})
	if err != nil {
		return err // TODO wrap
	}

	if res.StatusCode != 200 {
		return fmt.Errorf("error: %s, statusCode: %d", res.Error.Message, res.StatusCode)
	}

	log.PrintArray(cmd, res.APIEndpoints, map[string]string{
		"APIID":         "ApiID",
		"APIEndpointID": "ApiEndpointID",
	})

	return nil
}

func getApiEndpoint(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	apiID, err := getStringFlag(cmd, "api-id")
	if err != nil {
		return err
	}

	versionID, err := getStringFlag(cmd, "version-id")
	if err != nil {
		return err
	}

	apiEndpointID, err := getStringFlag(cmd, "api-endpoint-id")
	if err != nil {
		return err
	}

	s, err := sdk.InitSDK("")
	if err != nil {
		return err
	}

	res, err := s.APIEndpoints.GetAPIEndpoint(ctx, operations.GetAPIEndpointRequest{
		APIID:         apiID,
		VersionID:     versionID,
		APIEndpointID: apiEndpointID,
	})
	if err != nil {
		return err // TODO wrap
	}

	if res.StatusCode != 200 {
		return fmt.Errorf("error: %s, statusCode: %d", res.Error.Message, res.StatusCode)
	}

	log.PrintValue(cmd, res.APIEndpoint, map[string]string{
		"APIID":         "ApiID",
		"APIEndpointID": "ApiEndpointID",
	})

	return nil
}

func findApiEndpoint(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	apiID, err := getStringFlag(cmd, "api-id")
	if err != nil {
		return err
	}

	versionID, err := getStringFlag(cmd, "version-id")
	if err != nil {
		return err
	}

	displayName, err := getStringFlag(cmd, "display-name")
	if err != nil {
		return err
	}

	s, err := sdk.InitSDK("")
	if err != nil {
		return err
	}

	res, err := s.APIEndpoints.FindAPIEndpoint(ctx, operations.FindAPIEndpointRequest{
		APIID:       apiID,
		VersionID:   versionID,
		DisplayName: displayName,
	})
	if err != nil {
		return err // TODO wrap
	}

	if res.StatusCode != 200 {
		return fmt.Errorf("error: %s, statusCode: %d", res.Error.Message, res.StatusCode)
	}

	log.PrintValue(cmd, res.APIEndpoint, map[string]string{
		"APIID": "ApiID",
	})

	return nil
}

func generateOpenAPISpecForAPIEndpoint(cmd *cobra.Command, args []string) error {
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

	apiEndpointID, err := getStringFlag(cmd, "api-endpoint-id")
	if err != nil {
		return err
	}

	diff, _ := cmd.Flags().GetBool("diff")

	s, err := sdk.InitSDK("")
	if err != nil {
		return err
	}

	res, err := s.APIEndpoints.GenerateOpenAPISpecForAPIEndpoint(ctx, operations.GenerateOpenAPISpecForAPIEndpointRequest{
		APIID:         apiID,
		VersionID:     versionID,
		APIEndpointID: apiEndpointID,
	})
	if err != nil {
		return err // TODO wrap
	}

	if res.StatusCode != 200 {
		return fmt.Errorf("error: %s, statusCode: %d", res.Error.Message, res.StatusCode)
	}

	specDiff := res.GenerateOpenAPISpecDiff

	if diff && specDiff.CurrentSchema != "" {
		edits := myers.ComputeEdits(span.URIFromPath("openapi"), specDiff.CurrentSchema, specDiff.NewSchema)
		logger.PrintlnUnstyled(gotextdiff.ToUnified("openapi", "openapi", specDiff.CurrentSchema, edits))
	} else {
		logger.Println(res.GenerateOpenAPISpecDiff.NewSchema)
	}

	return nil
}

func generatePostmanCollectionForAPIEndpoint(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	apiID, err := getStringFlag(cmd, "api-id")
	if err != nil {
		return err
	}

	versionID, err := getStringFlag(cmd, "version-id")
	if err != nil {
		return err
	}

	apiEndpointID, err := getStringFlag(cmd, "api-endpoint-id")
	if err != nil {
		return err
	}

	s, err := sdk.InitSDK("")
	if err != nil {
		return err
	}

	res, err := s.APIEndpoints.GeneratePostmanCollectionForAPIEndpoint(ctx, operations.GeneratePostmanCollectionForAPIEndpointRequest{
		APIID:         apiID,
		VersionID:     versionID,
		APIEndpointID: apiEndpointID,
	})
	if err != nil {
		return err // TODO wrap
	}

	if res.StatusCode != 200 {
		return fmt.Errorf("error: %s, statusCode: %d", res.Error.Message, res.StatusCode)
	}

	log.From(ctx).PrintlnUnstyled(res.PostmanCollection)

	return nil
}
