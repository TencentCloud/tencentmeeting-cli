package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
