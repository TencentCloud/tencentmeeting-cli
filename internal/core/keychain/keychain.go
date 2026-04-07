// Package keychain provides cross-platform encrypted credential storage.
//
// Uses a layered encryption scheme of "Random Master Key + System Keychain + AES-256-GCM":
//   - Keychain only stores one 32-byte master key (written once, read-only afterwards)
//   - Business data is encrypted with the master key using AES-256-GCM and stored as local .enc files
//
// Different underlying implementations per platform, but a consistent Get/Set/Remove interface for callers:
//   - macOS: system Keychain (go-keyring) stores master key, AES-256-GCM encrypted data written to .enc files
//   - Linux/Unix: file stores master key (permission 0600), AES-256-GCM encrypted data written to .enc files
//   - Windows: DPAPI-encrypted master key stored in registry, AES-256-GCM encrypted data stored in registry
//
// Usage:
//
//	import "tmeet/internal/core/keychain"
//	kc := keychain.New()
//	_ = kc.Set("tmeet", "sdk123", `{"secret":"xxx","access_token":"xxx"}`)
//	data, _ := kc.Get("tmeet", "sdk123")
//	_ = kc.Remove("tmeet", "sdk123")
package keychain

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

const (
	// ServiceName is the service name identifier in Keychain, used to distinguish credentials of different applications.
	ServiceName = "tmeet"

	// MasterKeyAccount is the account identifier for the master key in Keychain.
	// The master key never changes after generation; token refreshes only update the .enc file without touching Keychain.
	MasterKeyAccount = "master.key"
)

// ErrNotFound indicates that the specified entry was not found in Keychain.
// Callers should check with errors.Is(err, ErrNotFound).
var ErrNotFound = errors.New("keychain: item not found")

// KeychainAccess is the unified cross-platform Keychain interface.
//
// Callers use Get/Set/Remove to store and retrieve encrypted data without worrying about platform differences or encryption details.
// Each (service, account) pair corresponds to an independent encrypted data entry.
type KeychainAccess interface {
	// Get reads the data for the specified service+account (internally auto-decrypts).
	// Returns ErrNotFound if the entry does not exist.
	Get(service, account string) (string, error)

	// Set writes the data for the specified service+account (internally auto-encrypts).
	// Overwrites if the entry already exists.
	Set(service, account, data string) error

	// Remove deletes the data for the specified service+account.
	// Returns ErrNotFound if the entry does not exist.
	Remove(service, account string) error
}

// New creates the KeychainAccess implementation for the current platform.
//
// Automatically selects based on the compilation target platform:
//   - macOS: darwinKeychain (system Keychain + .enc files)
//   - Linux/Unix: fileKeychain (file master key + .enc files)
//   - Windows: windowsKeychain (DPAPI + registry)
func New() KeychainAccess {
	return newPlatformKeychain()
}

// GetDataDir returns the platform-specific encrypted data storage directory (for .enc files and master key).
//
// Supports overriding via the TMEET_CLI_DATA_DIR environment variable for testing and custom deployments.
//
// Default paths:
//   - macOS:   ~/Library/Application Support/tmeet/
//   - Linux:   ~/.local/share/tmeet/
//   - Windows: %LOCALAPPDATA%\tmeet\
func GetDataDir() string {
	if dir := os.Getenv("TMEET_CLI_DATA_DIR"); dir != "" {
		return dir
	}

	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		fmt.Fprintf(os.Stderr, "warning: unable to determine home directory: %v, falling back to temp dir\n", err)
		home = os.TempDir()
	}

	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", ServiceName)
	case "windows":
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" {
			localAppData = filepath.Join(home, "AppData", "Local")
		}
		return filepath.Join(localAppData, ServiceName)
	default:
		return filepath.Join(home, ".local", "share", ServiceName)
	}
}
