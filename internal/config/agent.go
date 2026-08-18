package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"tmeet/internal/core/keychain"
	"tmeet/internal/exception"
)

// AgentConfig holds metadata about the calling AI agent (e.g. Cursor, Claude Desktop, Cline)
// and the underlying LLM model. It is non-sensitive telemetry-style information and is stored
// in plaintext, separate from user credentials.
//
// Persistence: <config_dir>/agent.json (atomic write via .tmp + rename).
type AgentConfig struct {
	Agent string `json:"agent,omitempty"` // AI-Agent name (e.g. Cursor, Claude Desktop)
	Model string `json:"model,omitempty"` // LLM model name (e.g. Claude 3.5 Sonnet, GPT-4o)
}

// GetAgentConfigPath returns the full path to agent.json.
func GetAgentConfigPath() string {
	return filepath.Join(GetConfigDir(), "agent.json")
}

// GetAgentConfig reads AgentConfig from agent.json.
//
// Returns (nil, nil) when the file does not exist (not-configured is a normal state).
// Returns an error only for real I/O or parse failures.
func GetAgentConfig() (*AgentConfig, error) {
	data, err := os.ReadFile(GetAgentConfigPath())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read agent config: %w", err)
	}
	cfg := &AgentConfig{}
	if err = json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse agent config: %w", err)
	}
	return cfg, nil
}

// SaveAgentConfig writes AgentConfig to agent.json atomically.
//
// Write strategy: write to .tmp -> Sync -> os.Rename, mirroring saveMeta().
// A nil cfg is treated as an empty config (both fields empty).
func SaveAgentConfig(cfg *AgentConfig) error {
	if cfg == nil {
		cfg = &AgentConfig{}
	}

	configDir := GetConfigDir()
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize agent config: %w", err)
	}

	configPath := GetAgentConfigPath()
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
		return fmt.Errorf("failed to save agent config: %w", err)
	}

	return nil
}

// ClearAgentConfig removes agent.json. Missing file is treated as success (idempotent).
//
// Note: this is intentionally NOT called by ClearUserConfig (logout). AgentConfig describes
// the calling environment, which is independent of the user's identity lifecycle.
func ClearAgentConfig() error {
	if err := os.Remove(GetAgentConfigPath()); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("failed to delete agent config: %w", err)
	}
	return nil
}

type AgentAccountConfig struct {
	AgentOpenId         string `json:"agent_open_id,omitempty"`         // unique identifier of the agent (sub-account)
	MasterOpenId        string `json:"master_open_id,omitempty"`        // OpenId of the master account that created this agent
	SdkId               string `json:"sdk_id,omitempty"`                // SDK application ID associated with this agent
	AccessToken         string `json:"access_token,omitempty"`          // agent accessToken (encrypted at rest, never stored in plaintext)
	RefreshToken        string `json:"refresh_token,omitempty"`         // agent refreshToken (encrypted at rest, never stored in plaintext)
	Expires             int64  `json:"expires,omitempty"`               // access_token expiry, Unix timestamp (seconds)
	RefreshTokenExpires int64  `json:"refresh_token_expires,omitempty"` // refresh_token expiry, Unix timestamp (seconds)
	CreateTime          int64  `json:"create_time,omitempty"`           // agent creation time, Unix timestamp (seconds), issued by server
}

const agentAccountSuffix = ".agent"

var agentAccountConfig *AgentAccountConfig

func validateAgentOpenId(agentOpenId string) error {
	if agentOpenId == "" {
		return fmt.Errorf("agent OpenId cannot be empty")
	}
	if !openIdPattern.MatchString(agentOpenId) {
		return fmt.Errorf("invalid agent OpenId format (only letters, digits, underscores allowed): %q", agentOpenId)
	}
	if agentOpenId == keychain.MasterKeyAccount {
		return fmt.Errorf("agent OpenId cannot use reserved name: %q", agentOpenId)
	}
	return nil
}

func agentKey(agentOpenId string) string {
	return agentOpenId + agentAccountSuffix
}

