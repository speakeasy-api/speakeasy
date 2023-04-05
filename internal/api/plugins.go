package api

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/speakeasy-api/speakeasy-client-sdk-go/pkg/models/operations"
	"github.com/speakeasy-api/speakeasy-client-sdk-go/pkg/models/shared"
	"github.com/speakeasy-api/speakeasy/internal/sdk"
	"github.com/speakeasy-api/speakeasy/internal/utils"
	"github.com/spf13/cobra"
)

func getPlugins(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	s, err := sdk.InitSDK("")
	if err != nil {
		return err
	}

	res, err := s.Plugins.GetPlugins(ctx)
	if err != nil {
		return err // TODO wrap
	}

	if res.StatusCode != 200 {
		return fmt.Errorf("error: %s, statusCode: %d", res.Error.Message, res.StatusCode)
	}

	utils.PrintArray(cmd, res.Plugins, map[string]string{
		"APIID": "ApiID",
	})

	return nil
}

func upsertPlugin(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	s, err := sdk.InitSDK("")
	if err != nil {
		return err
	}

	pluginID, err := getStringFlag(cmd, "plugin-id")
	if err != nil {
		return err
	}
	title, err := getStringFlag(cmd, "title")
	if err != nil {
		return err
	}

	codePath, err := getStringFlag(cmd, "file")
	if err != nil {
		return err
	}

	code, err := os.ReadFile(codePath)
	if err != nil {
		return err
	}

	res, err := s.Plugins.UpsertPlugin(ctx, shared.Plugin{
		PluginID: pluginID,
		Code:     string(code),
		Title:    title,
	})
	if err != nil {
		return err // TODO wrap
	}

	if res.StatusCode != 200 {
		return fmt.Errorf("error: %s, statusCode: %d", res.Error.Message, res.StatusCode)
	}

	utils.PrintValue(cmd, res.Plugin, map[string]string{})

	return nil
}

func runPlugin(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	s, err := sdk.InitSDK("")
	if err != nil {
		return err
	}

	pluginID, err := getStringFlag(cmd, "plugin-id")
	if err != nil {
		return err
	}

	filters, _ := cmd.Flags().GetString("filters")

	var f *shared.Filters

	if filters != "" {
		if err := json.Unmarshal([]byte(filters), &f); err != nil {
			return err
		}
	}

	res, err := s.Plugins.RunPlugin(ctx, operations.RunPluginRequest{
		Filters:  f,
		PluginID: pluginID,
	})
	if err != nil {
		return err // TODO wrap
	}
	fmt.Println(res.StatusCode)
	if res.StatusCode != 200 {
		return fmt.Errorf("error: %s, statusCode: %d", res.Error.Message, res.StatusCode)
	}

	utils.PrintArray(cmd, res.BoundedRequests, map[string]string{
		"APIID": "ApiID",
	})

	return nil
}
