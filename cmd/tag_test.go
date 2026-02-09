package cmd

import (
	"context"
	"testing"

	"github.com/speakeasy-api/sdk-gen-config/workflow"
	core "github.com/speakeasy-api/speakeasy-core/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// withAuthContext returns a context with the given org and workspace slugs set,
// as if the user were authenticated to that workspace.
func withAuthContext(ctx context.Context, orgSlug, workspaceSlug string) context.Context {
	ctx = context.WithValue(ctx, core.OrgSlugKey, orgSlug)
	ctx = context.WithValue(ctx, core.WorkspaceSlugKey, workspaceSlug)
	return ctx
}

func TestValidateRegistryWorkspace(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		location      string
		orgSlug       string
		workspaceSlug string
		expectError   bool
		errorContains []string
	}{
		{
			name:          "matching workspace - no error",
			location:      "registry.speakeasyapi.dev/org-a/ws-b/my-sdk",
			orgSlug:       "org-a",
			workspaceSlug: "ws-b",
			expectError:   false,
		},
		{
			name:          "org mismatch",
			location:      "registry.speakeasyapi.dev/org-a/ws-b/my-sdk",
			orgSlug:       "org-x",
			workspaceSlug: "ws-b",
			expectError:   true,
			errorContains: []string{"org-a/ws-b", "org-x/ws-b"},
		},
		{
			name:          "workspace mismatch",
			location:      "registry.speakeasyapi.dev/org-a/ws-b/my-sdk",
			orgSlug:       "org-a",
			workspaceSlug: "ws-y",
			expectError:   true,
			errorContains: []string{"org-a/ws-b", "org-a/ws-y"},
		},
		{
			name:          "both org and workspace mismatch",
			location:      "registry.speakeasyapi.dev/org-a/ws-b/my-sdk",
			orgSlug:       "org-x",
			workspaceSlug: "ws-y",
			expectError:   true,
			errorContains: []string{"org-a/ws-b", "org-x/ws-y"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := withAuthContext(context.Background(), tt.orgSlug, tt.workspaceSlug)
			reg := &workflow.SourceRegistry{
				Location: workflow.SourceRegistryLocation(tt.location),
			}

			err := validateRegistryWorkspace(ctx, reg)

			if tt.expectError {
				require.Error(t, err)
				for _, substr := range tt.errorContains {
					assert.Contains(t, err.Error(), substr)
				}
				return
			}

			require.NoError(t, err)
		})
	}
}
