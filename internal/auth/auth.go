package auth

import (
	"context"
	"fmt"
	core "github.com/speakeasy-api/speakeasy-core/auth"
	"github.com/speakeasy-api/speakeasy/internal/config"
	"github.com/speakeasy-api/speakeasy/internal/log"
)

func Authenticate(ctx context.Context, force bool) error {
	existingKey, preferExisting := config.GetSpeakeasyAPIKey()
	res, err := core.Authenticate(existingKey, preferExisting, force)
	if err != nil {
		return err
	}
	if err := config.SetSpeakeasyAuthInfo(res); err != nil {
		return fmt.Errorf("failed to save API key: %w", err)
	}

	log.From(ctx).
		WithInteractiveOnly().
		Successf("Authenticated with workspace successfully - %s/workspaces/%s\n", core.GetServerURL(), res.WorkspaceID)

	return nil
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
