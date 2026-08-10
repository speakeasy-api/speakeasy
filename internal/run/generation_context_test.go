package run

import (
	"context"
	"testing"

	generationaccess "github.com/speakeasy-api/generation-context/access"
)

func TestWithWorkflowGenerationContextPreservesExistingState(t *testing.T) {
	t.Parallel()

	original := generationaccess.WithDirect(context.Background())

	ctx, err := withWorkflowGenerationContext(original)
	if err != nil {
		t.Fatalf("with workflow generation context: %v", err)
	}
	if ctx != original {
		t.Fatal("existing state must be returned unchanged")
	}
	state, ok := generationaccess.StateFromContext(ctx)
	if !ok || state.Mode() != generationaccess.ModeDirect {
		t.Fatalf("existing direct state was not preserved: %#v", state)
	}
}

func TestWithWorkflowGenerationContextUnauthenticatedIsDirect(t *testing.T) {
	t.Parallel()

	ctx, err := withWorkflowGenerationContext(context.Background())
	if err != nil {
		t.Fatalf("with workflow generation context: %v", err)
	}
	state, ok := generationaccess.StateFromContext(ctx)
	if !ok {
		t.Fatal("expected direct state for unauthenticated invocation")
	}
	if state.Mode() != generationaccess.ModeDirect {
		t.Fatalf("unexpected mode %d", state.Mode())
	}
	if state.GeneratedLicense() != generationaccess.GeneratedLicenseAGPL {
		t.Fatalf("direct generation must be AGPL, got %q", state.GeneratedLicense())
	}
}
