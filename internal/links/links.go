package links

import (
	"context"
	"github.com/speakeasy-api/speakeasy-client-sdk-go/v3/pkg/models/operations"
	"github.com/speakeasy-api/speakeasy-core/auth"
	"github.com/speakeasy-api/speakeasy/internal/log"
)

func Shorten(ctx context.Context, url string) string {
	client, err := auth.GetSDKFromContext(ctx)
	if err != nil {
		log.From(ctx).Warnf("Failed to shorten link: %s", err.Error())
		return url
	}

	res, err := client.ShortURLs.Create(ctx, operations.CreateRequestBody{URL: url})
	if err != nil || res == nil || res.ShortURL == nil {
		log.From(ctx).Warnf("Failed to shorten link: %s", err.Error())
		return url
	}

	return res.ShortURL.ShortURL
}
