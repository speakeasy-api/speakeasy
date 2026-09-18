package auth

import (
	"context"
	"slices"
	"testing"

	core "github.com/speakeasy-api/speakeasy-core/auth"
	"github.com/speakeasy-api/speakeasy/internal/license"
)

func TestAuthenticateForceIgnoresExistingAPIKey(t *testing.T) {
	t.Parallel()

	ctx := context.WithValue(context.Background(), licenseContextKey{}, &license.License{})
	ctx = context.WithValue(ctx, core.LicenseTokenKey, []byte("stale-license"))
	freshLicense := []byte("fresh-license")
	coreForce := true
	persisted := false

	authCtx, err := authenticate(
		ctx,
		"api-key",
		true,
		func(ctx context.Context, apiKey string, force bool) (context.Context, core.SpeakeasyAuthInfo, error) {
			if apiKey != "api-key" {
				t.Fatalf("API key = %q, want api-key", apiKey)
			}
			if licenseFromContext(ctx) != nil {
				t.Fatal("offline license reached core authentication")
			}
			if token, ok := core.GetLicenseTokenFromContext(ctx); ok || len(token) != 0 {
				t.Fatalf("stale license reached core authentication: %q", token)
			}
			coreForce = force
			return context.WithValue(ctx, core.LicenseTokenKey, freshLicense), core.SpeakeasyAuthInfo{
				APIKey:      apiKey,
				WorkspaceID: "workspace",
			}, nil
		},
		func(ctx context.Context, info core.SpeakeasyAuthInfo) error {
			persisted = true
			if info.APIKey != "api-key" || info.WorkspaceID != "workspace" {
				t.Fatalf("persisted auth info = %#v", info)
			}
			token, ok := core.GetLicenseTokenFromContext(ctx)
			if !ok || !slices.Equal(token, freshLicense) {
				t.Fatalf("persisted license = %q, want %q", token, freshLicense)
			}
			return nil
		},
	)
	if err != nil {
		t.Fatalf("authenticate: %v", err)
	}
	if !coreForce {
		t.Fatal("force did not ignore the existing API key for browser authentication")
	}
	if !persisted {
		t.Fatal("refreshed authentication was not persisted")
	}
	if token, ok := core.GetLicenseTokenFromContext(authCtx); !ok || !slices.Equal(token, freshLicense) {
		t.Fatalf("authentication context license = %q, want %q", token, freshLicense)
	}
}

func TestAuthenticateForceUsesBrowserWithoutAPIKey(t *testing.T) {
	t.Parallel()

	coreForce := false
	_, err := authenticate(
		context.Background(),
		"",
		true,
		func(ctx context.Context, _ string, force bool) (context.Context, core.SpeakeasyAuthInfo, error) {
			coreForce = force
			return ctx, core.SpeakeasyAuthInfo{}, nil
		},
		func(context.Context, core.SpeakeasyAuthInfo) error { return nil },
	)
	if err != nil {
		t.Fatalf("authenticate: %v", err)
	}
	if !coreForce {
		t.Fatal("force did not request browser authentication without an API key")
	}
}

func TestCommandContextFallsBackToPlatformWithoutOfflineLicense(t *testing.T) {
	t.Setenv("SPEAKEASY_LICENSE_TOKEN", "not-a-license")

	wantCtx := context.WithValue(context.Background(), core.WorkspaceIDKey, "online-workspace")
	called := false
	ctx, err := commandContext(context.Background(), func(_ context.Context, force bool) (context.Context, error) {
		called = true
		if force {
			t.Fatal("command context forced online re-authentication")
		}
		return wantCtx, nil
	})
	if err != nil {
		t.Fatalf("command context: %v", err)
	}
	if !called || ctx != wantCtx {
		t.Fatal("command context did not fall back to platform authentication")
	}
}
