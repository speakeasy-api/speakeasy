package config

import (
	"context"
	"testing"

	core "github.com/speakeasy-api/speakeasy-core/auth"
	"github.com/spf13/viper"
)

func TestSetSpeakeasyAuthInfoOfflineLicensePersistence(t *testing.T) { //nolint:paralleltest
	originalCfg, originalDir := vCfg, cfgDir
	t.Cleanup(func() { vCfg, cfgDir = originalCfg, originalDir })
	cfgDir = t.TempDir()
	vCfg = viper.New()
	vCfg.SetConfigName("config")
	vCfg.SetConfigType("yaml")
	vCfg.AddConfigPath(cfgDir)

	vCfg.Set("speakeasy_workspace_id", "workspace-a")
	vCfg.Set("offline_license_token", "stored-token")

	info := core.SpeakeasyAuthInfo{APIKey: "api-key", WorkspaceID: "workspace-a"}

	if err := SetSpeakeasyAuthInfo(context.Background(), info); err != nil {
		t.Fatalf("set auth info without token: %v", err)
	}
	if got := GetOfflineLicenseToken(); got != "stored-token" {
		t.Fatalf("token-less authentication replaced the stored offline license: %q", got)
	}

	freshCtx := context.WithValue(context.Background(), core.LicenseTokenKey, []byte("fresh-token"))
	if err := SetSpeakeasyAuthInfo(freshCtx, info); err != nil {
		t.Fatalf("set auth info with fresh token: %v", err)
	}
	if got := GetOfflineLicenseToken(); got != "fresh-token" {
		t.Fatalf("fresh license token was not persisted: %q", got)
	}

	other := core.SpeakeasyAuthInfo{APIKey: "api-key", WorkspaceID: "workspace-b"}
	if err := SetSpeakeasyAuthInfo(context.Background(), other); err != nil {
		t.Fatalf("set auth info for other workspace: %v", err)
	}
	if got := GetOfflineLicenseToken(); got != "" {
		t.Fatalf("workspace change kept the previous workspace's offline license: %q", got)
	}
}

func TestSetSpeakeasyAuthInfoSelfWorkspaceIsNoOp(t *testing.T) { //nolint:paralleltest
	originalCfg, originalDir := vCfg, cfgDir
	t.Cleanup(func() { vCfg, cfgDir = originalCfg, originalDir })
	cfgDir = t.TempDir()
	vCfg = viper.New()
	vCfg.SetConfigName("config")
	vCfg.SetConfigType("yaml")
	vCfg.AddConfigPath(cfgDir)

	vCfg.Set("speakeasy_workspace_id", "self")
	vCfg.Set("speakeasy_customer_id", "customer-id")
	vCfg.Set("offline_license_token", "stored-token")

	info := core.SpeakeasyAuthInfo{APIKey: "api-key", WorkspaceID: "self"}
	if err := SetSpeakeasyAuthInfo(context.Background(), info); err != nil {
		t.Fatalf("set auth info for self: %v", err)
	}
	if got := vCfg.GetString("speakeasy_customer_id"); got != "customer-id" {
		t.Fatalf("self re-authentication blanked the stored customer id: %q", got)
	}
	if got := GetOfflineLicenseToken(); got != "stored-token" {
		t.Fatalf("self re-authentication changed the stored offline license: %q", got)
	}
}
