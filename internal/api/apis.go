package api

import (
	"fmt"

	"github.com/hexops/gotextdiff"
	"github.com/hexops/gotextdiff/myers"
	"github.com/hexops/gotextdiff/span"
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
		return fmt.Errorf("error: %s, statusCode: %d", res.Error.Message, res.StatusCode)
	}

	utils.PrintArray(cmd, res.Apis, map[string]string{
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

	s, err := initSDK()
	if err != nil {
		return err
	}

	res, err := s.GetAllAPIVersions(ctx, operations.GetAllAPIVersionsRequest{
		PathParams: operations.GetAllAPIVersionsPathParams{
			APIID: apiID,
		},
	})
	if err != nil {
		return err // TODO wrap
	}

	if res.StatusCode != 200 {
		return fmt.Errorf("error: %s, statusCode: %d", res.Error.Message, res.StatusCode)
	}

	utils.PrintArray(cmd, res.Apis, map[string]string{
		"APIID": "ApiID",
	})

	return nil
}

func generateOpenAPISpec(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	apiID, err := getStringFlag(cmd, "api-id")
	if err != nil {
		return err
	}

	versionID, err := getStringFlag(cmd, "version-id")
	if err != nil {
		return err
	}

	diff, _ := cmd.Flags().GetBool("diff")

	s, err := initSDK()
	if err != nil {
		return err
	}

	res, err := s.GenerateOpenAPISpec(ctx, operations.GenerateOpenAPISpecRequest{
		PathParams: operations.GenerateOpenAPISpecPathParams{
			APIID:     apiID,
			VersionID: versionID,
		},
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
		fmt.Println(gotextdiff.ToUnified("openapi", "openapi", specDiff.CurrentSchema, edits))
	} else {
		fmt.Println(res.GenerateOpenAPISpecDiff.NewSchema)
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

	s, err := initSDK()
	if err != nil {
		return err
	}

	res, err := s.GeneratePostmanCollection(ctx, operations.GeneratePostmanCollectionRequest{
		PathParams: operations.GeneratePostmanCollectionPathParams{
			APIID:     apiID,
			VersionID: versionID,
		},
	})
	if err != nil {
		return err // TODO wrap
	}

	if res.StatusCode != 200 {
		return fmt.Errorf("error: %s, statusCode: %d", res.Error.Message, res.StatusCode)
	}

	fmt.Println(string(res.PostmanCollection))

	return nil
}
