package auth

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/speakeasy-api/openapi-generation/v2/pkg/licensetoken"
	"github.com/speakeasy-api/speakeasy-client-sdk-go/v3/pkg/models/operations"
	"github.com/speakeasy-api/speakeasy-client-sdk-go/v3/pkg/models/shared"
	core "github.com/speakeasy-api/speakeasy-core/auth"
	"github.com/speakeasy-api/speakeasy/internal/config"
	"github.com/speakeasy-api/speakeasy/internal/env"
	"github.com/speakeasy-api/speakeasy/internal/interactivity"
	"github.com/speakeasy-api/speakeasy/internal/license"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/sdk"
	"github.com/speakeasy-api/speakeasy/internal/utils"
)

type licenseContextKey struct{}

const licenseHint = "For offline authentication, configure offline_license_token or set SPEAKEASY_LICENSE_TOKEN or SPEAKEASY_LICENSE_FILE"

type coreAuthenticateFunc func(context.Context, string, bool) (context.Context, core.SpeakeasyAuthInfo, error)
type persistAuthInfoFunc func(context.Context, core.SpeakeasyAuthInfo) error
type authenticateWithHintFunc func(context.Context, bool) (context.Context, error)

func Authenticate(ctx context.Context, force bool) (context.Context, error) {
	return authenticate(ctx, config.GetSpeakeasyAPIKey(), force, core.Authenticate, persistAuthInfo)
}

func authenticate(ctx context.Context, apiKey string, force bool, authenticateCore coreAuthenticateFunc, persist persistAuthInfoFunc) (context.Context, error) {
	ctx = context.WithValue(ctx, licenseContextKey{}, (*license.License)(nil))
	ctx = context.WithValue(ctx, core.LicenseTokenKey, []byte(nil))

	// force ignores the existing API key and opens the browser.
	authCtx, res, err := authenticateCore(ctx, apiKey, force)
	if err != nil {
		return authCtx, err
	}
	if err := persist(authCtx, res); err != nil {
		return authCtx, fmt.Errorf("failed to save API key: %w", err)
	}
	return authCtx, nil
}

func persistAuthInfo(ctx context.Context, info core.SpeakeasyAuthInfo) error {
	return config.SetSpeakeasyAuthInfo(persistableLicenseContext(ctx, info.WorkspaceID), info)
}

// CommandContext authenticates with the stored offline license when it is usable and with the platform otherwise.
// `speakeasy auth login` is the explicit way to bypass the offline license and refresh the persisted license online.
func CommandContext(ctx context.Context) (context.Context, error) {
	return commandContext(ctx, authenticateWithHint)
}

func commandContext(ctx context.Context, authenticateOnline authenticateWithHintFunc) (context.Context, error) {
	lic, warning := license.Resolve(os.Getenv, config.GetOfflineLicenseToken(), config.GetWorkspaceID())
	if warning != "" {
		log.From(ctx).Warn(warning)
	}
	// With no persisted workspace there is no proof a config-stored license
	// belongs to the configured API key's workspace; authenticate online once
	// to establish it. An env-supplied license is an explicit choice and is
	// honored (air-gapped environments cannot go online), with a warning that
	// the pairing is unverified.
	if lic != nil && config.GetSpeakeasyAPIKey() != "" && config.GetWorkspaceID() == "" {
		if lic.Source == "" {
			log.From(ctx).Warn("Ignoring the stored offline license: the configured API key's workspace is not known yet; authenticating online")
			lic = nil
		} else {
			log.From(ctx).Warn(fmt.Sprintf("Using %s for workspace %s; unable to verify it matches the configured API key's workspace", lic.Source, lic.Info.WorkspaceSlug))
		}
	}
	if lic != nil {
		licenseCtx, err := license.ContextFromLicense(ctx, lic, config.GetSpeakeasyAPIKey())
		if err == nil {
			return context.WithValue(licenseCtx, licenseContextKey{}, lic), nil
		}
		log.From(ctx).Warn("Could not use the stored offline license; falling back to platform authentication")
	}
	return authenticateOnline(ctx, false)
}

// EnsureTargets re-authenticates online when the offline license does not cover every target.
func EnsureTargets(ctx context.Context, targets []string) (context.Context, error) {
	lic := licenseFromContext(ctx)
	if lic == nil {
		return ctx, nil
	}
	for _, target := range targets {
		if !lic.Info.Covers(target) {
			return authenticateWithHint(ctx, false)
		}
	}
	return ctx, nil
}

// EnsurePlatform re-authenticates online when an offline-license context has no SDK client.
func EnsurePlatform(ctx context.Context) (context.Context, error) {
	if licenseFromContext(ctx) == nil {
		return ctx, nil
	}
	if _, err := core.GetSDKFromContext(ctx); err == nil {
		return ctx, nil
	}
	return authenticateWithHint(ctx, false)
}

