package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// setupAgentTestEnv redirects the config directory to a temp dir.
func setupAgentTestEnv(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	t.Setenv("TMEET_CLI_CONFIG_DIR", tmpDir)
	return tmpDir
}

func TestGetAgentConfigNotExist(t *testing.T) {
	setupAgentTestEnv(t)

	got, err := GetAgentConfig()
	if err != nil {
		t.Fatalf("GetAgentConfig() error: %v", err)
	}
	if got != nil {
		t.Fatalf("GetAgentConfig() = %+v, want nil for missing file", got)
	}
}

func TestSaveAndGetAgentConfig(t *testing.T) {
	setupAgentTestEnv(t)

	cfg := &AgentConfig{Agent: "Cursor", Model: "Claude 3.5 Sonnet"}
	if err := SaveAgentConfig(cfg); err != nil {
		t.Fatalf("SaveAgentConfig() error: %v", err)
	}

	got, err := GetAgentConfig()
	if err != nil {
		t.Fatalf("GetAgentConfig() error: %v", err)
	}
	if got == nil {
		t.Fatal("GetAgentConfig() returned nil")
	}
	if got.Agent != cfg.Agent {
		t.Errorf("Agent = %q, want %q", got.Agent, cfg.Agent)
	}
	if got.Model != cfg.Model {
		t.Errorf("Model = %q, want %q", got.Model, cfg.Model)
	}
}

func TestSaveAgentConfigNil(t *testing.T) {
	setupAgentTestEnv(t)

	if err := SaveAgentConfig(nil); err != nil {
		t.Fatalf("SaveAgentConfig(nil) should succeed, got %v", err)
	}

	got, err := GetAgentConfig()
	if err != nil {
		t.Fatalf("GetAgentConfig() error: %v", err)
	}
	if got == nil {
		t.Fatal("GetAgentConfig() returned nil after saving nil")
	}
	if got.Agent != "" || got.Model != "" {
		t.Errorf("expected empty AgentConfig, got %+v", got)
	}
}

func TestSaveAgentConfigOverwrite(t *testing.T) {
	setupAgentTestEnv(t)

	_ = SaveAgentConfig(&AgentConfig{Agent: "Cursor", Model: "GPT-4o"})
	_ = SaveAgentConfig(&AgentConfig{Agent: "Claude Desktop", Model: "Claude 3.5 Sonnet"})

	got, err := GetAgentConfig()
	if err != nil {
		t.Fatalf("GetAgentConfig() error: %v", err)
	}
	if got.Agent != "Claude Desktop" || got.Model != "Claude 3.5 Sonnet" {
		t.Errorf("overwrite failed, got %+v", got)
	}
}

func TestClearAgentConfig(t *testing.T) {
	setupAgentTestEnv(t)

	_ = SaveAgentConfig(&AgentConfig{Agent: "Cline"})

	if err := ClearAgentConfig(); err != nil {
		t.Fatalf("ClearAgentConfig() error: %v", err)
	}

	if _, err := os.Stat(GetAgentConfigPath()); !os.IsNotExist(err) {
		t.Errorf("agent.json should be removed, stat err=%v", err)
	}

	got, err := GetAgentConfig()
	if err != nil {
		t.Fatalf("GetAgentConfig() after clear error: %v", err)
	}
	if got != nil {
		t.Fatalf("GetAgentConfig() after clear = %+v, want nil", got)
	}
}

func TestClearAgentConfigIdempotent(t *testing.T) {
	setupAgentTestEnv(t)

	if err := ClearAgentConfig(); err != nil {
		t.Fatalf("first clear (missing file) should succeed, got %v", err)
	}
	if err := ClearAgentConfig(); err != nil {
		t.Fatalf("second clear should succeed, got %v", err)
	}
}

func TestGetAgentConfigCorruptedJSON(t *testing.T) {
	dir := setupAgentTestEnv(t)

	_ = os.MkdirAll(dir, 0700)
	if err := os.WriteFile(GetAgentConfigPath(), []byte("{invalid json!!!"), 0600); err != nil {
		t.Fatal(err)
	}

	_, err := GetAgentConfig()
	if err == nil {
		t.Fatal("GetAgentConfig() should fail on corrupted JSON")
	}
}

func TestGetAgentConfigPath(t *testing.T) {
	t.Setenv("TMEET_CLI_CONFIG_DIR", "/some/dir")
	got := GetAgentConfigPath()
	want := filepath.Join("/some/dir", "agent.json")
	if got != want {
		t.Errorf("GetAgentConfigPath() = %q, want %q", got, want)
	}
}

func TestSaveAgentConfigAtomicWriteContent(t *testing.T) {
	setupAgentTestEnv(t)

	cfg := &AgentConfig{Agent: "CodeBuddy", Model: "DeepSeek"}
	if err := SaveAgentConfig(cfg); err != nil {
		t.Fatalf("SaveAgentConfig() error: %v", err)
	}

	// Verify the on-disk payload is valid JSON matching the input.
	data, err := os.ReadFile(GetAgentConfigPath())
	if err != nil {
		t.Fatalf("read agent.json: %v", err)
	}
	var back AgentConfig
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatalf("agent.json is not valid JSON: %v (%s)", err, string(data))
	}
	if back.Agent != cfg.Agent || back.Model != cfg.Model {
		t.Errorf("on-disk content mismatch: got %+v, want %+v", back, cfg)
	}
}
