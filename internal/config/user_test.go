package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"tmeet/internal/core/keychain"
)

// setupTestEnv 设置测试环境：临时目录 + mock keychain + 清空缓存。
// 返回 mock 实例和清理函数，调用方需 defer cleanup()。
func setupTestEnv(t *testing.T) (*keychain.MockKeychain, func()) {
	t.Helper()

	tmpDir := t.TempDir()
	t.Setenv("TMEET_CLI_CONFIG_DIR", tmpDir)

	mock := keychain.NewMockKeychain()
	SetKeychain(mock)
	ResetCache()

	cleanup := func() {
		SetKeychain(nil)
		ResetCache()
	}

	return mock, cleanup
}

func TestSaveAndGetUserConfig(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	cfg := &UserConfig{
		SdkId:        "sdk-test-001",
		OpenId:       "open_001",
		AccessToken:  "access-token-xyz",
		RefreshToken: "refresh-token-xyz",
		Expires:      1700000000,
	}

	if err := SaveUserConfig(cfg); err != nil {
		t.Fatalf("SaveUserConfig() error: %v", err)
	}

	ResetCache()

	got, err := GetUserConfig()
	if err != nil {
		t.Fatalf("GetUserConfig() error: %v", err)
	}
	if got == nil {
		t.Fatal("GetUserConfig() returned nil")
	}
	if got.SdkId != cfg.SdkId {
		t.Errorf("SdkId = %q, want %q", got.SdkId, cfg.SdkId)
	}
	if got.AccessToken != cfg.AccessToken {
		t.Errorf("AccessToken = %q, want %q", got.AccessToken, cfg.AccessToken)
	}
	if got.RefreshToken != cfg.RefreshToken {
		t.Errorf("RefreshToken = %q, want %q", got.RefreshToken, cfg.RefreshToken)
	}
	if got.Expires != cfg.Expires {
		t.Errorf("Expires = %d, want %d", got.Expires, cfg.Expires)
	}
}

func TestGetUserConfigNotFound(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	got, err := GetUserConfig()
	if err != nil {
		t.Fatalf("GetUserConfig() error: %v", err)
	}
	if got != nil {
		t.Fatalf("GetUserConfig() = %+v, want nil", got)
	}
}

func TestClearUserConfig(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	cfg := &UserConfig{SdkId: "sdk-clear-test", OpenId: "open_clear_test"}
	if err := SaveUserConfig(cfg); err != nil {
		t.Fatalf("SaveUserConfig() error: %v", err)
	}

	if err := ClearUserConfig(); err != nil {
		t.Fatalf("ClearUserConfig() error: %v", err)
	}

	ResetCache()

	got, err := GetUserConfig()
	if err != nil {
		t.Fatalf("GetUserConfig() after clear error: %v", err)
	}
	if got != nil {
		t.Fatalf("GetUserConfig() after clear = %+v, want nil", got)
	}
}

func TestMultiAppSwitch(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	cfgA := &UserConfig{SdkId: "sdk-app-a", OpenId: "open_app_a", AccessToken: "token-a"}
	cfgB := &UserConfig{SdkId: "sdk-app-b", OpenId: "open_app_b", AccessToken: "token-b"}

	if err := SaveUserConfig(cfgA); err != nil {
		t.Fatalf("SaveUserConfig(A) error: %v", err)
	}
	if err := SaveUserConfig(cfgB); err != nil {
		t.Fatalf("SaveUserConfig(B) error: %v", err)
	}

	ResetCache()

	got, err := GetUserConfig()
	if err != nil {
		t.Fatalf("GetUserConfig() error: %v", err)
	}
	if got == nil {
		t.Fatal("GetUserConfig() returned nil, expected app B")
	}
	if got.OpenId != "open_app_b" {
		t.Errorf("active OpenId = %q, want %q", got.OpenId, "open_app_b")
	}
	if got.AccessToken != "token-b" {
		t.Errorf("active AccessToken = %q, want %q", got.AccessToken, "token-b")
	}
}

func TestSaveUserConfigEmptyOpenId(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	cfg := &UserConfig{SdkId: "sdk-test", OpenId: ""}
	err := SaveUserConfig(cfg)
	if err == nil {
		t.Fatal("SaveUserConfig() should fail with empty OpenId")
	}
}

func TestSaveUserConfigNil(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	err := SaveUserConfig(nil)
	if err == nil {
		t.Fatal("SaveUserConfig(nil) should fail")
	}
}