func authenticateWithHint(ctx context.Context, force bool) (context.Context, error) {
	// Without an API key the only online path is a browser login, which would
	// hang a headless session; fail fast with the offline-license hint instead.
	if config.GetSpeakeasyAPIKey() == "" && !utils.IsInteractive() {
		return ctx, fmt.Errorf("authentication required but no API key is configured in a non-interactive session. %s", licenseHint)
	}
	authCtx, err := Authenticate(ctx, force)
	if err != nil && config.GetSpeakeasyAPIKey() == "" {
		return authCtx, fmt.Errorf("%w. %s", err, licenseHint)
	}
	return authCtx, err
}

func licenseFromContext(ctx context.Context) *license.License {
	lic, _ := ctx.Value(licenseContextKey{}).(*license.License)
	return lic
}

// HasOfflineLicense reports whether ctx was authenticated with the offline
// license rather than the platform.
func HasOfflineLicense(ctx context.Context) bool {
	return licenseFromContext(ctx) != nil
}

func persistableLicenseContext(ctx context.Context, workspaceID string) context.Context {
	persisted := []byte(nil)
	// A token persisted inside a GitHub Actions container would flip the same
	// job's later commands to offline auth and bypass the platform access
	// check; the container is ephemeral, so nothing is gained by storing it.
	if !env.IsGithubAction() {
		if token, ok := core.GetLicenseTokenFromContext(ctx); ok {
			info, err := licensetoken.Inspect(token)
			if err == nil && info.Tier != string(shared.AccountTypeFree) && info.WorkspaceID == workspaceID {
				persisted = token
			}
		}
	}
	return context.WithValue(ctx, core.LicenseTokenKey, persisted)
}

func UseExistingAPIKeyIfAvailable(ctx context.Context) (context.Context, error) {
	existingApiKey := config.GetSpeakeasyAPIKey()
	if existingApiKey == "" {
		return ctx, nil
	}
	ctx, err := core.NewContextWithSDK(ctx, existingApiKey)
	if err != nil {
		return ctx, err
	}
	workspaceID, err := core.GetWorkspaceIDFromContext(ctx)
	if err != nil {
		return ctx, err
	}
	_ = config.SetSpeakeasyAuthInfo(persistableLicenseContext(ctx, workspaceID), core.SpeakeasyAuthInfo{
		APIKey:      existingApiKey,
		WorkspaceID: workspaceID,
	})

	return ctx, nil
}

func Logout(ctx context.Context) error {
	if err := config.ClearSpeakeasyAuthInfo(); err != nil {
		return fmt.Errorf("failed to remove API key: %w", err)
	}

	log.From(ctx).
		WithInteractiveOnly().
		Success("Logout successful!")

	return nil
}

func ConfirmWorkspace(ctx context.Context) error {
	confirmEnv := os.Getenv("SPEAKEASY_CONFIRM_WORKSPACE")
	if confirmEnv == "" {
		return nil
	}

	workspaceID, err := core.GetWorkspaceIDFromContext(ctx)
	if err != nil {
		return nil //nolint:nilerr // Ignore error
	}

	client, err := sdk.InitSDK()
	if err != nil {
		return nil //nolint:nilerr // Ignore error
	}

	wsReq := operations.GetWorkspaceRequest{
		WorkspaceID: &workspaceID,
	}

	wsRes, err := client.Workspaces.GetByID(ctx, wsReq)
	if err != nil {
		return nil //nolint:nilerr // Ignore error
	}

	if wsRes.StatusCode != http.StatusOK || wsRes.Workspace == nil {
		return nil
	}

	orgReq := operations.GetOrganizationRequest{
		OrganizationID: wsRes.Workspace.OrganizationID,
	}

	orgRes, err := client.Organizations.Get(ctx, orgReq)
	if err != nil {
		return nil //nolint:nilerr // Ignore error
	}

	if orgRes.StatusCode != http.StatusOK || orgRes.Organization == nil {
		return nil
	}

	workspaceName := wsRes.Workspace.Name
	if workspaceName == "" {
		workspaceName = wsRes.Workspace.Slug
	}

	orgName := orgRes.Organization.Name
	if orgName == "" {
		orgName = orgRes.Organization.Slug
	}

	if workspaceID == "self" || (orgRes.Organization.Internal != nil && *orgRes.Organization.Internal) {
		log.From(ctx).Info(fmt.Sprintf("Running command in workspace: %s/%s", orgName, workspaceName))
		return nil
	}

	message := fmt.Sprintf("You are about to run this command in workspace: %s/%s", orgName, workspaceName)
	confirmed := interactivity.SimpleConfirm(message, false)

	if !confirmed {
		return fmt.Errorf("command cancelled by user")
	}

	return nil
}
