package auth

import (
	"context"
	"fmt"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/styles"

	core "github.com/speakeasy-api/speakeasy-core/auth"
	"github.com/speakeasy-api/speakeasy/internal/config"
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
		WithStyle(styles.Success).
		Printf("Authenticated with workspace successfully - %s/workspaces/%s\n", core.GetServerURL(), res.WorkspaceID)

	return nil
}

func Logout(ctx context.Context) error {
	if err := config.ClearSpeakeasyAuthInfo(); err != nil {
		return fmt.Errorf("failed to remove API key: %w", err)
	}

	log.From(ctx).
		WithInteractiveOnly().
		WithStyle(styles.Success).
		Println("Logout successful!")

	return nil
}
