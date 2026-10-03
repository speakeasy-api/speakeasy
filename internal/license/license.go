package license

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	speakeasy "github.com/speakeasy-api/speakeasy-client-sdk-go/v3"
	"github.com/speakeasy-api/speakeasy-client-sdk-go/v3/pkg/models/shared"
	core "github.com/speakeasy-api/speakeasy-core/auth"
	"github.com/speakeasy-api/speakeasy/internal/log"

	"github.com/speakeasy-api/openapi-generation/v2/pkg/licensetoken"
)

const (
	licenseTokenEnvironment = "SPEAKEASY_LICENSE_TOKEN"
	licenseFileEnvironment  = "SPEAKEASY_LICENSE_FILE"
)

type License struct {
	Token []byte
	Info  licensetoken.TokenInfo
	// Source is the environment variable the token was resolved from; empty
	// when it came from the CLI config.
	Source string
}

var inspect = licensetoken.Inspect

// Resolve returns the first configured license source when it validates for the workspace, otherwise nil and a warning naming the source.
func Resolve(getenv func(string) string, configToken string, workspaceID string) (*License, string) {
	resolve := func(token []byte, source string) (*License, string) {
		token = []byte(strings.TrimSpace(string(token)))
		info, err := inspect(token)
		if err == nil && usable(info, workspaceID) {
			return &License{Token: token, Info: info, Source: source}, ""
		}
		if source == "" {
			return nil, ""
		}
		return nil, "Ignoring unusable " + source + "; falling back to platform authentication"
	}

	if token := strings.TrimSpace(getenv(licenseTokenEnvironment)); token != "" {
		return resolve([]byte(token), licenseTokenEnvironment)
	}
	if path := strings.TrimSpace(getenv(licenseFileEnvironment)); path != "" {
		token, err := os.ReadFile(path)
		if err != nil {
			return nil, "Ignoring unreadable " + licenseFileEnvironment + "; falling back to platform authentication"
		}
		return resolve(token, licenseFileEnvironment)
	}
	if token := strings.TrimSpace(configToken); token != "" {
		return resolve([]byte(token), "")
	}
	return nil, ""
}

func usable(info licensetoken.TokenInfo, workspaceID string) bool {
	return workspaceID == "" || info.WorkspaceID == workspaceID
}

type payload struct {
	License licenseClaims `json:"license"`
}

type licenseClaims struct {
	OrgID              string    `json:"org_id"`
	Features           []string  `json:"features"`
	AddOns             []string  `json:"add_ons"`
	TelemetryDisabled  bool      `json:"telemetry_disabled"`
	WorkspaceCreatedAt time.Time `json:"workspace_created_at"`
}

func decodeClaims(token []byte) (licenseClaims, error) {
	parts := strings.Split(string(token), ".")
	if len(parts) != 3 {
		return licenseClaims{}, fmt.Errorf("invalid license token payload")
	}
	payloadJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return licenseClaims{}, fmt.Errorf("decode license token payload: %w", err)
	}
	var decoded payload
	if err := json.Unmarshal(payloadJSON, &decoded); err != nil {
		return licenseClaims{}, fmt.Errorf("decode license token claims: %w", err)
	}
	return decoded.License, nil
}

func ContextFromLicense(ctx context.Context, lic *License, apiKey string) (context.Context, error) {
	claims, err := decodeClaims(lic.Token)
	if err != nil {
		return ctx, err
	}

	accountType, err := accountTypeFromTier(lic.Info.Tier)
	if err != nil {
		return ctx, err
	}
	featureFlags := make([]string, 0, len(claims.Features))
	for _, feature := range claims.Features {
		candidate := shared.WorkspaceFeatureFlag(feature)
		if candidate.IsExact() {
			featureFlags = append(featureFlags, string(candidate))
		}
	}
	addOns := make([]shared.BillingAddOn, 0, len(claims.AddOns))
	for _, addOn := range claims.AddOns {
		candidate := shared.BillingAddOn(addOn)
		if candidate.IsExact() {
			addOns = append(addOns, candidate)
		}
	}

	if apiKey != "" {
		security := shared.Security{APIKey: &apiKey}
		// The SDK's default HTTP client keeps its built-in timeout, matching
		// the client core auth stores after an online authentication.
		sdk := speakeasy.New(
			speakeasy.WithSecurity(security),
			speakeasy.WithServerURL(core.GetServerURL()),
			speakeasy.WithWorkspaceID(lic.Info.WorkspaceID),
		)
		ctx = context.WithValue(ctx, core.SpeakeasySDKKey, sdk)
	}
	ctx = context.WithValue(ctx, core.WorkspaceIDKey, lic.Info.WorkspaceID)
	ctx = context.WithValue(ctx, core.AccountTypeKey, accountType)
	ctx = context.WithValue(ctx, core.WorkspaceFeatureFlagsKey, featureFlags)
	ctx = context.WithValue(ctx, core.OrgSlugKey, lic.Info.OrgSlug)
	ctx = context.WithValue(ctx, core.WorkspaceSlugKey, lic.Info.WorkspaceSlug)
	ctx = context.WithValue(ctx, core.WorkspaceCreatedAtKey, claims.WorkspaceCreatedAt)
	ctx = context.WithValue(ctx, core.TelemetryDisabledSlug, claims.TelemetryDisabled || apiKey == "")
	ctx = context.WithValue(ctx, core.BillingAddOnsKey, addOns)
	ctx = context.WithValue(ctx, core.LicenseTokenKey, append([]byte(nil), lic.Token...))

	log.From(ctx).Infof("Using the offline license for %s (expires %s); skipping platform authentication", lic.Info.WorkspaceSlug, lic.Info.ExpiresAt.Format(time.DateOnly))
	return ctx, nil
}

func accountTypeFromTier(tier string) (shared.AccountType, error) {
	if tier == "oss" {
		return shared.AccountTypeEnterprise, nil
	}
	candidate := shared.AccountType(tier)
	if !candidate.IsExact() || candidate == shared.AccountTypeOss {
		return "", fmt.Errorf("unsupported license tier %q", tier)
	}
	return candidate, nil
}