func TestSaveUserConfigReservedOpenId(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	cfg := &UserConfig{SdkId: "sdk-test", OpenId: keychain.MasterKeyAccount}
	err := SaveUserConfig(cfg)
	if err == nil {
		t.Fatal("SaveUserConfig() should reject OpenId that conflicts with master key account name")
	}
}

func TestGetUserConfigCorruptedData(t *testing.T) {
	mock, cleanup := setupTestEnv(t)
	defer cleanup()

	// 写入非法 JSON 到 mock keychain
	_ = mock.Set(keychain.ServiceName, "open_corrupt", "not-valid-json{{{")
	_ = saveMeta(&AppMeta{ActiveOpenId: "open_corrupt"})

	ResetCache()

	_, err := GetUserConfig()
	if err == nil {
		t.Fatal("GetUserConfig() should fail on corrupted data")
	}
}

func TestMultiAppSwitchBack(t *testing.T) {
	mock, cleanup := setupTestEnv(t)
	defer cleanup()
	_ = mock

	cfgA := &UserConfig{SdkId: "sdk-app-a", OpenId: "open_app_a", AccessToken: "token-a"}
	cfgB := &UserConfig{SdkId: "sdk-app-b", OpenId: "open_app_b", AccessToken: "token-b"}

	_ = SaveUserConfig(cfgA)
	_ = SaveUserConfig(cfgB)

	_ = saveMeta(&AppMeta{ActiveOpenId: "open_app_a"})
	ResetCache()

	got, err := GetUserConfig()
	if err != nil {
		t.Fatalf("GetUserConfig() error: %v", err)
	}
	if got == nil || got.OpenId != "open_app_a" {
		t.Fatalf("after switch back, OpenId = %v, want open_app_a", got)
	}
	if got.AccessToken != "token-a" {
		t.Errorf("after switch back, AccessToken = %q, want token-a", got.AccessToken)
	}
}

func TestClearUserConfigFilesRemoved(t *testing.T) {
	mock, cleanup := setupTestEnv(t)
	defer cleanup()

	cfg := &UserConfig{SdkId: "sdk-clear-verify", OpenId: "open_clear_verify"}
	_ = SaveUserConfig(cfg)

	if _, err := mock.Get(keychain.ServiceName, "open_clear_verify"); err != nil {
		t.Fatalf("data should exist before clear: %v", err)
	}

	_ = ClearUserConfig()

	if _, err := mock.Get(keychain.ServiceName, "open_clear_verify"); err != keychain.ErrNotFound {
		t.Errorf("keychain data should be removed after clear, got err=%v", err)
	}

	if _, err := os.Stat(GetConfigPath()); !os.IsNotExist(err) {
		t.Errorf("config.json should be removed after clear, stat err=%v", err)
	}
}

func TestClearUserConfigIdempotent(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	if err := ClearUserConfig(); err != nil {
		t.Fatalf("first clear (empty state) error: %v", err)
	}
	if err := ClearUserConfig(); err != nil {
		t.Fatalf("second clear error: %v", err)
	}
}

func TestValidateOpenId(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	cases := []struct {
		name    string
		openId  string
		wantErr bool
	}{
		{"normal", "open_123_abc", false},
		{"underscore", "open_v2", false},
		{"numeric", "12345678", false},
		{"alpha-only", "abcXYZ", false},
		{"empty", "", true},
		{"reserved-master-key", keychain.MasterKeyAccount, true},
		{"with-dot", "open.v2", true},
		{"with-hyphen", "open-123", true},
		{"with-slash", "open/evil", true},
		{"with-backslash", "open\\evil", true},
		{"path-traversal", "../../../tmp/evil", true},
		{"double-dot", "..", true},
		{"has-space", "open id", true},
		{"chinese", "\u4e2d\u6587", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &UserConfig{SdkId: "sdk-test", OpenId: tc.openId}
			err := SaveUserConfig(cfg)
			if (err != nil) != tc.wantErr {
				t.Errorf("OpenId=%q: err=%v, wantErr=%v", tc.openId, err, tc.wantErr)
			}
		})
	}
}

