package sdk

import (
	"errors"
	"os"

	sdk "github.com/speakeasy-api/speakeasy-client-sdk-go"
	"github.com/speakeasy-api/speakeasy-client-sdk-go/pkg/models/shared"
	"github.com/speakeasy-api/speakeasy/internal/config"
)

func InitSDK(apiKey string) (*sdk.SDK, error) {
	if apiKey == "" {
		apiKey, _ = config.GetSpeakeasyAPIKey()
	}
	if apiKey == "" {
		return nil, errors.New("no api key available, please set SPEAKEASY_API_KEY or run 'speakeasy auth' to authenticate the CLI with the Speakeasy Platform")
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
