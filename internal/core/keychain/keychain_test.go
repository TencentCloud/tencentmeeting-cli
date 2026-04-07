package keychain

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestGetDataDirEnvOverride(t *testing.T) {
	t.Setenv("TMEET_CLI_DATA_DIR", "/custom/data/path")
	got := GetDataDir()
	if got != "/custom/data/path" {
		t.Errorf("GetDataDir() = %q, want /custom/data/path", got)
	}
}

func TestGetDataDirDefault(t *testing.T) {
	t.Setenv("TMEET_CLI_DATA_DIR", "")
	got := GetDataDir()
	if got == "" {
		t.Fatal("GetDataDir() should not return empty string")
	}

	home, _ := os.UserHomeDir()
	if home == "" {
		t.Skip("cannot determine home directory")
	}

	switch runtime.GOOS {
	case "darwin":
		expected := filepath.Join(home, "Library", "Application Support", ServiceName)
		if got != expected {
			t.Errorf("GetDataDir() = %q, want %q", got, expected)
		}
	case "linux":
		expected := filepath.Join(home, ".local", "share", ServiceName)
		if got != expected {
			t.Errorf("GetDataDir() = %q, want %q", got, expected)
		}
	case "windows":
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" {
			localAppData = filepath.Join(home, "AppData", "Local")
		}
		expected := filepath.Join(localAppData, ServiceName)
		if got != expected {
			t.Errorf("GetDataDir() = %q, want %q", got, expected)
		}
	}
}

func TestNewReturnsNonNil(t *testing.T) {
	kc := New()
	if kc == nil {
		t.Fatal("New() should return non-nil KeychainAccess")
	}
}
