package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	sdk "github.com/speakeasy-api/speakeasy-client-sdk-go"
	"github.com/speakeasy-api/speakeasy-client-sdk-go/pkg/models/operations"
	"github.com/speakeasy-api/speakeasy-client-sdk-go/pkg/models/shared"
	"github.com/spf13/cobra"
)

func getApis(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	apiKey := os.Getenv("SPEAKEASY_API_KEY")
	if apiKey == "" {
		return errors.New("SPEAKEASY_API_KEY not set")
	}

	s := sdk.New()
	s.ConfigureSecurity(shared.Security{
		APIKey: shared.APIKey{
			APIKey: apiKey,
		},
	})

	res, err := s.GetApisV1(ctx, operations.GetApisV1Request{})
	if err != nil {
		return err // TODO wrap
	}

	if res.StatusCode != 200 {
		return fmt.Errorf("unexpected status code: %d", res.StatusCode)
	}

	fmt.Println("--------------------------------------")

	for _, api := range res.Responses[res.StatusCode][res.ContentType].API {
		metadata, err := json.Marshal(api.MetaData)
		if err != nil {
			return err
		}

		matched := false
		if api.Matched != nil {
			matched = *api.Matched
		}

		fmt.Printf(`ApiID: %s
VersionID: %s
Description: %s
MetaData: %s
Matched: %t
CreatedAt: %s
UpdatedAt: %s
`, api.APIID, api.VersionID, api.Description, string(metadata), matched, api.CreatedAt, api.UpdatedAt)

		fmt.Println("--------------------------------------")
	}

	return nil
}

func getAllAPIEndpoints(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	apiKey := os.Getenv("SPEAKEASY_API_KEY")
	if apiKey == "" {
		return errors.New("SPEAKEASY_API_KEY not set")
	}

	apiID, err := cmd.Flags().GetString("api-id")
	if err != nil {
		return err
	}
	if apiID == "" {
		return errors.New("api-id not set")
	}

	s := sdk.New()
	s.ConfigureSecurity(shared.Security{
		APIKey: shared.APIKey{
			APIKey: apiKey,
		},
	})

	res, err := s.GetAllAPIEndpointsV1(ctx, operations.GetAllAPIEndpointsV1Request{
		PathParams: operations.GetAllAPIEndpointsV1PathParams{
			APIID: apiID,
		},
	})
	if err != nil {
		return err // TODO wrap
	}

	if res.StatusCode != 200 {
		return fmt.Errorf("unexpected status code: %d", res.StatusCode)
	}

	fmt.Println("--------------------------------------")

	for _, endpoint := range res.Responses[res.StatusCode][res.ContentType].APIEndpoint {
		fmt.Printf(`ApiID: %s
VersionID: %s
ApiEndpointID: %s
Description: %s
Method: %s
Path: %s
CreatedAt: %s
UpdatedAt: %s
`, endpoint.APIID, endpoint.VersionID, endpoint.APIEndpointID, endpoint.Description, endpoint.Method, endpoint.Path, endpoint.CreatedAt, endpoint.UpdatedAt)

		fmt.Println("--------------------------------------")
	}

	return nil
}
