package api

import (
	"encoding/json"
	"fmt"

	"github.com/speakeasy-api/speakeasy-client-sdk-go/pkg/models/operations"
	"github.com/speakeasy-api/speakeasy-client-sdk-go/pkg/models/shared"
	"github.com/speakeasy-api/speakeasy/internal/utils"
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

	s, err := initSDK()
	if err != nil {
		return err
	}

	res, err := s.QueryEventLog(ctx, operations.QueryEventLogRequest{
		QueryParams: operations.QueryEventLogQueryParams{
			Filters: f,
		},
	})
	if err != nil {
		return err // TODO wrap
	}

	if res.StatusCode != 200 {
		return fmt.Errorf("error: %s, statusCode: %d", res.Error.Message, res.StatusCode)
	}

	utils.PrintArray(cmd, res.BoundedRequests, map[string]string{
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

	s, err := initSDK()
	if err != nil {
		return err
	}

	res, err := s.GetRequestFromEventLog(ctx, operations.GetRequestFromEventLogRequest{
		PathParams: operations.GetRequestFromEventLogPathParams{
			RequestID: requestID,
		},
	})
	if err != nil {
		return err // TODO wrap
	}

	if res.StatusCode != 200 {
		return fmt.Errorf("error: %s, statusCode: %d", res.Error.Message, res.StatusCode)
	}

	utils.PrintValue(cmd, res.UnboundedRequest, map[string]string{
		"APIID":         "ApiID",
		"APIEndpointID": "ApiEndpointID",
	})

	return nil
}
