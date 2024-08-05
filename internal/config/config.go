package config

import (
	"context"
	"fmt"
	"github.com/speakeasy-api/speakeasy/internal/charm/styles"
	"os"
	"path/filepath"

	core "github.com/speakeasy-api/speakeasy-core/auth"

	"github.com/spf13/viper"
)

var (
	vCfg   = viper.New()
	cfgDir string
)

const (
	workspaceKeysKey = "workspace_api_keys"
)

func Load() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	cfgDir = filepath.Join(home, ".speakeasy")

	vCfg.SetConfigName("config")
	vCfg.SetConfigType("yaml")
	vCfg.AddConfigPath(cfgDir)

	if err := vCfg.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return err
		}
	}

	return nil
}

func GetCustomerID() string {
	return vCfg.GetString("speakeasy_customer_id")
}

func GetSpeakeasyAPIKey() string {
	apiKey := os.Getenv("SPEAKEASY_API_KEY")
	if apiKey == "" {
		return vCfg.GetString("speakeasy_api_key")
	}

	return apiKey
}

func GetWorkspaceID() string {
	return vCfg.GetString("speakeasy_workspace_id")
}

func GetStudioSecret() string {
	return vCfg.GetString("speakeasy_studio_secret")
}

func GetWorkspaceAPIKey(orgSlug, workspaceSlug string) string {
	keys := vCfg.Sub(workspaceKeysKey)

	if keys != nil {
		return keys.GetString(getWorkspaceKey(orgSlug, workspaceSlug))
	}

	return ""
}

func SetWorkspaceAPIKey(orgSlug, workspaceSlug, key string) error {
	keys := map[string]interface{}{}

	keysVal := vCfg.Get(workspaceKeysKey)
	if keysVal != nil {
		keys = keysVal.(map[string]interface{})
	}

	keys[getWorkspaceKey(orgSlug, workspaceSlug)] = key

	vCfg.Set(workspaceKeysKey, keys)

	return save()
}

func getWorkspaceKey(orgSlug, workspaceSlug string) string {
	return fmt.Sprintf("%s@%s", orgSlug, workspaceSlug)
}

func SetStudioSecret(secret string) error {
	vCfg.Set("speakeasy_studio_secret", secret)
	return save()
}

func SetSpeakeasyAuthInfo(ctx context.Context, info core.SpeakeasyAuthInfo) error {
	// Keep speakeasy-self as default workspace
	if vCfg.GetString("speakeasy_workspace_id") != "self" {
		vCfg.Set("speakeasy_api_key", info.APIKey)
		vCfg.Set("speakeasy_workspace_id", info.WorkspaceID)
		vCfg.Set("speakeasy_customer_id", info.CustomerID)
	} else {
		println(styles.DimmedItalic.Render("Keeping speakeasy-self as default workspace. New workspace will still be usable as a registry source. Logout first if you want to change default workspaces\n"))
	}

	orgSlug := core.GetOrgSlugFromContext(ctx)
	workspaceSlug := core.GetWorkspaceSlugFromContext(ctx)

	// SetWorkspaceAPIKey executes save()
	return SetWorkspaceAPIKey(orgSlug, workspaceSlug, info.APIKey)
}

func ClearSpeakeasyAuthInfo() error {
	vCfg.Set("speakeasy_api_key", "")
	vCfg.Set("speakeasy_workspace_id", "")
	vCfg.Set("speakeasy_customer_id", "")
	vCfg.Set("speakeasy_studio_secret", "")
	return save()
}

func save() error {
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		return err
	}

	if err := vCfg.WriteConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return err
		}

		if err := vCfg.SafeWriteConfig(); err != nil {
			return err
		}
	}

	return nil
}
