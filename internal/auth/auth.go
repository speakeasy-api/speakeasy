package auth

import (
	"context"
	"fmt"
	core "github.com/speakeasy-api/speakeasy-core/auth"
	"github.com/speakeasy-api/speakeasy/internal/config"
	"github.com/speakeasy-api/speakeasy/internal/log"
)

func Authenticate(ctx context.Context, force bool, existingKey string) (context.Context, error) {
	if existingKey != "" {
		existingKey = config.GetSpeakeasyAPIKey()
	}
	authCtx, res, err := core.Authenticate(ctx, existingKey, force)
	if err != nil {
		return authCtx, err
	}
	if err := config.SetSpeakeasyAuthInfo(res); err != nil {
		return authCtx, fmt.Errorf("failed to save API key: %w", err)
	}

	return authCtx, nil
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
