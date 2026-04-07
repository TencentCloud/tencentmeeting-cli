package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"sync"

	"tmeet/internal/core/keychain"
	"tmeet/internal/exception"
)

// kc is the Keychain access instance, supports injection for testing.
var (
	kc   keychain.KeychainAccess
	kcMu sync.Mutex
)

// SetKeychain injects a custom Keychain implementation.
//
// When not called, GetUserConfig and other functions will automatically use the default platform implementation (keychain.New()).
// Primary use: inject keychain.NewMockKeychain() in unit tests to avoid real system calls.
//
// Usage:
//
//	config.SetKeychain(keychain.NewMockKeychain())
func SetKeychain(k keychain.KeychainAccess) {
	kcMu.Lock()
	defer kcMu.Unlock()
	kc = k
}

// getKeychain returns the Keychain instance (lazy initialization, thread-safe).
func getKeychain() keychain.KeychainAccess {
	kcMu.Lock()
	defer kcMu.Unlock()
	if kc == nil {
		kc = keychain.New()
	}
	return kc
}

// Only letters, digits, and underscores are allowed.
var openIdPattern = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)

// validateOpenId validates the OpenId format (whitelist mode) to prevent path traversal and filesystem anomalies.
// Also rejects values that conflict with keychain internal reserved names to avoid overwriting the master key.
func validateOpenId(openId string) error {
	if openId == "" {
		return fmt.Errorf("OpenId cannot be empty")
	}
	if !openIdPattern.MatchString(openId) {
		return fmt.Errorf("invalid OpenId format (only letters, digits, underscores allowed): %q", openId)
	}
	if openId == keychain.MasterKeyAccount {
		return fmt.Errorf("OpenId cannot use reserved name: %q", openId)
	}
	return nil
}

// GetUserConfig retrieves the user configuration for the currently active application.
//
// Internally decrypts and reads from the encrypted .enc file; callers do not need to know the encryption details.
// Returns (nil, nil) if not configured (config.json does not exist) or not logged in (.enc does not exist).
// Results are cached in memory; multiple calls will not repeat decryption.
//
// Note: userConfig cache is not concurrency-safe; this is fine for CLI single-threaded scenarios.
// If concurrent use is needed, additional locking is required to protect userConfig.
func GetUserConfig() (*UserConfig, error) {
	if userConfig != nil {
		return userConfig, nil
	}

	meta, err := loadMeta()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, exception.GetUserConfigUnknownError.With("failed to load app metadata: %v", err)
	}
	if meta.ActiveOpenId == "" {
		return nil, nil
	}

	if err = validateOpenId(meta.ActiveOpenId); err != nil {
		return nil, exception.GetUserConfigUnknownError.With("invalid OpenId in app metadata: %v", err)
	}

	data, err := getKeychain().Get(keychain.ServiceName, meta.ActiveOpenId)
	if err != nil {
		if errors.Is(err, keychain.ErrNotFound) {
			return nil, nil
		}
		return nil, exception.GetUserConfigUnknownError.With("failed to read encrypted config: %v", err)
	}
	if data == "" {
		return nil, nil
	}

	cfg := &UserConfig{}
	if err = json.Unmarshal([]byte(data), cfg); err != nil {
		return nil, exception.ParseUserConfigError.With("failed to parse user config: %v", err)
	}

	userConfig = cfg
	return userConfig, nil
}

// SaveUserConfig saves the user configuration (encrypted write to .enc file).
//
// Internally serializes UserConfig as JSON and stores it encrypted via keychain.
// Also updates active_open_id in config.json to support multi-user switching.
// cfg.OpenId must not be empty, otherwise an error is returned.
func SaveUserConfig(config *UserConfig) error {
	if config == nil {
		return exception.InvalidArgsError.With("config cannot be nil")
	}
	if err := validateOpenId(config.OpenId); err != nil {
		return exception.InvalidArgsError.With("invalid OpenId: %v", err)
	}

	data, err := json.Marshal(config)
	if err != nil {
		return exception.InitializeFailedError.With("failed to serialize user config: %v", err)
	}

	if err = getKeychain().Set(keychain.ServiceName, config.OpenId, string(data)); err != nil {
		return exception.InitializeFailedError.With("failed to save encrypted config: %v", err)
	}

	if err = saveMeta(&AppMeta{ActiveOpenId: config.OpenId}); err != nil {
		return exception.InitializeFailedError.With("failed to update app metadata (encrypted data saved, please retry): %v", err)
	}

	userConfig = config
	return nil
}

// ClearUserConfig clears the configuration for the currently active user.
//
// Deletes the corresponding .enc encrypted file and active_open_id from config.json.
// The master key is not deleted (for use by other users).
func ClearUserConfig() (retErr error) {
	defer func() {
		userConfig = nil
		if err := clearMeta(); err != nil && retErr == nil {
			retErr = exception.LogoutFailedError.With("failed to clear app metadata: %v", err)
		}
	}()

	meta, err := loadMeta()
	if err != nil || meta == nil || meta.ActiveOpenId == "" {
		return nil
	}

	if err = validateOpenId(meta.ActiveOpenId); err != nil {
		return nil
	}

	if err = getKeychain().Remove(keychain.ServiceName, meta.ActiveOpenId); err != nil && !errors.Is(err, keychain.ErrNotFound) {
		return exception.LogoutFailedError.With("failed to delete encrypted config: %v", err)
	}
	return nil
}

// ResetCache 清除内存中的配置缓存，下次 GetUserConfig 会重新从加密文件读取。
// 主要用于测试场景或需要强制刷新配置时。
func ResetCache() {
	userConfig = nil
}
