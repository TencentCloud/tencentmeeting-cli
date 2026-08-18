package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// UserConfig holds the user configuration, stored encrypted as a whole.
//
// All fields (including secret, access_token, refresh_token) are encrypted with AES-256-GCM
// and written to the <open_id>.enc file; plaintext is never persisted to disk.
// Use GetUserConfig / SaveUserConfig / ClearUserConfig for read/write;
// do not directly manipulate the underlying encrypted file.
type UserConfig struct {
	SdkId               string `json:"sdk_id,omitempty"`                // Application ID
	OpenId              string `json:"open_id,omitempty"`               // User openId, unique identifier for the user (unique per OAuth application per user)
	AccessToken         string `json:"access_token,omitempty"`          // User accessToken (encrypted storage, plaintext never persisted)
	RefreshToken        string `json:"refresh_token,omitempty"`         // User refreshToken (encrypted storage, plaintext never persisted)
	Expires             int64  `json:"expires,omitempty"`               // access_token expiry time, Unix timestamp (seconds)
	RefreshTokenExpires int64  `json:"refresh_token_expires,omitempty"` // refresh_token expiry time, Unix timestamp (seconds)
}

// AppMeta holds application metadata, stored in plaintext in config.json.
//
// Contains only non-sensitive information, used to locate the encrypted file (<open_id>.enc) for the current active user.
// Supports multi-application scenarios: each application has its own .enc file, identified by ActiveOpenId.
type AppMeta struct {
	ActiveOpenId string `json:"active_open_id"` // OpenId of the currently active user
}

// userConfig is the in-memory cached user configuration to avoid repeated decryption.
var userConfig *UserConfig

// GetConfigDir returns the configuration file directory path.
//
// Supports overriding via the TMEET_CLI_CONFIG_DIR environment variable for testing and custom deployments.
// Default path: ~/.tmeet/
func GetConfigDir() string {
	if dir := os.Getenv("TMEET_CLI_CONFIG_DIR"); dir != "" {
		return dir
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		fmt.Fprintf(os.Stderr, "warning: unable to determine home directory: %v, falling back to temp dir\n", err)
		home = os.TempDir()
	}
	return filepath.Join(home, ".tmeet")
}

// GetConfigPath 返回 config.json 的完整路径。
func GetConfigPath() string {
	return filepath.Join(GetConfigDir(), "config.json")
}

// GetTokenLockPath 返回 token.lock 的完整路径。
func GetTokenLockPath() string {
	return filepath.Join(GetConfigDir(), "token.lock")
}

// loadMeta 从 config.json 读取应用元数据。
// 文件不存在或读取失败时返回 nil 和错误。
func loadMeta() (*AppMeta, error) {
	data, err := os.ReadFile(GetConfigPath())
	if err != nil {
		return nil, err
	}
	meta := &AppMeta{}
	if err = json.Unmarshal(data, meta); err != nil {
		return nil, fmt.Errorf("failed to parse app metadata: %w", err)
	}
	return meta, nil
}

// saveMeta 保存应用元数据到 config.json，使用原子性写入。
//
// 写入策略：先写 .tmp 临时文件 → Sync 刷盘 → os.Rename 原子替换。
func saveMeta(meta *AppMeta) error {
	configDir := GetConfigDir()
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize app metadata: %w", err)
	}

	configPath := GetConfigPath()
	tmpFile, err := os.CreateTemp(configDir, "."+filepath.Base(configPath)+"-*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	defer func() { _ = os.Remove(tmpPath) }()

	if _, err = tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("failed to write temp file: %w", err)
	}
	if err = tmpFile.Sync(); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("failed to sync temp file: %w", err)
	}
	if err = tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	if err = os.Rename(tmpPath, configPath); err != nil {
		return fmt.Errorf("failed to save app metadata: %w", err)
	}

	return nil
}

// clearMeta 删除 config.json 文件。
// 文件不存在时不报错（幂等操作）。
func clearMeta() error {
	configPath := GetConfigPath()
	if err := os.Remove(configPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("failed to delete config file: %w", err)
	}
	return nil
}
