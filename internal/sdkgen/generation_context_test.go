package sdkgen

import (
	"context"
	"errors"
	"testing"
	"time"

	generationaccess "github.com/speakeasy-api/generation-context/access"
	"github.com/speakeasy-api/openapi-generation/v2/pkg/licensetoken"
	"github.com/speakeasy-api/speakeasy-client-sdk-go/v3/pkg/models/shared"
	"github.com/speakeasy-api/speakeasy-core/auth"
)

func TestWithGenerationContextAlwaysElectsCommercial(t *testing.T) {
	t.Parallel()

	createdAt := time.Date(2024, time.January, 2, 3, 4, 5, 0, time.UTC)
	testCases := []struct {
		name         string
		licenseToken []byte
		wantLicense  generationaccess.GeneratedLicense
		wantToken    bool
	}{
		{name: "license token elects commercial and attaches the token", licenseToken: []byte("license-token"), wantLicense: generationaccess.GeneratedLicenseCommercial, wantToken: true},
		{name: "no license token still elects commercial", wantLicense: generationaccess.GeneratedLicenseCommercial},
		{name: "empty license token still elects commercial", licenseToken: []byte{}, wantLicense: generationaccess.GeneratedLicenseCommercial},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			legacy := auth.WithAdminSkipLicenseCheck(
				context.Background(),
				"workspace-id",
				shared.AccountTypeBusiness,
				nil,
				"org",
				"workspace",
				createdAt,
				[]shared.BillingAddOn{shared.BillingAddOnSnippetAi},
			)

			ctx, err := withGenerationContext(legacy, tt.licenseToken)
			if err != nil {
				t.Fatalf("with generation context: %v", err)
			}
			state, ok := generationaccess.StateFromContext(ctx)
			if !ok {
				t.Fatal("expected shared authenticated state")
			}
			if state.Mode() != generationaccess.ModeAuthenticated {
				t.Fatalf("unexpected mode %d", state.Mode())
			}
			if state.GeneratedLicense() != tt.wantLicense {
				t.Fatalf("expected license %q, got %q", tt.wantLicense, state.GeneratedLicense())
			}
			if !state.HasBillingAddOn(shared.BillingAddOnSnippetAi) {
				t.Fatal("caller bridge lost authenticated billing add-ons")
			}
			_, verification, err := licensetoken.ResolveGenerationAccess(ctx)
			if tt.wantToken {
				if !errors.Is(err, licensetoken.ErrInvalidLicenseToken) {
					t.Fatalf("expected the attached fixture token to reach the validator and be rejected, got verification=%v err=%v", verification, err)
				}
			} else if err != nil || verification != nil {
				t.Fatalf("expected no license token attached, got verification=%v err=%v", verification, err)
			}
		})
	}
}

func TestWithGenerationContextPreservesCancellableContext(t *testing.T) {
	t.Parallel()

	createdAt := time.Now()
	legacy := auth.WithAdminSkipLicenseCheck(
		context.Background(), "workspace-id", shared.AccountTypeBusiness, nil, "org", "workspace", createdAt, nil,
	)
	cancellable, cancel := context.WithCancel(legacy)
	defer cancel()

	ctx, err := withGenerationContext(cancellable, []byte("license-token"))
	if err != nil {
		t.Fatalf("with generation context: %v", err)
	}
	state, ok := generationaccess.StateFromContext(ctx)
	if !ok || state.GeneratedLicense() != generationaccess.GeneratedLicenseCommercial {
		t.Fatalf("cancellable context lost generation state: %#v", state)
	}
	cancel()
	if ctx.Err() != context.Canceled {
		t.Fatalf("cancellable context lost cancellation: %v", ctx.Err())
	}
}

func TestWithGenerationContextRejectsIncompleteAuthentication(t *testing.T) {
	t.Parallel()

	original := auth.SetAccountTypeInContext(context.Background(), string(shared.AccountTypeBusiness))
	ctx, err := withGenerationContext(original, []byte("license-token"))
	if err == nil {
		t.Fatal("expected incomplete authentication error")
	}
	if ctx != original {
		t.Fatal("failure must return the original context")
	}
	if _, ok := generationaccess.StateFromContext(ctx); ok {
		t.Fatal("failure inserted partial shared state")
	}
}
