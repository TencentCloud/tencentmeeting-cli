//go:build !darwin && !windows

// keychain_other.go is the Linux/Unix platform Keychain implementation.
//
// Linux lacks a unified Keychain mechanism (GNOME Keyring / KWallet have low coverage, CI/Docker has no desktop environment),
// falling back to file storage is the industry standard practice (Feishu lark-cli, DingTalk dws, GitHub CLI all do this).
//
// The master key is stored as raw bytes in GetDataDir()/master.key with permission 0600 (readable/writable only by the current user).
// Encrypted data is stored as .enc files after AES-256-GCM encryption.
//
// Security note: if an attacker has already obtained current user permissions, 0600 files and browser cookies carry the same risk level.

package keychain

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
)

// fileKeychain is the Linux/Unix platform implementation using file storage for the master key.
// The master key is cached in memory via sync.Once to avoid reading the file on every operation.
type fileKeychain struct {
	cachedKey []byte
	keyErr    error
	once      sync.Once
}

func newPlatformKeychain() KeychainAccess {
	return &fileKeychain{}
}

// masterKeyPath returns the full path to the master key file.
func masterKeyPath() string {
	return filepath.Join(GetDataDir(), MasterKeyAccount)
}

// getMasterKey retrieves the master key (with in-memory cache).
// On first call, loads from file or generates; subsequent calls return the cached value.
func (k *fileKeychain) getMasterKey() ([]byte, error) {
	k.once.Do(func() {
		k.cachedKey, k.keyErr = k.loadOrCreateMasterKey()
	})
	return k.cachedKey, k.keyErr
}

// loadOrCreateMasterKey loads the master key from file; auto-generates and writes it (permission 0600) if not found.
func (k *fileKeychain) loadOrCreateMasterKey() ([]byte, error) {
	mkPath := masterKeyPath()

	data, err := os.ReadFile(mkPath)
	if err == nil {
		if len(data) != masterKeySize {
			return nil, fmt.Errorf("invalid master key file length: expected %d bytes, got %d bytes", masterKeySize, len(data))
		}
		info, statErr := os.Stat(mkPath)
		if statErr == nil && info.Mode().Perm() != 0600 {
			fmt.Fprintf(os.Stderr, "warning: insecure permissions on master key file: %o, recommend 0600\n", info.Mode().Perm())
		}
		return data, nil
	}

	if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("failed to read master key file: %w", err)
	}

	// SEC-001: Tighten umask to 0077 before writing master.key to ensure new files/directories are not readable by other users on the same machine.
	// umask is process-global state; defer restores it to avoid affecting subsequent operations.
	oldUmask := syscall.Umask(0077)
	defer syscall.Umask(oldUmask)

	masterKey, err := generateMasterKey()
	if err != nil {
		return nil, err
	}

	dir := filepath.Dir(mkPath)
	if err = os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}
	if err = os.Chmod(dir, 0700); err != nil { // SEC-002: umask may cause MkdirAll to create 0755
		return nil, fmt.Errorf("failed to set data directory permissions: %w", err)
	}

	if err = atomicWriteFile(mkPath, masterKey); err != nil {
		return nil, fmt.Errorf("failed to write master key file: %w", err)
	}

	if err = os.Chmod(mkPath, 0600); err != nil {
		return nil, fmt.Errorf("failed to set master key file permissions: %w", err)
	}

	return masterKey, nil
}

func (k *fileKeychain) Get(service, account string) (string, error) {
	masterKey, err := k.getMasterKey()
	if err != nil {
		return "", fmt.Errorf("failed to load master key: %w", err)
	}

	filePath := encFilePath(GetDataDir(), account)
	cipherData, err := readEncFile(filePath)
	if err != nil {
		return "", err
	}

	// SEC-009: account 作为 AAD 绑定密文与账号，防止跨账号 .enc 文件替换攻击。
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

func (k *fileKeychain) Set(service, account, data string) error {
	masterKey, err := k.getMasterKey()
	if err != nil {
		return fmt.Errorf("failed to load master key: %w", err)
	}

	cipherData, err := encrypt(masterKey, []byte(data), []byte(account)) // SEC-009: account 作为 AAD
	if err != nil {
		return err
	}

	filePath := encFilePath(GetDataDir(), account)
	return atomicWriteFile(filePath, cipherData)
}

func (k *fileKeychain) Remove(service, account string) error {
	filePath := encFilePath(GetDataDir(), account)
	if err := os.Remove(filePath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ErrNotFound
		}
		return fmt.Errorf("failed to delete encrypted file: %w", err)
	}
	return nil
}
