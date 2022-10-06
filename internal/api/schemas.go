package api

import (
	"fmt"
	"os"
	"path"

	"github.com/speakeasy-api/speakeasy-client-sdk-go/pkg/models/operations"
	"github.com/speakeasy-api/speakeasy/internal/utils"
	"github.com/spf13/cobra"
)

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

func getSchemas(cmd *cobra.Command, args []string) error {
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

	res, err := s.GetSchemas(ctx, operations.GetSchemasRequest{
		PathParams: operations.GetSchemasPathParams{
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

	utils.PrintArray(cmd, res.Responses[res.StatusCode][res.ContentType].Schemata, map[string]string{
		"APIID": "ApiID",
	})

	return nil
}

func getSchemaRevision(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	apiID, err := getStringFlag(cmd, "api-id")
	if err != nil {
		return err
	}

	versionID, err := getStringFlag(cmd, "version-id")
	if err != nil {
		return err
	}

	revisionID, err := getStringFlag(cmd, "revision-id")
	if err != nil {
		return err
	}

	s, err := initSDK()
	if err != nil {
		return err
	}

	res, err := s.GetSchemaRevision(ctx, operations.GetSchemaRevisionRequest{
		PathParams: operations.GetSchemaRevisionPathParams{
			APIID:      apiID,
			VersionID:  versionID,
			RevisionID: revisionID,
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

	utils.PrintValue(cmd, res.Responses[res.StatusCode][res.ContentType].Schema, map[string]string{
		"APIID": "ApiID",
	})

	return nil
}

func getSchemaDiff(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	apiID, err := getStringFlag(cmd, "api-id")
	if err != nil {
		return err
	}

	versionID, err := getStringFlag(cmd, "version-id")
	if err != nil {
		return err
	}

	baseRevisionID, err := getStringFlag(cmd, "base-revision-id")
	if err != nil {
		return err
	}

	targetRevisionID, err := getStringFlag(cmd, "target-revision-id")
	if err != nil {
		return err
	}

	s, err := initSDK()
	if err != nil {
		return err
	}

	res, err := s.GetSchemaDiff(ctx, operations.GetSchemaDiffRequest{
		PathParams: operations.GetSchemaDiffPathParams{
			APIID:            apiID,
			VersionID:        versionID,
			BaseRevisionID:   baseRevisionID,
			TargetRevisionID: targetRevisionID,
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

	utils.PrintValue(cmd, res.Responses[res.StatusCode][res.ContentType].SchemaDiff, nil)

	return nil
}

func downloadLatestSchema(cmd *cobra.Command, args []string) error {
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

	res, err := s.DownloadSchema(ctx, operations.DownloadSchemaRequest{
		PathParams: operations.DownloadSchemaPathParams{
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

	fmt.Println(string(res.Responses[res.StatusCode][res.ContentType].Schema))

	return nil
}

func downloadSchemaRevision(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	apiID, err := getStringFlag(cmd, "api-id")
	if err != nil {
		return err
	}

	versionID, err := getStringFlag(cmd, "version-id")
	if err != nil {
		return err
	}

	revisionID, err := getStringFlag(cmd, "revision-id")
	if err != nil {
		return err
	}

	s, err := initSDK()
	if err != nil {
		return err
	}

	res, err := s.DownloadSchemaRevision(ctx, operations.DownloadSchemaRevisionRequest{
		PathParams: operations.DownloadSchemaRevisionPathParams{
			APIID:      apiID,
			VersionID:  versionID,
			RevisionID: revisionID,
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

	fmt.Println(string(res.Responses[res.StatusCode][res.ContentType].Schema))

	return nil
}
