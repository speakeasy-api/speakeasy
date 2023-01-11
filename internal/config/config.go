package config

import (
	"os"
	"path"

	"github.com/spf13/viper"
)

var (
	vCfg   = viper.New()
	cfgDir string
)

type SpeakeasyAuthInfo struct {
	APIKey      string `json:"apiKey"`
	WorkspaceID string `json:"workspaceId"`
	CustomerID  string `json:"customerId"`
}

func Load() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	cfgDir = path.Join(home, ".speakeasy")

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

func GetSpeakeasyAPIKey() (string, bool) {
	apiKey := os.Getenv("SPEAKEASY_API_KEY")
	if apiKey == "" {
		return vCfg.GetString("speakeasy_api_key"), false
	}

	return apiKey, true
}

func GetWorkspaceID() string {
	return vCfg.GetString("speakeasy_workspace_id")
}

func SetSpeakeasyAuthInfo(info SpeakeasyAuthInfo) error {
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
	if err := os.MkdirAll(cfgDir, os.ModePerm); err != nil {
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
