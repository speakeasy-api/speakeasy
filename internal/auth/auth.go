package auth

import (
	"context"
	"fmt"
	"os"

	"github.com/speakeasy-api/speakeasy-client-sdk-go/v3/pkg/models/operations"
	core "github.com/speakeasy-api/speakeasy-core/auth"
	"github.com/speakeasy-api/speakeasy/internal/config"
	"github.com/speakeasy-api/speakeasy/internal/interactivity"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/sdk"
)

func Authenticate(ctx context.Context, force bool) (context.Context, error) {
	existingKey := config.GetSpeakeasyAPIKey()
	authCtx, res, err := core.Authenticate(ctx, existingKey, force)
	if err != nil {
		return authCtx, err
	}
	if err := config.SetSpeakeasyAuthInfo(authCtx, res); err != nil {
		return authCtx, fmt.Errorf("failed to save API key: %w", err)
	}

	return authCtx, nil
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
	config.SetSpeakeasyAuthInfo(ctx, core.SpeakeasyAuthInfo{
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
		return nil
	}

	client, err := sdk.InitSDK()
	if err != nil {
		return nil
	}

	wsReq := operations.GetWorkspaceRequest{
		WorkspaceID: &workspaceID,
	}

	wsRes, err := client.Workspaces.GetByID(ctx, wsReq)
	if err != nil {
		return nil
	}

	if wsRes.StatusCode != 200 || wsRes.Workspace == nil {
		return nil
	}

	orgReq := operations.GetOrganizationRequest{
		OrganizationID: wsRes.Workspace.OrganizationID,
	}

	orgRes, err := client.Organizations.Get(ctx, orgReq)
	if err != nil {
		return nil
	}

	if orgRes.StatusCode != 200 || orgRes.Organization == nil {
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
