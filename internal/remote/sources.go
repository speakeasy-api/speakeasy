package remote

import (
	"context"
	"fmt"
	"sort"

	core "github.com/speakeasy-api/speakeasy-core/auth"

	"github.com/speakeasy-api/speakeasy-client-sdk-go/v3/pkg/models/operations"
	"github.com/speakeasy-api/speakeasy-client-sdk-go/v3/pkg/models/shared"
	"github.com/speakeasy-api/speakeasy/internal/sdk"
)

// FindRecentRemoteSources returns a list of recent remote sources
// based on events in the workspace.
func FindRecentRemoteNamespaces(ctx context.Context, limit int) ([]shared.Namespace, error) {
	speakeasyClient, err := sdk.InitSDK()

	if err != nil {
		return nil, err
	}

	res, err := speakeasyClient.Artifacts.GetNamespaces(ctx)

	if err != nil {
		return nil, err
	}

	items := res.GetNamespacesResponse.Items

	sort.Slice(items, func(i, j int) bool {
		return items[i].UpdatedAt.After(items[j].UpdatedAt)
	})

	var namespaces []shared.Namespace

	for i, item := range items {
		if i >= limit {
			break
		}
		namespaces = append(namespaces, item)
	}

	return namespaces, nil
}

func FetchRevisions(ctx context.Context, namespace string) ([]shared.Revision, error) {
	speakeasyClient, err := sdk.InitSDK()

	if err != nil {
		return nil, err
	}

	res, err := speakeasyClient.Artifacts.GetRevisions(ctx, operations.GetRevisionsRequest{
		NamespaceName: namespace,
	})

	if err != nil {
		return nil, err
	}

	if res.GetRevisionsResponse == nil {
		return nil, fmt.Errorf("no revisions found for namespace %s", namespace)
	}

	return res.GetRevisionsResponse.Items, nil
}

func GetRegistryUriForRevision(ctx context.Context, revision shared.Revision) (string, error) {
	orgSlug := core.GetOrgSlugFromContext(ctx)
	workspaceSlug := core.GetWorkspaceSlugFromContext(ctx)

	if orgSlug == "" || workspaceSlug == "" {
		return "", fmt.Errorf("could not generate registry uri: missing organization or workspace slug")
	}

	hasTags := len(revision.Tags) > 0

	// TODO: base should be configurable
	// TODO: should we be using new domain?
	// TODO: prefer latest?
	if hasTags {
		return fmt.Sprintf("registry.speakeasyapi.dev/%s/%s/%s:%s", orgSlug, workspaceSlug, revision.NamespaceName, revision.Tags[0]), nil
	}
	return fmt.Sprintf("registry.speakeasyapi.dev/%s/%s/%s@%s", orgSlug, workspaceSlug, revision.NamespaceName, revision.Digest), nil
}
