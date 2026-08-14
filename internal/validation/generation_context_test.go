package validation

import (
	"context"
	"testing"
	"time"

	generationaccess "github.com/speakeasy-api/generation-context/access"
	"github.com/speakeasy-api/speakeasy-client-sdk-go/v3/pkg/models/shared"
	"github.com/stretchr/testify/require"
)

func TestWithValidationGenerationContext(t *testing.T) {
	t.Parallel()

	t.Run("adds direct access when caller state is absent", func(t *testing.T) {
		t.Parallel()

		ctx := withValidationGenerationContext(context.Background())

		state, ok := generationaccess.StateFromContext(ctx)
		require.True(t, ok)
		require.Equal(t, generationaccess.ModeDirect, state.Mode())
	})

	t.Run("preserves caller-supplied authenticated access", func(t *testing.T) {
		t.Parallel()

		createdAt := time.Date(2024, time.January, 2, 3, 4, 5, 0, time.UTC)
		original, err := generationaccess.WithAuthenticated(context.Background(), generationaccess.AuthenticatedInfo{
			AccountType:        shared.AccountTypeBusiness,
			WorkspaceID:        "validation-test-workspace",
			WorkspaceCreatedAt: &createdAt,
			GeneratedLicense:   generationaccess.GeneratedLicenseCommercial,
		})
		require.NoError(t, err)

		ctx := withValidationGenerationContext(original)

		state, ok := generationaccess.StateFromContext(ctx)
		require.True(t, ok)
		require.Equal(t, generationaccess.ModeAuthenticated, state.Mode())
		require.Equal(t, generationaccess.GeneratedLicenseCommercial, state.GeneratedLicense())
		accountType, ok := state.AccountType()
		require.True(t, ok)
		require.Equal(t, shared.AccountTypeBusiness, accountType)
		workspaceID, ok := state.WorkspaceID()
		require.True(t, ok)
		require.Equal(t, "validation-test-workspace", workspaceID)
		require.Equal(t, &createdAt, state.WorkspaceCreatedAt())
	})
}
