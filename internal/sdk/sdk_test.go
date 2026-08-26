package sdk

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	speakeasy "github.com/speakeasy-api/speakeasy-client-sdk-go/v3"
	"github.com/speakeasy-api/speakeasy-client-sdk-go/v3/pkg/models/operations"
	"github.com/speakeasy-api/speakeasy-client-sdk-go/v3/pkg/models/shared"
	"github.com/stretchr/testify/require"
)

// Guards against the SDK silently dropping auth on registry operations.
// client-sdk-go v3.27.0 was generated from a spec that marked all Artifacts
// and Subscriptions operations `security: []`, so these calls went out with
// no x-api-key header and the platform returned 403s (CLI v1.795.2, reverted
// in #2120). Nothing fails at compile time when that happens — the only
// observable difference is the missing header, which this test pins down for
// the operations the CLI actually calls.
func TestInitSDKWithKey_SendsAPIKeyOnRegistryOperations(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	apiKeyByPath := map[string]string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		apiKeyByPath[r.URL.Path] = r.Header.Get("x-api-key")
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	s, err := InitSDKWithKey("test-api-key", speakeasy.WithServerURL(server.URL))
	require.NoError(t, err)

	ctx := context.Background()

	// registry/tagging.go (speakeasy tag promote/apply, ci tag)
	_, _ = s.Artifacts.PostTags(ctx, operations.PostTagsRequest{
		NamespaceName: "test-namespace",
		AddTags: &shared.AddTags{
			RevisionDigest: "sha256:0000000000000000000000000000000000000000000000000000000000000000",
			Tags:           []string{"main"},
		},
	})

	// internal/remote/sources.go hasMainRevision
	_, _ = s.Artifacts.GetRevisions(ctx, operations.GetRevisionsRequest{
		NamespaceName: "test-namespace",
	})

	// The response bodies above are not representative, so the calls may
	// return unmarshalling errors — all this test cares about is that the
	// requests carried the API key.
	mu.Lock()
	defer mu.Unlock()
	require.Len(t, apiKeyByPath, 2, "expected both operations to reach the server")
	for path, apiKey := range apiKeyByPath {
		require.Equalf(t, "test-api-key", apiKey, "request to %s was sent without the x-api-key header: the SDK dropped auth for this operation", path)
	}
}
