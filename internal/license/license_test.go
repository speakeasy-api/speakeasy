package license

import (
	"context"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/speakeasy-api/openapi-generation/v2/pkg/licensetoken"
	"github.com/speakeasy-api/speakeasy-client-sdk-go/v3/pkg/models/shared"
	core "github.com/speakeasy-api/speakeasy-core/auth"
	"github.com/speakeasy-api/speakeasy/registry"
)

func TestResolveSourcePrecedence(t *testing.T) { //nolint:paralleltest
	originalInspect := inspect
	t.Cleanup(func() { inspect = originalInspect })
	inspect = func(token []byte) (licensetoken.TokenInfo, error) {
		if string(token) == "invalid-token" {
			return licensetoken.TokenInfo{}, errors.New("invalid")
		}
		return licensetoken.TokenInfo{WorkspaceID: "workspace", Targets: []string{"*"}, Message: string(token)}, nil
	}

	licenseFile := filepath.Join(t.TempDir(), "license.jwt")
	if err := os.WriteFile(licenseFile, []byte("file-token"), 0o600); err != nil {
		t.Fatalf("write license file: %v", err)
	}
	invalidLicenseFile := filepath.Join(t.TempDir(), "invalid-license.jwt")
	if err := os.WriteFile(invalidLicenseFile, []byte("invalid-token"), 0o600); err != nil {
		t.Fatalf("write invalid license file: %v", err)
	}

	tests := []struct {
		name        string
		environment map[string]string
		configToken string
		wantToken   string
		wantWarning string
	}{
		{name: "environment", environment: map[string]string{licenseTokenEnvironment: "environment-token", licenseFileEnvironment: filepath.Join(t.TempDir(), "missing.jwt")}, configToken: "config-token", wantToken: "environment-token"},
		{name: "invalid environment overrides file and config", environment: map[string]string{licenseTokenEnvironment: "invalid-token", licenseFileEnvironment: licenseFile}, configToken: "config-token", wantWarning: licenseTokenEnvironment},
		{name: "file", environment: map[string]string{licenseFileEnvironment: licenseFile}, configToken: "config-token", wantToken: "file-token"},
		{name: "invalid file overrides config", environment: map[string]string{licenseFileEnvironment: invalidLicenseFile}, configToken: "config-token", wantWarning: licenseFileEnvironment},
		{name: "config", environment: map[string]string{}, configToken: "config-token", wantToken: "config-token"},
	}
	for _, tt := range tests { //nolint:paralleltest
		t.Run(tt.name, func(t *testing.T) {
			lic, warning := Resolve(func(key string) string { return tt.environment[key] }, tt.configToken, "workspace")
			token := ""
			if lic != nil {
				token = string(lic.Token)
			}
			if token != tt.wantToken {
				t.Fatalf("license token = %q, want %q", token, tt.wantToken)
			}
			if tt.wantWarning != "" && !strings.Contains(warning, tt.wantWarning) {
				t.Fatalf("warning = %q, want source %q", warning, tt.wantWarning)
			}
			if tt.wantWarning == "" && warning != "" {
				t.Fatalf("unexpected warning = %q", warning)
			}
		})
	}
}

func TestResolveSkipsUnusableCandidates(t *testing.T) { //nolint:paralleltest
	originalInspect := inspect
	t.Cleanup(func() { inspect = originalInspect })

	tests := []struct {
		name       string
		info       licensetoken.TokenInfo
		inspectErr error
	}{
		{name: "other workspace", info: licensetoken.TokenInfo{WorkspaceID: "other", Targets: []string{"*"}}},
		{name: "expired", inspectErr: errors.New("expired")},
		{name: "invalid", inspectErr: errors.New("bad signature")},
	}
	for _, tt := range tests { //nolint:paralleltest
		t.Run(tt.name, func(t *testing.T) {
			inspect = func([]byte) (licensetoken.TokenInfo, error) { return tt.info, tt.inspectErr }
			lic, warning := Resolve(func(key string) string {
				if key == licenseTokenEnvironment {
					return "secret-token"
				}
				return ""
			}, "", "workspace")
			if lic != nil || warning == "" {
				t.Fatalf("resolution = %#v, warning = %q", lic, warning)
			}
			if strings.Contains(warning, "secret-token") {
				t.Fatal("warning contains the license token")
			}
		})
	}
}

func TestResolveUnreadableFileOverridesConfig(t *testing.T) { //nolint:paralleltest
	originalInspect := inspect
	t.Cleanup(func() { inspect = originalInspect })
	inspect = func(token []byte) (licensetoken.TokenInfo, error) {
		return licensetoken.TokenInfo{WorkspaceID: "workspace", Message: string(token)}, nil
	}

	lic, warning := Resolve(func(key string) string {
		if key == licenseFileEnvironment {
			return filepath.Join(t.TempDir(), "missing.jwt")
		}
		return ""
	}, "config-token", "workspace")
	if lic != nil || !strings.Contains(warning, licenseFileEnvironment) {
		t.Fatalf("resolution = %#v, warning = %q", lic, warning)
	}
}

