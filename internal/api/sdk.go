package api

import (
	"errors"
	"os"

	sdk "github.com/speakeasy-api/speakeasy-client-sdk-go"
	"github.com/speakeasy-api/speakeasy-client-sdk-go/pkg/models/shared"
)

func initSDK() (*sdk.SDK, error) {
	apiKey := os.Getenv("SPEAKEASY_API_KEY")
	if apiKey == "" {
		return nil, errors.New("SPEAKEASY_API_KEY not set")
	}

	opts := []sdk.SDKOption{
		sdk.WithSecurity(shared.Security{
			APIKey: shared.SchemeAPIKey{
				APIKey: apiKey,
			},
		}),
	}

	serverURL := os.Getenv("SPEAKEASY_SERVER_URL")
	if serverURL != "" {
		opts = append(opts, sdk.WithServerURL(serverURL, nil))
	}

	s := sdk.New(opts...)

	return s, nil
}