func TestMultiAppFullLifecycle(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	cfgA := &UserConfig{SdkId: "app-a", OpenId: "open_a", AccessToken: "ta"}
	cfgB := &UserConfig{SdkId: "app-b", OpenId: "open_b", AccessToken: "tb"}

	_ = SaveUserConfig(cfgA)
	_ = SaveUserConfig(cfgB)

	// Clear active (B)
	_ = ClearUserConfig()
	ResetCache()

	// Switch back to A — should still have A's data
	_ = saveMeta(&AppMeta{ActiveOpenId: "open_a"})
	ResetCache()

	got, err := GetUserConfig()
	if err != nil || got == nil {
		t.Fatalf("app-a should still be readable after clearing app-b: err=%v, got=%v", err, got)
	}
	if got.AccessToken != "ta" {
		t.Errorf("app-a data mismatch: %+v", got)
	}
}

func TestLoadMetaCorruptedJSON(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	configPath := GetConfigPath()
	_ = os.MkdirAll(filepath.Dir(configPath), 0700)
	_ = os.WriteFile(configPath, []byte("{invalid json!!!"), 0600)

	ResetCache()
	_, err := GetUserConfig()
	if err == nil {
		t.Fatal("GetUserConfig should fail on corrupted config.json")
	}
}

func TestSaveUserConfigReservedMasterKey(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	cfg := &UserConfig{SdkId: "sdk-test", OpenId: keychain.MasterKeyAccount}
	err := SaveUserConfig(cfg)
	if err == nil {
		t.Fatal("SaveUserConfig() should reject OpenId that conflicts with master key account name")
	}
}

func TestClearUserConfigWithInvalidMeta(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	_ = saveMeta(&AppMeta{ActiveOpenId: keychain.MasterKeyAccount})

	err := ClearUserConfig()
	if err != nil {
		t.Fatalf("ClearUserConfig should not error on invalid meta, got: %v", err)
	}

	ResetCache()
	got, _ := GetUserConfig()
	if got != nil {
		t.Fatal("GetUserConfig should return nil after clearing invalid meta")
	}
}

// --- 覆盖率补充 ---

func TestGetConfigDirEnvOverride(t *testing.T) {
	t.Setenv("TMEET_CLI_CONFIG_DIR", "/custom/config")
	got := GetConfigDir()
	if got != "/custom/config" {
		t.Errorf("GetConfigDir() = %q, want /custom/config", got)
	}
}

func TestGetConfigDirDefault(t *testing.T) {
	t.Setenv("TMEET_CLI_CONFIG_DIR", "")
	got := GetConfigDir()
	if got == "" {
		t.Fatal("GetConfigDir() should not return empty string")
	}
	home, _ := os.UserHomeDir()
	if home != "" {
		expected := filepath.Join(home, ".tmeet")
		if got != expected {
			t.Errorf("GetConfigDir() = %q, want %q", got, expected)
		}
	}
}

func TestGetConfigPath(t *testing.T) {
	t.Setenv("TMEET_CLI_CONFIG_DIR", "/test/dir")
	got := GetConfigPath()
	expected := filepath.Join("/test/dir", "config.json")
	if got != expected {
		t.Errorf("GetConfigPath() = %q, want %q", got, expected)
	}
}

func TestGetUserConfigCacheHit(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	cfg := &UserConfig{SdkId: "sdk-cache", OpenId: "open_cache"}
	_ = SaveUserConfig(cfg)

	got1, err := GetUserConfig()
	if err != nil {
		t.Fatalf("first GetUserConfig() error: %v", err)
	}

	got2, err := GetUserConfig()
	if err != nil {
		t.Fatalf("second GetUserConfig() error: %v", err)
	}
	if got1 != got2 {
		t.Fatal("GetUserConfig() should return cached pointer on second call")
	}
}

func TestGetUserConfigEmptyActiveOpenId(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	_ = saveMeta(&AppMeta{ActiveOpenId: ""})
	ResetCache()

	got, err := GetUserConfig()
	if err != nil {
		t.Fatalf("GetUserConfig() error: %v", err)
	}
	if got != nil {
		t.Fatalf("GetUserConfig() = %+v, want nil for empty active_open_id", got)
	}
}

func TestGetUserConfigInvalidActiveOpenId(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	_ = saveMeta(&AppMeta{ActiveOpenId: keychain.MasterKeyAccount})
	ResetCache()

	_, err := GetUserConfig()
	if err == nil {
		t.Fatal("GetUserConfig() should fail with invalid active_open_id in meta")
	}
}

