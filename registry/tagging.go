package registry

import (
	"context"

	"github.com/speakeasy-api/speakeasy-client-sdk-go/v3/pkg/models/operations"
	"github.com/speakeasy-api/speakeasy-client-sdk-go/v3/pkg/models/shared"
	core "github.com/speakeasy-api/speakeasy-core/auth"
)

func AddTags(ctx context.Context, namespaceName, revisionDigest string, tags []string) error {
	s, err := core.GetSDKFromContext(ctx)
	if err != nil {
		return err
	}

	_, err = s.Artifacts.PostTags(ctx, operations.PostTagsRequest{
		NamespaceName: namespaceName,
		AddTags: &shared.AddTags{
			RevisionDigest: revisionDigest,
			Tags:           tags,
		},
	})

	return err
}