func TestResolveSilentlyIgnoresUnusableConfigToken(t *testing.T) { //nolint:paralleltest
	originalInspect := inspect
	t.Cleanup(func() { inspect = originalInspect })
	inspect = func([]byte) (licensetoken.TokenInfo, error) {
		return licensetoken.TokenInfo{}, errors.New("invalid")
	}

	lic, warning := Resolve(func(string) string { return "" }, "invalid-config-token", "workspace")
	if warning != "" {
		t.Fatalf("unexpected warning = %q", warning)
	}
	if lic != nil {
		t.Fatalf("resolution = %#v, want nil", lic)
	}
}

func TestContextFromLicense(t *testing.T) {
	t.Parallel()

	createdAt := time.Date(2024, time.March, 2, 1, 2, 3, 0, time.UTC)
	token := testToken(`{"license":{"org_id":"org","features":["schema_registry","unknown"],"add_ons":["sdk_testing","unknown"],"telemetry_disabled":false,"workspace_created_at":"2024-03-02T01:02:03Z"}}`)
	lic := &License{
		Token: token,
		Info: licensetoken.TokenInfo{
			WorkspaceID:   "workspace",
			WorkspaceSlug: "workspace-slug",
			OrgSlug:       "org-slug",
			Tier:          "oss",
			ExpiresAt:     time.Date(2027, time.January, 1, 0, 0, 0, 0, time.UTC),
		},
	}

	ctx, err := ContextFromLicense(context.Background(), lic, "")
	if err != nil {
		t.Fatalf("context from license: %v", err)
	}
	workspaceID, err := core.GetWorkspaceIDFromContext(ctx)
	if err != nil || workspaceID != "workspace" {
		t.Fatalf("workspace ID = %q, %v", workspaceID, err)
	}
	accountType := core.GetAccountTypeFromContext(ctx)
	if accountType == nil || *accountType != shared.AccountTypeEnterprise {
		t.Fatalf("account type = %v, want enterprise", accountType)
	}
	if enabled, err := core.HasWorkspaceFeatureFlag(ctx, string(shared.WorkspaceFeatureFlagSchemaRegistry)); err != nil || !enabled {
		t.Fatalf("schema registry feature = %t, %v", enabled, err)
	}
	if enabled, err := core.HasWorkspaceFeatureFlag(ctx, "unknown"); err != nil || enabled {
		t.Fatalf("unknown feature = %t, %v", enabled, err)
	}
	if !core.IsTelemetryDisabled(ctx) {
		t.Fatal("telemetry is enabled without an API key")
	}
	if registry.IsRegistryEnabled(ctx) {
		t.Fatal("registry is enabled without an API key")
	}
	if got := core.GetWorkspaceCreatedAtFromContext(ctx); got == nil || !got.Equal(createdAt) {
		t.Fatalf("workspace created at = %v, want %v", got, createdAt)
	}
	tokenFromContext, ok := core.GetLicenseTokenFromContext(ctx)
	if !ok || !slices.Equal(tokenFromContext, token) {
		t.Fatal("license token missing from context")
	}
	if core.GetOrgSlugFromContext(ctx) != "org-slug" || core.GetWorkspaceSlugFromContext(ctx) != "workspace-slug" {
		t.Fatalf("slugs = %q/%q", core.GetOrgSlugFromContext(ctx), core.GetWorkspaceSlugFromContext(ctx))
	}
	if enabled, err := core.HasBillingAddOn(ctx, shared.BillingAddOnSDKTesting); err != nil || !enabled {
		t.Fatalf("SDK testing add-on = %t, %v", enabled, err)
	}
	if _, err := core.GetSDKFromContext(ctx); err == nil {
		t.Fatal("SDK present without API key")
	}

	withSDK, err := ContextFromLicense(context.Background(), lic, "api-key")
	if err != nil {
		t.Fatalf("context with SDK: %v", err)
	}
	if _, err := core.GetSDKFromContext(withSDK); err != nil {
		t.Fatalf("SDK missing with API key: %v", err)
	}
	if core.IsTelemetryDisabled(withSDK) {
		t.Fatal("telemetry claim was overridden with an API key")
	}
}

func TestTokenInfoCoversTargets(t *testing.T) {
	t.Parallel()

	if !(licensetoken.TokenInfo{Targets: []string{"go"}}).Covers("go") {
		t.Fatal("exact target is not covered")
	}
	if !(licensetoken.TokenInfo{Targets: []string{"*"}}).Covers("typescript") {
		t.Fatal("wildcard target is not covered")
	}
}

func testToken(payload string) []byte {
	return []byte("header." + base64.RawURLEncoding.EncodeToString([]byte(payload)) + ".signature")
}
