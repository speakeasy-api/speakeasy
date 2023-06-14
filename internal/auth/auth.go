package auth

import (
	"fmt"
	core "github.com/speakeasy-api/speakeasy-core/auth"
	"github.com/speakeasy-api/speakeasy/internal/config"
)

func Authenticate(force bool) error {
	existingKey, preferExisting := config.GetSpeakeasyAPIKey()
	res, err := core.Authenticate(existingKey, preferExisting, force)
	if err != nil {
		return err
	}
	if err := config.SetSpeakeasyAuthInfo(res); err != nil {
		return fmt.Errorf("failed to save API key: %w", err)
	}

	fmt.Printf("Authenticated with workspace successfully - %s/workspaces/%s\n", core.GetServerURL(), res.WorkspaceID)

	return nil
}

func Logout() error {
	if err := config.ClearSpeakeasyAuthInfo(); err != nil {
		return fmt.Errorf("failed to remove API key: %w", err)
	}

	fmt.Println("Logout successful!")

	return nil
}
