//go:build windows

// keychain_windows.go is the Windows platform Keychain implementation, referencing Feishu lark-cli / DingTalk dws.
//
// The master key is encrypted with DPAPI (CryptProtectData) and stored in the HKCU registry.
// DPAPI is bound to the current Windows user's login credentials; other users/devices cannot decrypt.
// Encrypted data is stored in the registry in base64 form after AES-256-GCM encryption.
//
// Registry path: HKCU\Software\TmeetCli\keychain
//   - master_key: DPAPI-encrypted master key (base64-encoded)
//   - <account>:  AES-256-GCM encrypted business data (base64-encoded)

package keychain

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const (
	registryKeyPath = `Software\TmeetCli\keychain`

	// cryptprotectUIForbidden disables DPAPI from showing user interaction dialogs (CLI tools should not show popups).
	cryptprotectUIForbidden = 0x1
)

// windowsKeychain is the Windows platform implementation using DPAPI + registry.
// The master key is cached in memory via sync.Once to avoid DPAPI decryption + registry reads on every operation (~2-10ms).
type windowsKeychain struct {
	cachedKey []byte
	keyErr    error
	once      sync.Once
}

func newPlatformKeychain() KeychainAccess {
	return &windowsKeychain{}
}

type dataBlob struct {
	cbData uint32
	pbData *byte
}

func newDataBlob(data []byte) *dataBlob {
	if len(data) == 0 {
		return &dataBlob{}
	}
	return &dataBlob{
		cbData: uint32(len(data)),
		pbData: &data[0],
	}
}

func (b *dataBlob) toBytes() []byte {
	d := make([]byte, b.cbData)
	copy(d, unsafe.Slice(b.pbData, b.cbData))
	return d
}

var (
	crypt32                = windows.NewLazySystemDLL("crypt32.dll")
	procCryptProtectData   = crypt32.NewProc("CryptProtectData")
	procCryptUnprotectData = crypt32.NewProc("CryptUnprotectData")
)

// dpapiEntropy generates the DPAPI optionalEntropy parameter (SEC-004).
// Format aligned with Feishu/DingTalk: service + "\x00" + account.
func dpapiEntropy(service, account string) []byte {
	return []byte(service + "\x00" + account)
}

func cryptProtectData(plaintext, entropy []byte) ([]byte, error) {
	inBlob := newDataBlob(plaintext)
	var entropyPtr uintptr
	if len(entropy) > 0 {
		entropyBlob := newDataBlob(entropy)
		entropyPtr = uintptr(unsafe.Pointer(entropyBlob))
	}
	var outBlob dataBlob

	r, _, err := procCryptProtectData.Call(
		uintptr(unsafe.Pointer(inBlob)),
		0,
		entropyPtr,
		0, 0,
		cryptprotectUIForbidden,
		uintptr(unsafe.Pointer(&outBlob)),
	)
	if r == 0 {
		return nil, fmt.Errorf("CryptProtectData call failed: %w", err)
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(outBlob.pbData)))

	return outBlob.toBytes(), nil
}

func cryptUnprotectData(ciphertext, entropy []byte) ([]byte, error) {
	inBlob := newDataBlob(ciphertext)
	var entropyPtr uintptr
	if len(entropy) > 0 {
		entropyBlob := newDataBlob(entropy)
		entropyPtr = uintptr(unsafe.Pointer(entropyBlob))
	}
	var outBlob dataBlob

	r, _, err := procCryptUnprotectData.Call(
		uintptr(unsafe.Pointer(inBlob)),
		0,
		entropyPtr,
		0, 0,
		cryptprotectUIForbidden,
		uintptr(unsafe.Pointer(&outBlob)),
	)
	if r == 0 {
		return nil, fmt.Errorf("CryptUnprotectData call failed: %w", err)
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(outBlob.pbData)))

	return outBlob.toBytes(), nil
}

// getMasterKey retrieves the master key (with in-memory cache).
func (k *windowsKeychain) getMasterKey() ([]byte, error) {
	k.once.Do(func() {
		k.cachedKey, k.keyErr = k.loadOrCreateMasterKey()
	})
	return k.cachedKey, k.keyErr
}

// loadOrCreateMasterKey loads the DPAPI-protected master key from the registry; auto-generates if not found.
// Returns an error directly on permission denied, format anomalies, etc. to avoid silently rebuilding and making historical ciphertexts undecryptable.
func (k *windowsKeychain) loadOrCreateMasterKey() ([]byte, error) {
	masterKey, err := k.loadMasterKeyFromRegistry()
	if err == nil {
		return masterKey, nil
	}
	if err != ErrNotFound {
		return nil, err
	}

	// master key 不存在，首次生成
	return k.createAndStoreMasterKey()
}

