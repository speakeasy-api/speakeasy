package api

import (
	"encoding/json"
	"fmt"
	"os"
	"path"

	"github.com/speakeasy-api/speakeasy-client-sdk-go/pkg/models/operations"
	"github.com/speakeasy-api/speakeasy/internal/utils"
	"github.com/spf13/cobra"
)

func getApis(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	s, err := initSDK()
	if err != nil {
		return err
	}

	res, err := s.GetApisV1(ctx, operations.GetApisV1Request{})
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

	fmt.Println("--------------------------------------")

	for _, api := range res.Responses[res.StatusCode][res.ContentType].Apis {
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

	apiID, err := getStringFlag(cmd, "api-id")
	if err != nil {
		return err
	}

	s, err := initSDK()
	if err != nil {
		return err
	}

	res, err := s.GetAllAPIEndpointsV1(ctx, operations.GetAllAPIEndpointsV1Request{
		PathParams: operations.GetAllAPIEndpointsV1PathParams{
			APIID: apiID,
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

	fmt.Println("--------------------------------------")

	for _, endpoint := range res.Responses[res.StatusCode][res.ContentType].APIEndpoints {
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

func registerSchema(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	apiID, err := getStringFlag(cmd, "api-id")
	if err != nil {
		return err
	}

	versionID, err := getStringFlag(cmd, "version-id")
	if err != nil {
		return err
	}

	schemaPath, err := getStringFlag(cmd, "schema")
	if err != nil {
		return err
	}

	data, err := os.ReadFile(schemaPath)
	if err != nil {
		return err // TODO wrap
	}

	s, err := initSDK()
	if err != nil {
		return err
	}

	res, err := s.RegisterSchemaV1(ctx, operations.RegisterSchemaV1Request{
		PathParams: operations.RegisterSchemaV1PathParams{
			APIID:     apiID,
			VersionID: versionID,
		},
		Request: operations.RegisterSchemaV1RequestBody{
			File: operations.RegisterSchemaV1RequestBodyFile{
				Content: data,
				File:    path.Base(schemaPath),
			},
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

	fmt.Printf("schema successfully registered for: %s - %s %s\n", apiID, versionID, utils.Green("✓"))

	return nil
}

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

	res, err := s.GetVersionMetadataV1(ctx, operations.GetVersionMetadataV1Request{
		PathParams: operations.GetVersionMetadataV1PathParams{
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

	if len(res.Responses[res.StatusCode][res.ContentType].VersionMetadata) == 0 {
		fmt.Println("no metadata found")
		return nil
	}

	fmt.Println("--------------------------------------")

	for _, metadata := range res.Responses[res.StatusCode][res.ContentType].VersionMetadata {
		fmt.Printf(`ApiID: %s
VersionID: %s
Key: %s
Value: %s
CreatedAt: %s
`, metadata.APIID, metadata.VersionID, metadata.MetaKey, metadata.MetaValue, metadata.CreatedAt)

		fmt.Println("--------------------------------------")
	}

	return nil
}
