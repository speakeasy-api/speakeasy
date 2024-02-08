package config

import (
	"os"
	"path/filepath"

	core "github.com/speakeasy-api/speakeasy-core/auth"

	"github.com/spf13/viper"
)

var (
	vCfg   = viper.New()
	cfgDir string
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

func SetSpeakeasyAuthInfo(info core.SpeakeasyAuthInfo) error {
	vCfg.Set("speakeasy_api_key", info.APIKey)
	vCfg.Set("speakeasy_workspace_id", info.WorkspaceID)
	vCfg.Set("speakeasy_customer_id", info.CustomerID)
	return save()
}

func ClearSpeakeasyAuthInfo() error {
	vCfg.Set("speakeasy_api_key", "")
	vCfg.Set("speakeasy_workspace_id", "")
	vCfg.Set("speakeasy_customer_id", "")
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
