package registry

import (
	"context"
	"fmt"
	"strings"

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

	if isMismatchedWorkspaceError(err) {
		return fmt.Errorf("The current workspace does not match the original workspace for this registry entry. Ensure you are in the correct workspace.")
	}
	return err
}

func isMismatchedWorkspaceError(err error) bool {
	message := err.Error()

	if strings.Contains(message, "resolving for reference") {
		return true
	}

	return false
}
