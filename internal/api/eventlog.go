package api

import (
	"encoding/json"
	"fmt"
	"github.com/speakeasy-api/speakeasy/internal/log"

	"github.com/speakeasy-api/speakeasy-client-sdk-go/v3/pkg/models/operations"
	"github.com/speakeasy-api/speakeasy-client-sdk-go/v3/pkg/models/shared"
	"github.com/speakeasy-api/speakeasy/internal/sdk"
	"github.com/spf13/cobra"
)

func queryEventLog(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	filters, _ := cmd.Flags().GetString("filters")

	var f *shared.Filters

	if filters != "" {
		if err := json.Unmarshal([]byte(filters), &f); err != nil {
			return err
		}
	}

	s, err := sdk.InitSDK("")
	if err != nil {
		return err
	}

	res, err := s.Requests.QueryEventLog(ctx, operations.QueryEventLogRequest{
		Filters: f,
	})
	if err != nil {
		return err // TODO wrap
	}

	if res.StatusCode != 200 {
		return fmt.Errorf("error: %s, statusCode: %d", res.Error.Message, res.StatusCode)
	}

	log.PrintArray(cmd, res.BoundedRequests, map[string]string{
		"APIID":         "ApiID",
		"APIEndpointID": "ApiEndpointID",
	})

	return nil
}

func getRequestFromEventLog(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	requestID, err := getStringFlag(cmd, "request-id")
	if err != nil {
		return err
	}

	s, err := sdk.InitSDK("")
	if err != nil {
		return err
	}

	res, err := s.Requests.GetRequestFromEventLog(ctx, operations.GetRequestFromEventLogRequest{
		RequestID: requestID,
	})
	if err != nil {
		return err // TODO wrap
	}

	if res.StatusCode != 200 {
		return fmt.Errorf("error: %s, statusCode: %d", res.Error.Message, res.StatusCode)
	}

	log.PrintValue(cmd, res.UnboundedRequest, map[string]string{
		"APIID":         "ApiID",
		"APIEndpointID": "ApiEndpointID",
	})

	return nil
}
