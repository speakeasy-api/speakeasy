package remote

import (
	"context"
	"fmt"
	"time"

	core "github.com/speakeasy-api/speakeasy-core/auth"
	"github.com/speakeasy-api/speakeasy/internal/sdk"

	"github.com/speakeasy-api/speakeasy-client-sdk-go/v3/pkg/models/operations"
	"github.com/speakeasy-api/speakeasy-client-sdk-go/v3/pkg/models/shared"
)

// RecentGeneration represents a recent generation of a target in the workspace.
// The source of this data is our CLI event stream, which is updated every time
// a target is generated.
type RecentGeneration struct {
	CreatedAt            time.Time
	ID                   string
	TargetName           string
	Target               string
	SourceNamespace      string
	SourceRevisionDigest string
	Success              bool

	// May not be set
	GitRepo *string
}

const (
	// The event stream contains multiple events for the same namespace, so we want to
	// break execution once we've seen a minimum, arbitrary number of unique namespaces.
	minimumRecentGenerationsToShow int = 5
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
	res, err := speakeasyClient.Events.SearchWorkspaceEvents(ctx, operations.SearchWorkspaceEventsRequest{
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

		seenUniqueNamespaces[*event.SourceNamespaceName] = true

		generations = append(generations, RecentGeneration{
			ID:                   event.ID,
			CreatedAt:            event.CreatedAt,
			TargetName:           *event.GenerateTargetName,
			Target:               *event.GenerateTarget,
			GitRepo:              event.GenerateRepoURL,
			SourceNamespace:      *event.SourceNamespaceName,
			SourceRevisionDigest: *event.SourceRevisionDigest,
			Success:              event.Success,
		})

		if len(seenUniqueNamespaces) >= minimumRecentGenerationsToShow {
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

func GetRegistryUriForSource(ctx context.Context, sourceNamespace, sourceRevisionDigest string) (string, error) {
	orgSlug := core.GetOrgSlugFromContext(ctx)
	workspaceSlug := core.GetWorkspaceSlugFromContext(ctx)

	if orgSlug == "" || workspaceSlug == "" {
		return "", fmt.Errorf("could not generate registry uri: missing organization or workspace slug")
	}

	return fmt.Sprintf("registry.speakeasyapi.dev/%s/%s/%s@%s", orgSlug, workspaceSlug, sourceNamespace, sourceRevisionDigest), nil
}