// loadMasterKeyFromRegistry reads and decrypts the master key from the registry.
// Returns ErrNotFound if the key or value does not exist; returns specific errors for permission denied, corruption, etc.
func (k *windowsKeychain) loadMasterKeyFromRegistry() ([]byte, error) {
	regKey, err := registry.OpenKey(registry.CURRENT_USER, registryKeyPath, registry.READ)
	if err != nil {
		if errors.Is(err, registry.ErrNotExist) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to open registry key: %w", err)
	}
	defer regKey.Close()

	encoded, _, err := regKey.GetStringValue(MasterKeyAccount)
	if err != nil {
		if errors.Is(err, registry.ErrNotExist) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to read master key from registry: %w", err)
	}

	protectedKey, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("failed to decode master key base64: %w", err)
	}

	// SEC-004: Use optionalEntropy to raise the DPAPI decryption threshold, aligned with Feishu/DingTalk.
	// Other processes on the same user don't know the entropy value and cannot directly call CryptUnprotectData to decrypt.
	// Backward compatibility: old DPAPI ciphertexts have no entropy; auto-migrate to entropy version on first read.
	entropy := dpapiEntropy(ServiceName, MasterKeyAccount)
	masterKey, err := cryptUnprotectData(protectedKey, entropy)
	if err != nil {
		// Fallback: try decrypting without entropy (old format compatibility).
		masterKey, err = cryptUnprotectData(protectedKey, nil)
		if err != nil {
			return nil, fmt.Errorf("DPAPI failed to decrypt master key (user credentials may have changed): %w", err)
		}
		// Auto-migrate: re-encrypt with entropy-based DPAPI and update registry.
		if newProtected, reErr := cryptProtectData(masterKey, entropy); reErr == nil {
			newEncoded := base64.StdEncoding.EncodeToString(newProtected)
			if wKey, _, wErr := registry.CreateKey(registry.CURRENT_USER, registryKeyPath, registry.ALL_ACCESS); wErr == nil {
				if sErr := wKey.SetStringValue(MasterKeyAccount, newEncoded); sErr != nil {
					fmt.Fprintf(os.Stderr, "warning: DPAPI entropy migration write failed: %v\n", sErr)
				}
				wKey.Close()
			} else {
				fmt.Fprintf(os.Stderr, "warning: DPAPI entropy migration open registry failed: %v\n", wErr)
			}
		} else {
			fmt.Fprintf(os.Stderr, "warning: DPAPI entropy migration re-encrypt failed: %v\n", reErr)
		}
	}
	if len(masterKey) != masterKeySize {
		return nil, fmt.Errorf("invalid master key length: expected %d bytes, got %d bytes", masterKeySize, len(masterKey))
	}
	return masterKey, nil
}

// createAndStoreMasterKey generates a new master key, encrypts it with DPAPI, and stores it in the registry.
func (k *windowsKeychain) createAndStoreMasterKey() ([]byte, error) {
	masterKey, err := generateMasterKey()
	if err != nil {
		return nil, err
	}

	entropy := dpapiEntropy(ServiceName, MasterKeyAccount)
	protectedKey, err := cryptProtectData(masterKey, entropy)
	if err != nil {
		return nil, fmt.Errorf("DPAPI failed to encrypt master key: %w", err)
	}

	regKey, _, err := registry.CreateKey(registry.CURRENT_USER, registryKeyPath, registry.ALL_ACCESS)
	if err != nil {
		return nil, fmt.Errorf("failed to create registry key: %w", err)
	}
	defer regKey.Close()

	encoded := base64.StdEncoding.EncodeToString(protectedKey)
	if err = regKey.SetStringValue(MasterKeyAccount, encoded); err != nil {
		return nil, fmt.Errorf("failed to write to registry: %w", err)
	}

	return masterKey, nil
}

func (k *windowsKeychain) Get(service, account string) (string, error) {
	masterKey, err := k.getMasterKey()
	if err != nil {
		return "", fmt.Errorf("failed to load master key: %w", err)
	}

	regKey, err := registry.OpenKey(registry.CURRENT_USER, registryKeyPath, registry.READ)
	if err != nil {
		return "", ErrNotFound
	}
	defer regKey.Close()

	encoded, _, err := regKey.GetStringValue(account)
	if err != nil {
		return "", ErrNotFound
	}

	cipherData, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("failed to decode encrypted data base64: %w", err)
	}

	// SEC-009: account as AAD binds ciphertext to account, preventing cross-account registry value replacement attacks.
	aad := []byte(account)
	plaintext, needsMigration, err := decryptWithAADFallback(masterKey, cipherData, aad)
	if err != nil {
		return "", err
	}
	if needsMigration {
		if newCipher, encErr := encrypt(masterKey, plaintext, aad); encErr == nil {
			newEncoded := base64.StdEncoding.EncodeToString(newCipher)
			if wKey, _, wErr := registry.CreateKey(registry.CURRENT_USER, registryKeyPath, registry.ALL_ACCESS); wErr == nil {
				if sErr := wKey.SetStringValue(account, newEncoded); sErr != nil {
					fmt.Fprintf(os.Stderr, "warning: AAD migration write failed (account=%s): %v\n", account, sErr)
				}
				wKey.Close()
			}
		}
	}
	defer zeroBytes(plaintext)

	return string(plaintext), nil
}

func (k *windowsKeychain) Set(service, account, data string) error {
	masterKey, err := k.getMasterKey()
	if err != nil {
		return fmt.Errorf("failed to load master key: %w", err)
	}

	cipherData, err := encrypt(masterKey, []byte(data), []byte(account)) // SEC-009: account 作为 AAD
	if err != nil {
		return err
	}

	regKey, _, err := registry.CreateKey(registry.CURRENT_USER, registryKeyPath, registry.ALL_ACCESS)
	if err != nil {
		return fmt.Errorf("failed to create registry key: %w", err)
	}
	defer regKey.Close()

	encoded := base64.StdEncoding.EncodeToString(cipherData)
	if err = regKey.SetStringValue(account, encoded); err != nil {
		return fmt.Errorf("failed to write to registry: %w", err)
	}

	return nil
}

func (k *windowsKeychain) Remove(service, account string) error {
	regKey, err := registry.OpenKey(registry.CURRENT_USER, registryKeyPath, registry.ALL_ACCESS)
	if err != nil {
		return ErrNotFound
	}
	defer regKey.Close()

	if err = regKey.DeleteValue(account); err != nil {
		return ErrNotFound
	}

	return nil
}
