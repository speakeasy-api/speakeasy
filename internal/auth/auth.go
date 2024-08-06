package auth

import (
	"context"
	"fmt"

	core "github.com/speakeasy-api/speakeasy-core/auth"
	"github.com/speakeasy-api/speakeasy/internal/config"
	"github.com/speakeasy-api/speakeasy/internal/log"
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
