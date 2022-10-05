package api

import (
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

	res, err := s.GetApis(ctx, operations.GetApisRequest{})
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

	utils.PrettyPrintArray(res.Responses[res.StatusCode][res.ContentType].Apis, map[string]string{
		"APIID": "ApiID",
	})

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

	res, err := s.GetAllAPIEndpoints(ctx, operations.GetAllAPIEndpointsRequest{
		PathParams: operations.GetAllAPIEndpointsPathParams{
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

	utils.PrettyPrintArray(res.Responses[res.StatusCode][res.ContentType].APIEndpoints, map[string]string{
		"APIID": "ApiID",
	})

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

	res, err := s.RegisterSchema(ctx, operations.RegisterSchemaRequest{
		PathParams: operations.RegisterSchemaPathParams{
			APIID:     apiID,
			VersionID: versionID,
		},
		Request: operations.RegisterSchemaRequestBody{
			File: operations.RegisterSchemaRequestBodyFile{
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

	if len(res.Responses[res.StatusCode][res.ContentType].VersionMetadata) == 0 {
		fmt.Println("no metadata found")
		return nil
	}

	utils.PrettyPrintArray(res.Responses[res.StatusCode][res.ContentType].VersionMetadata, map[string]string{
		"APIID":     "ApiID",
		"MetaKey":   "Key",
		"MetaValue": "Value",
	})

	return nil
}
