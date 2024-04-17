package api

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/speakeasy-api/speakeasy/internal/log"

	"github.com/speakeasy-api/speakeasy-client-sdk-go/v3/pkg/models/operations"
	"github.com/speakeasy-api/speakeasy/internal/sdk"
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

	s, err := sdk.InitSDK("")
	if err != nil {
		return err
	}

	res, err := s.Schemas.RegisterSchema(ctx, operations.RegisterSchemaRequest{
		APIID:     apiID,
		VersionID: versionID,
		RequestBody: operations.RegisterSchemaRequestBody{
			File: operations.RegisterSchemaFile{
				Content:  data,
				FileName: filepath.Base(schemaPath),
			},
		},
	})
	if err != nil {
		return err // TODO wrap
	}

	if res.StatusCode != 200 {
		return fmt.Errorf("error: %s, statusCode: %d", res.Error.Message, res.StatusCode)
	}

	log.From(ctx).Successf("Schema successfully registered for: %s - %s %s ✓", apiID, versionID)

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

	s, err := sdk.InitSDK("")
	if err != nil {
		return err
	}

	res, err := s.Schemas.GetSchemas(ctx, operations.GetSchemasRequest{
		APIID:     apiID,
		VersionID: versionID,
	})
	if err != nil {
		return err // TODO wrap
	}

	if res.StatusCode != 200 {
		return fmt.Errorf("error: %s, statusCode: %d", res.Error.Message, res.StatusCode)
	}

	log.PrintArray(cmd, res.Classes, map[string]string{
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

	s, err := sdk.InitSDK("")
	if err != nil {
		return err
	}

	res, err := s.Schemas.GetSchemaRevision(ctx, operations.GetSchemaRevisionRequest{
		APIID:      apiID,
		VersionID:  versionID,
		RevisionID: revisionID,
	})
	if err != nil {
		return err // TODO wrap
	}

	if res.StatusCode != 200 {
		return fmt.Errorf("error: %s, statusCode: %d", res.Error.Message, res.StatusCode)
	}

	log.PrintValue(cmd, res.Schema, map[string]string{
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

	s, err := sdk.InitSDK("")
	if err != nil {
		return err
	}

	res, err := s.Schemas.GetSchemaDiff(ctx, operations.GetSchemaDiffRequest{
		APIID:            apiID,
		VersionID:        versionID,
		BaseRevisionID:   baseRevisionID,
		TargetRevisionID: targetRevisionID,
	})
	if err != nil {
		return err // TODO wrap
	}

	if res.StatusCode != 200 {
		return fmt.Errorf("error: %s, statusCode: %d", res.Error.Message, res.StatusCode)
	}

	log.PrintValue(cmd, res.SchemaDiff, nil)

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

	s, err := sdk.InitSDK("")
	if err != nil {
		return err
	}

	res, err := s.Schemas.DownloadSchema(ctx, operations.DownloadSchemaRequest{
		APIID:     apiID,
		VersionID: versionID,
	})
	if err != nil {
		return err // TODO wrap
	}

	if res.StatusCode != 200 {
		return fmt.Errorf("error: %s, statusCode: %d", res.Error.Message, res.StatusCode)
	}

	if res.TwoHundredApplicationJSONSchema != nil {
		defer res.TwoHundredApplicationJSONSchema.Close()
		jsonSchema, err := io.ReadAll(res.TwoHundredApplicationJSONSchema)
		if err != nil {
			return err
		}
		log.From(ctx).Println(string(jsonSchema))
	}
	if res.TwoHundredApplicationXYamlSchema != nil {
		defer res.TwoHundredApplicationXYamlSchema.Close()
		yamlSchema, err := io.ReadAll(res.TwoHundredApplicationXYamlSchema)
		if err != nil {
			return err
		}
		log.From(ctx).Println(string(yamlSchema))
	}

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

	s, err := sdk.InitSDK("")
	if err != nil {
		return err
	}

	res, err := s.Schemas.DownloadSchemaRevision(ctx, operations.DownloadSchemaRevisionRequest{
		APIID:      apiID,
		VersionID:  versionID,
		RevisionID: revisionID,
	})
	if err != nil {
		return err // TODO wrap
	}

	if res.StatusCode != 200 {
		return fmt.Errorf("error: %s, statusCode: %d", res.Error.Message, res.StatusCode)
	}

	if res.TwoHundredApplicationJSONSchema != nil {
		defer res.TwoHundredApplicationJSONSchema.Close()
		jsonSchema, err := io.ReadAll(res.TwoHundredApplicationJSONSchema)
		if err != nil {
			return err
		}
		log.From(ctx).Println(string(jsonSchema))
	}
	if res.TwoHundredApplicationXYamlSchema != nil {
		defer res.TwoHundredApplicationXYamlSchema.Close()
		yamlSchema, err := io.ReadAll(res.TwoHundredApplicationXYamlSchema)
		if err != nil {
			return err
		}
		log.From(ctx).Println(string(yamlSchema))
	}

	return nil
}