func TestSaveUserConfigUpdatesCache(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	cfg1 := &UserConfig{SdkId: "sdk-v1", OpenId: "open_v1"}
	_ = SaveUserConfig(cfg1)

	cfg2 := &UserConfig{SdkId: "sdk-v1", OpenId: "open_v1"}
	_ = SaveUserConfig(cfg2)
}

func TestClearUserConfigNoMeta(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	// 直接清除，不先保存任何东西，loadMeta 返回 ErrNotExist
	err := ClearUserConfig()
	if err != nil {
		t.Fatalf("ClearUserConfig() should succeed when no meta exists: %v", err)
	}
}

func TestGetUserConfigNoFile(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	ResetCache()

	got, err := GetUserConfig()
	if err != nil {
		t.Fatalf("GetUserConfig() error: %v", err)
	}
	if got != nil {
		t.Fatalf("GetUserConfig() = %+v, want nil", got)
	}
}

func TestSaveMetaMkdirError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root bypasses permission checks")
	}

	tmpDir := t.TempDir()
	blocker := filepath.Join(tmpDir, "block")
	_ = os.WriteFile(blocker, []byte("x"), 0600)

	t.Setenv("TMEET_CLI_CONFIG_DIR", filepath.Join(blocker, "sub"))
	mock := keychain.NewMockKeychain()
	SetKeychain(mock)
	defer func() {
		SetKeychain(nil)
		ResetCache()
	}()

	cfg := &UserConfig{SdkId: "sdk-test", OpenId: "open_test"}
	err := SaveUserConfig(cfg)
	if err == nil {
		t.Fatal("SaveUserConfig should fail when config dir cannot be created")
	}
}

func TestSaveUserConfigOverwrite(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	cfg := &UserConfig{SdkId: "sdk-ow", OpenId: "open_ow", AccessToken: "t1"}
	_ = SaveUserConfig(cfg)

	cfg.AccessToken = "t2"
	_ = SaveUserConfig(cfg)

	ResetCache()

	got, err := GetUserConfig()
	if err != nil {
		t.Fatalf("GetUserConfig() error: %v", err)
	}
	if got.AccessToken != "t2" {
		t.Errorf("overwrite failed: got AccessToken=%q", got.AccessToken)
	}
}

func TestGetUserConfigKeychainNotFound(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	_ = saveMeta(&AppMeta{ActiveOpenId: "open_missing"})
	ResetCache()

	got, err := GetUserConfig()
	if err != nil {
		t.Fatalf("GetUserConfig() should not error when keychain item not found: %v", err)
	}
	if got != nil {
		t.Fatalf("GetUserConfig() = %+v, want nil", got)
	}
}

// --- keychain 错误传播测试 ---

type errorKeychain struct {
	getErr    error
	setErr    error
	removeErr error
}

func (e *errorKeychain) Get(_, _ string) (string, error) { return "", e.getErr }
func (e *errorKeychain) Set(_, _, _ string) error        { return e.setErr }
func (e *errorKeychain) Remove(_, _ string) error        { return e.removeErr }

func TestGetUserConfigKeychainError(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("TMEET_CLI_CONFIG_DIR", tmpDir)

	ek := &errorKeychain{getErr: fmt.Errorf("disk I/O error")}
	SetKeychain(ek)
	ResetCache()
	defer func() {
		SetKeychain(nil)
		ResetCache()
	}()

	_ = saveMeta(&AppMeta{ActiveOpenId: "open_err_test"})
	ResetCache()

	_, err := GetUserConfig()
	if err == nil {
		t.Fatal("GetUserConfig() should fail when keychain.Get returns non-ErrNotFound error")
	}
	if !strings.Contains(err.Error(), "failed to read encrypted config") {
		t.Errorf("error should wrap 'failed to read encrypted config', got: %v", err)
	}
}

func TestSaveUserConfigKeychainSetError(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("TMEET_CLI_CONFIG_DIR", tmpDir)

	ek := &errorKeychain{setErr: fmt.Errorf("registry write denied")}
	SetKeychain(ek)
	ResetCache()
	defer func() {
		SetKeychain(nil)
		ResetCache()
	}()

	cfg := &UserConfig{SdkId: "sdk-set-err", OpenId: "open_set_err"}
	err := SaveUserConfig(cfg)
	if err == nil {
		t.Fatal("SaveUserConfig() should fail when keychain.Set returns error")
	}
	if !strings.Contains(err.Error(), "failed to save encrypted config") {
		t.Errorf("error should wrap 'failed to save encrypted config', got: %v", err)
	}
}
