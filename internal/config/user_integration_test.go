package config

import (
	"os"
	"path/filepath"
	"testing"
)

// 使用真实 keychain.New()（非 Mock），配置与加密数据均落在独立临时目录，
// 验证 SaveUserConfig / GetUserConfig / ClearUserConfig 在 darwin / linux / windows 上端到端可用。
func TestIntegrationUserConfigRealKeychainRoundtrip(t *testing.T) {
	tmp := t.TempDir()
	cfgDir := filepath.Join(tmp, "cfg")
	dataDir := filepath.Join(tmp, "data")
	if err := os.MkdirAll(cfgDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		t.Fatal(err)
	}

	t.Setenv("TMEET_CLI_CONFIG_DIR", cfgDir)
	t.Setenv("TMEET_CLI_DATA_DIR", dataDir)

	SetKeychain(nil)
	ResetCache()
	t.Cleanup(func() {
		SetKeychain(nil)
		ResetCache()
	})

	sdkID := "sdk-platform-integration-001"
	openID := "open_platform_integration_001"
	cfg := &UserConfig{
		SdkId:        sdkID,
		OpenId:       openID,
		AccessToken:  "at-integration",
		RefreshToken: "rt-integration",
		Expires:      1234567890,
	}

	if err := SaveUserConfig(cfg); err != nil {
		t.Fatalf("SaveUserConfig: %v", err)
	}

	ResetCache()

	got, err := GetUserConfig()
	if err != nil {
		t.Fatalf("GetUserConfig: %v", err)
	}
	if got == nil {
		t.Fatal("GetUserConfig: nil config")
	}
	if got.SdkId != cfg.SdkId || got.AccessToken != cfg.AccessToken {
		t.Fatalf("GetUserConfig = %+v, want %+v", got, cfg)
	}

	if err := ClearUserConfig(); err != nil {
		t.Fatalf("ClearUserConfig: %v", err)
	}

	ResetCache()

	after, err := GetUserConfig()
	if err != nil {
		t.Fatalf("GetUserConfig after clear: %v", err)
	}
	if after != nil {
		t.Fatalf("after ClearUserConfig, want nil, got %+v", after)
	}
}