// GetAgentAccountConfig reads the active agent (sub-account) config from the encrypted keychain.
// Returns (nil, nil) when no agent is configured (normal state).
func GetAgentAccountConfig() (*AgentAccountConfig, error) {
	if agentAccountConfig != nil {
		return agentAccountConfig, nil
	}

	meta, err := loadMeta()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, exception.GetUserConfigUnknownError.With("failed to load app metadata: %v", err)
	}
	if meta.ActiveAgentOpenId == "" {
		return nil, nil
	}

	if err = validateAgentOpenId(meta.ActiveAgentOpenId); err != nil {
		return nil, exception.GetUserConfigUnknownError.With("invalid agent OpenId in app metadata: %v", err)
	}

	data, err := getKeychain().Get(keychain.ServiceName, agentKey(meta.ActiveAgentOpenId))
	if err != nil {
		if errors.Is(err, keychain.ErrNotFound) {
			return nil, nil
		}
		return nil, exception.GetUserConfigUnknownError.With("failed to read encrypted agent config: %v", err)
	}
	if data == "" {
		return nil, nil
	}

	cfg := &AgentAccountConfig{}
	if err = json.Unmarshal([]byte(data), cfg); err != nil {
		return nil, exception.ParseUserConfigError.With("failed to parse agent config: %v", err)
	}

	agentAccountConfig = cfg
	return agentAccountConfig, nil
}

// SaveAgentAccountConfig persists the agent config to the encrypted keychain and
// updates the metadata pointer. The master account must be logged in beforehand.
func SaveAgentAccountConfig(cfg *AgentAccountConfig) error {
	if cfg == nil {
		return exception.InvalidArgsError.With("agent config cannot be nil")
	}
	if err := validateAgentOpenId(cfg.AgentOpenId); err != nil {
		return exception.InvalidArgsError.With("invalid agent OpenId: %v", err)
	}

	// Agent creation requires the master account to be logged in first.
	meta, err := loadMeta()
	if err != nil || meta == nil || meta.ActiveOpenId == "" {
		return exception.InvalidArgsError.With("cannot create agent before the main account is logged in")
	}
	if err = validateOpenId(meta.ActiveOpenId); err != nil {
		return exception.InvalidArgsError.With("cannot create agent: invalid main account: %v", err)
	}

	data, err := json.Marshal(cfg)
	if err != nil {
		return exception.InitializeFailedError.With("failed to serialize agent config: %v", err)
	}

	if err = getKeychain().Set(keychain.ServiceName, agentKey(cfg.AgentOpenId), string(data)); err != nil {
		return exception.InitializeFailedError.With("failed to save encrypted agent config: %v", err)
	}

	// Only update the agent pointer; keep the master account pointer unchanged.
	meta.ActiveAgentOpenId = cfg.AgentOpenId
	if err = saveMeta(meta); err != nil {
		return exception.InitializeFailedError.With("failed to update app metadata (encrypted data saved, please retry): %v", err)
	}

	agentAccountConfig = cfg
	return nil
}

// ClearAgentAccountConfig removes the active agent config from the keychain and
// clears the metadata pointer. Missing entries are treated as success (idempotent).
func ClearAgentAccountConfig() error {
	defer func() { agentAccountConfig = nil }()

	meta, err := loadMeta()
	if err != nil || meta == nil || meta.ActiveAgentOpenId == "" {
		return nil
	}
	if err = validateAgentOpenId(meta.ActiveAgentOpenId); err != nil {
		// Agent identifier in metadata is invalid: clear the pointer; no corresponding file to delete.
		meta.ActiveAgentOpenId = ""
		if err = saveMeta(meta); err != nil {
			return exception.LogoutFailedError.With("failed to clear agent metadata: %v", err)
		}
		return nil
	}

	if err = getKeychain().Remove(keychain.ServiceName, agentKey(meta.ActiveAgentOpenId)); err != nil &&
		!errors.Is(err, keychain.ErrNotFound) {
		return exception.LogoutFailedError.With("failed to delete encrypted agent config: %v", err)
	}

	meta.ActiveAgentOpenId = ""
	if err = saveMeta(meta); err != nil {
		return exception.LogoutFailedError.With("failed to clear agent metadata: %v", err)
	}
	return nil
}
