package sdk

import (
	"errors"
	"os"

	speakeasy "github.com/speakeasy-api/speakeasy-client-sdk-go/v3"
	"github.com/speakeasy-api/speakeasy-client-sdk-go/v3/pkg/models/shared"

	"github.com/speakeasy-api/speakeasy/internal/config"
)

func InitSDK(apiKey string) (*speakeasy.Speakeasy, error) {
	if apiKey == "" {
		apiKey = config.GetSpeakeasyAPIKey()
	}
	if apiKey == "" {
		return nil, errors.New("no api key available, please set SPEAKEASY_API_KEY or run 'speakeasy auth' to authenticate the CLI with the Speakeasy Platform")
	}

	opts := []speakeasy.SDKOption{
		speakeasy.WithSecurity(shared.Security{APIKey: &apiKey}),
	}

	serverURL := os.Getenv("SPEAKEASY_SERVER_URL")
	if serverURL != "" {
		opts = append(opts, speakeasy.WithServerURL(serverURL))
	}

	s := speakeasy.New(opts...)

	return s, nil
}
