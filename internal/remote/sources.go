package remote

import (
	"context"
	"fmt"
	"time"

	"github.com/samber/lo"
	core "github.com/speakeasy-api/speakeasy-core/auth"
	"github.com/speakeasy-api/speakeasy/internal/sdk"

	speakeasyclientsdkgo "github.com/speakeasy-api/speakeasy-client-sdk-go/v3"
	"github.com/speakeasy-api/speakeasy-client-sdk-go/v3/pkg/models/operations"
	"github.com/speakeasy-api/speakeasy-client-sdk-go/v3/pkg/models/shared"
)

// RecentGeneration represents a recent generation of a target in the workspace.
// The source of this data is our CLI event stream, which is updated every time
// a target is generated.
type RecentGeneration struct {
	CreatedAt       time.Time
	ID              string
	TargetName      string
	Target          string
	SourceNamespace string
	Success         bool
	Published       bool
	RegistryUri     string

	// May not be set
	GitRepoOrg *string
	GitRepo    *string

	// gen.yaml
	GenerateConfig *string
}

const (
	// The event stream contains multiple events for the same namespace, so we want to
	// break execution once we've seen a minimum, arbitrary number of unique namespaces.
	recentGenerationsToShow int = 5
)

// GetRecentWorkspaceGenerations returns the most recent generations of targets in a workspace
// This is based on the CLi event stream, which is updated on every CLI interaction.
func GetRecentWorkspaceGenerations(ctx context.Context) ([]RecentGeneration, error) {
	workspaceId, err := core.GetWorkspaceIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	speakeasyClient, err := sdk.InitSDK()
	if err != nil {
		return nil, err
	}

	// The event stream is limited to the most recent 250 events
	res, err := speakeasyClient.Events.Search(ctx, operations.SearchWorkspaceEventsRequest{
		WorkspaceID:     &workspaceId,
		InteractionType: shared.InteractionTypeTargetGenerate.ToPointer(),
	})

	if err != nil {
		return nil, err
	}

	if len(res.CliEventBatch) == 0 {
		return nil, fmt.Errorf("no events found for workspace %s", workspaceId)
	}

	seenUniqueNamespaces := make(map[string]bool)

	var generations []RecentGeneration

	for _, event := range res.CliEventBatch {
		// Filter out cli events that aren't generation based, or lack the required
		// fields.
		if !isRelevantGenerationEvent(event) {
			continue
		}

		if seenUniqueNamespaces[*event.SourceNamespaceName] {
			continue
		}

		if !hasMainRevision(ctx, speakeasyClient, *event.SourceNamespaceName) {
			continue
		}

		seenUniqueNamespaces[*event.SourceNamespaceName] = true

		registryUri, err := GetRegistryUriForSource(ctx, *event.SourceNamespaceName)
		if err != nil {
			return nil, err
		}

		generations = append(generations, RecentGeneration{
			ID:              event.ID,
			CreatedAt:       event.CreatedAt,
			TargetName:      *event.GenerateTargetName,
			Target:          *event.GenerateTarget,
			GitRepoOrg:      event.GitRemoteDefaultOwner,
			GitRepo:         event.GitRemoteDefaultRepo,
			SourceNamespace: *event.SourceNamespaceName,
			GenerateConfig:  event.GenerateConfigPreRaw,
			RegistryUri:     registryUri,
			Success:         event.Success,
		})

		if len(seenUniqueNamespaces) >= recentGenerationsToShow {
			break
		}
	}

	return generations, nil
}

func isRelevantGenerationEvent(event shared.CliEvent) bool {
	if event.SourceRevisionDigest == nil {
		return false
	}
	if event.GenerateTarget == nil {
		return false
	}
	if event.GenerateTargetName == nil {
		return false
	}
	if event.SourceNamespaceName == nil {
		return false
	}

	return true
}

const (
	mainRevisionTag = "main"
)

func hasMainRevision(ctx context.Context, client *speakeasyclientsdkgo.Speakeasy, namespace string) bool {
	revisions, err := client.Artifacts.GetRevisions(ctx, operations.GetRevisionsRequest{
		NamespaceName: namespace,
	})

	if err != nil {
		return false
	}

	if len(revisions.GetRevisionsResponse.GetItems()) == 0 {
		return false
	}

	foundMainTag := false

	for _, revision := range revisions.GetRevisionsResponse.GetItems() {
		if lo.Contains(revision.GetTags(), mainRevisionTag) {
			foundMainTag = true
			break
		}
	}

	return foundMainTag
}

func GetRegistryUriForSource(ctx context.Context, sourceNamespace string) (string, error) {
	orgSlug := core.GetOrgSlugFromContext(ctx)
	workspaceSlug := core.GetWorkspaceSlugFromContext(ctx)

	if orgSlug == "" || workspaceSlug == "" {
		return "", fmt.Errorf("could not generate registry uri: missing organization or workspace slug")
	}

	return fmt.Sprintf("registry.speakeasyapi.dev/%s/%s/%s:main", orgSlug, workspaceSlug, sourceNamespace), nil
}
