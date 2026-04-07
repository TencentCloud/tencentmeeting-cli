// crypto.go provides AES-256-GCM encryption/decryption, master key generation, and atomic .enc file read/write.
//
// Encryption format: nonce(12B) || ciphertext || tag(16B)
// Total overhead is only 28 bytes; a typical token JSON of ~500-800B encrypts to under 1KB.
//
// This file is the common infrastructure for all platform keychain implementations and contains no platform-specific code.

package keychain

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"tmeet/internal/core/filecheck"
	"tmeet/internal/core/filelock"
)

const (
	masterKeySize = 32 // AES-256 key length (bytes)
	nonceSize     = 12 // GCM standard nonce length (bytes)
)

// generateMasterKey generates a 32-byte (256-bit) cryptographically secure random master key.
// Uses crypto/rand to ensure unpredictability; the key is typically stored only once and read-only afterwards.
func generateMasterKey() ([]byte, error) {
	key := make([]byte, masterKeySize)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("failed to generate master key: %w", err)
	}
	return key, nil
}

// encrypt performs authenticated encryption of plaintext using AES-256-GCM.
//
// Return format: nonce(12B) || ciphertext || tag(16B)
// Each encryption uses a new random nonce, ensuring different ciphertexts for the same plaintext.
// additionalData is used as GCM AAD for authentication but not encrypted, binding context (e.g. account key) to prevent cross-account replacement (SEC-009).
func encrypt(masterKey, plaintext, additionalData []byte) ([]byte, error) {
	block, err := aes.NewCipher(masterKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := make([]byte, nonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate random nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, additionalData)
	return ciphertext, nil
}

// decrypt performs authenticated decryption using AES-256-GCM.
//
// Input format: nonce(12B) || ciphertext || tag(16B)
// Any tampering (including nonce, ciphertext, tag, or AAD mismatch) will cause decryption failure, providing integrity guarantees (AEAD).
// additionalData must match the value passed during encryption, otherwise authentication fails (SEC-009).
func decrypt(masterKey, cipherData, additionalData []byte) ([]byte, error) {
	minLen := nonceSize + 16
	if len(cipherData) < minLen {
		return nil, fmt.Errorf("ciphertext too short: length %d, minimum %d bytes required", len(cipherData), minLen)
	}

	block, err := aes.NewCipher(masterKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := cipherData[:nonceSize]
	ciphertext := cipherData[nonceSize:]

	plaintext, err := gcm.Open(nil, nonce, ciphertext, additionalData)
	if err != nil {
		return nil, fmt.Errorf("AES-GCM decryption failed (data may be tampered or master key mismatch): %w", err)
	}

	return plaintext, nil
}

// decryptWithAADFallback performs decryption with AAD fallback (SEC-009).
//
// Decryption strategy:
//  1. Try decrypting with aad first (new format)
//  2. If failed, try decrypting with nil AAD (backward compatibility with old format)
//  3. Return error if all attempts fail
//
// Return value needsMigration=true means decryption succeeded via nil AAD fallback;
// callers should re-encrypt the data with AAD.
// This function is pure cryptographic decision logic with no platform I/O, testable on any platform.
func decryptWithAADFallback(masterKey, cipherData, aad []byte) (plaintext []byte, needsMigration bool, err error) {
	plaintext, err = decrypt(masterKey, cipherData, aad)
	if err == nil {
		return plaintext, false, nil
	}

	plaintext, err = decrypt(masterKey, cipherData, nil)
	if err != nil {
		return nil, false, err
	}
	return plaintext, true, nil
}

// atomicWriteFile atomically writes data to the specified file.
//
// Write strategy: write to .tmp temp file → Sync flush → os.Rename atomic replace.
// Ensures existing files are not corrupted even if write is interrupted or process crashes.
// Used for .enc cipher files and master.key and other files requiring atomicity.
//
// Security hardening:
//   - SEC-002: Chmod(0700) after MkdirAll to prevent directory permissions being too loose due to umask
//   - SEC-012: filelock cross-process exclusive lock to prevent concurrent writes from multiple CLI instances
func atomicWriteFile(filePath string, data []byte) error {
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}
	if err := os.Chmod(dir, 0700); err != nil { // SEC-002: umask may cause MkdirAll to create 0755
		return fmt.Errorf("failed to set data directory permissions: %w", err)
	}

	lockPath := filePath + ".lock"
	return filelock.WithLock(lockPath, func() error { // SEC-012: 跨进程排他锁
		tmpFile, err := os.CreateTemp(dir, "."+filepath.Base(filePath)+"-*.tmp")
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

		if err = os.Rename(tmpPath, filePath); err != nil {
			return fmt.Errorf("failed to atomically replace encrypted file: %w", err)
		}

		return nil
	})
}

// readEncFile reads the full contents of a .enc encrypted file.
// Returns ErrNotFound when the file does not exist; callers can use this to detect first-time use.
//
// Security check via filecheck before reading (SEC-010):
// Rejects symlinks and files not owned by the current user to prevent symlink attacks and cross-user file replacement.
func readEncFile(filePath string) ([]byte, error) {
	if err := filecheck.ValidateBeforeRead(filePath); err != nil { // SEC-010
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to read encrypted file: %w", err)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("encrypted file is empty: %s", filePath)
	}
	return data, nil
}

// encFilePath 拼接加密数据文件的完整路径。
// 格式: <dataDir>/<account>.enc
func encFilePath(dataDir, account string) string {
	return filepath.Join(dataDir, account+".enc")
}

// zeroBytes zeroes out a byte slice to securely erase sensitive data from memory.
//
// Note: Go's GC may have already copied the data before zeroing, and string types are immutable and cannot be zeroed.
// This function provides best-effort memory cleanup and cannot completely eliminate all memory residue.
func zeroBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
