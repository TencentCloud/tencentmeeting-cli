//go:build darwin

// keychain_darwin.go is the macOS platform Keychain implementation.
//
// The master key is stored in the system Keychain (via go-keyring calling Security.framework),
// base64-encoded. On first use, a 32-byte random master key is auto-generated and written to Keychain; subsequent accesses are read-only.
// Encrypted data is stored as .enc files after AES-256-GCM encryption.
//
// macOS system Keychain is bound to the current user's login credentials; copying .enc files to another device cannot decrypt them.
// The master key never changes after generation; token refreshes only update the .enc file without touching Keychain.
// This is the fundamental reason for minimizing Keychain interactions and authorization popups.

package keychain

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"sync"

	"github.com/zalando/go-keyring"
)

// darwinKeychain is the macOS platform implementation.
// The master key is cached in memory via sync.Once to avoid accessing the system Keychain on every operation (IPC ~1-5ms).
type darwinKeychain struct {
	cachedKey []byte
	keyErr    error
	once      sync.Once
}

func newPlatformKeychain() KeychainAccess {
	return &darwinKeychain{}
}

// getMasterKey retrieves the master key (with in-memory cache).
// On first call, loads from or generates into the system Keychain; subsequent calls return the cached value.
func (k *darwinKeychain) getMasterKey() ([]byte, error) {
	k.once.Do(func() {
		k.cachedKey, k.keyErr = k.loadOrCreateMasterKey()
	})
	return k.cachedKey, k.keyErr
}

// loadOrCreateMasterKey loads the master key from the system Keychain; auto-generates and stores it if not found.
func (k *darwinKeychain) loadOrCreateMasterKey() ([]byte, error) {
	encoded, err := keyring.Get(ServiceName, MasterKeyAccount)
	if err == nil {
		masterKey, decErr := base64.StdEncoding.DecodeString(encoded)
		if decErr != nil {
			return nil, fmt.Errorf("failed to decode master key: %w", decErr)
		}
		if len(masterKey) != masterKeySize {
			return nil, fmt.Errorf("invalid master key length: expected %d bytes, got %d bytes", masterKeySize, len(masterKey))
		}
		return masterKey, nil
	}

	if !errors.Is(err, keyring.ErrNotFound) {
		return nil, fmt.Errorf("failed to read system Keychain: %w", err)
	}

	masterKey, err := generateMasterKey()
	if err != nil {
		return nil, err
	}

	encoded = base64.StdEncoding.EncodeToString(masterKey)
	if err = keyring.Set(ServiceName, MasterKeyAccount, encoded); err != nil {
		return nil, fmt.Errorf("failed to write system Keychain: %w", err)
	}

	return masterKey, nil
}

func (k *darwinKeychain) Get(service, account string) (string, error) {
	masterKey, err := k.getMasterKey()
	if err != nil {
		return "", fmt.Errorf("failed to load master key: %w", err)
	}

	filePath := encFilePath(GetDataDir(), account)
	cipherData, err := readEncFile(filePath)
	if err != nil {
		return "", err
	}

	// SEC-009: account as AAD binds ciphertext to account, preventing cross-account .enc file replacement attacks.
	aad := []byte(account)
	plaintext, needsMigration, err := decryptWithAADFallback(masterKey, cipherData, aad)
	if err != nil {
		return "", err
	}
	if needsMigration {
		if newCipher, encErr := encrypt(masterKey, plaintext, aad); encErr == nil {
			if wErr := atomicWriteFile(filePath, newCipher); wErr != nil {
				fmt.Fprintf(os.Stderr, "warning: AAD migration write failed (account=%s): %v\n", account, wErr)
			}
		}
	}
	defer zeroBytes(plaintext)

	return string(plaintext), nil
}

func (k *darwinKeychain) Set(service, account, data string) error {
	masterKey, err := k.getMasterKey()
	if err != nil {
		return fmt.Errorf("failed to load master key: %w", err)
	}

	cipherData, err := encrypt(masterKey, []byte(data), []byte(account)) // SEC-009: account as AAD
	if err != nil {
		return err
	}

	filePath := encFilePath(GetDataDir(), account)
	return atomicWriteFile(filePath, cipherData)
}

func (k *darwinKeychain) Remove(service, account string) error {
	filePath := encFilePath(GetDataDir(), account)
	if err := os.Remove(filePath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ErrNotFound
		}
		return fmt.Errorf("failed to delete encrypted file: %w", err)
	}
	return nil
}
