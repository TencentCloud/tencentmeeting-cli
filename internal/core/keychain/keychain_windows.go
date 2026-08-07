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
	"syscall"
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

	// The syscall.Errno returned by Call() is preserved as-is (wrapped with %w),
	// so callers can use errors.Is / errors.As to classify DPAPI failures such as
	// NTE_BAD_KEY_STATE and trigger targeted self-healing (see isDPAPIKeyInvalid).
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

// isDPAPIKeyInvalid reports whether the given DPAPI error indicates that the
// user's DPAPI master key can no longer decrypt the historical ciphertext
// (e.g. Windows password was force-reset by an administrator, account type
// was switched between local/Microsoft account, Windows Hello / PIN was
// rebuilt, or the system was restored to a state predating the credential
// change).
//
// Only errors that unambiguously mean "key permanently unusable" are matched
// here. Transient failures (permission denied, DLL load failure, out-of-memory,
// EDR interception, etc.) MUST NOT match, otherwise a temporary hiccup would
// cause us to wipe recoverable ciphertext.
//
// Known Windows error codes that qualify:
//   - NTE_BAD_KEY_STATE       (0x8009000B) "Key not valid for use in specified state."
//     This is exactly what the reported user log shows.
//   - ERROR_INVALID_DATA      (0x0000000D) DPAPI blob header is intact but the
//     protected payload cannot be unwrapped with the current master key.
//   - NTE_BAD_DATA            (0x80090005) DPAPI blob is structurally corrupt
//     or was produced under a foreign credential (e.g. cloned VM image).
func isDPAPIKeyInvalid(err error) bool {
	if err == nil {
		return false
	}
	var errno syscall.Errno
	if !errors.As(err, &errno) {
		return false
	}
	// Compare via uintptr because the constants defined in golang.org/x/sys/windows
	// are typed as windows.Handle (not syscall.Errno), while procCryptUnprotectData.Call
	// returns syscall.Errno. Both share the same underlying numeric HRESULT/Win32 code.
	switch uintptr(errno) {
	case uintptr(windows.NTE_BAD_KEY_STATE),
		uintptr(windows.ERROR_INVALID_DATA),
		uintptr(windows.NTE_BAD_DATA):
		return true
	}
	return false
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
		entropyErr := err
		// Fallback: try decrypting without entropy (old format compatibility).
		masterKey, err = cryptUnprotectData(protectedKey, nil)
		if err != nil {
			// Self-heal only when BOTH attempts fail with a DPAPI "key permanently
			// unusable" error code (e.g. NTE_BAD_KEY_STATE). In that case the
			// ciphertext will never be decryptable again, so we purge the stale
			// master_key together with every business ciphertext encrypted under
			// it (they are all unrecoverable too), and return ErrNotFound so the
			// caller (loadOrCreateMasterKey) transparently generates a fresh
			// master key. The user will simply be prompted to re-login.
			//
			// Transient errors (permission denied, EDR interception, DLL not
			// loaded, ...) are surfaced as-is so recoverable data is not wiped.
			if isDPAPIKeyInvalid(entropyErr) && isDPAPIKeyInvalid(err) {
				k.purgeStaleRegistryEntries()
				return nil, ErrNotFound
			}
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

	// If after this deletion the only remaining value under the registry key is
	// master_key itself, treat the operation as the final "logout" step and
	// cascade-delete master_key as well. Rationale:
	//   - master_key is per-user (HKCU) and has no cross-user reuse value on
	//     Windows, so keeping it around brings no benefit.
	//   - If the user's DPAPI credentials later become invalid (password force-
	//     reset by admin, account type switched, Windows Hello rebuilt, system
	//     restore, ...), a leftover master_key becomes an undecryptable zombie
	//     that permanently blocks the next `login` ("Key not valid for use in
	//     specified state").
	//   - Regenerating master_key on the next login is cheap (<10ms).
	// If any other business account ciphertext still exists (e.g. multi-account
	// scenario where only one account is being removed), master_key MUST be
	// preserved so the remaining accounts stay decryptable.
	names, listErr := regKey.ReadValueNames(-1)
	if listErr != nil {
		// Best-effort: if enumeration fails (rare permission issue), skip the
		// cascade cleanup silently and fall back to the pre-change behavior.
		return nil
	}
	onlyMasterKeyLeft := true
	for _, n := range names {
		if n != MasterKeyAccount {
			onlyMasterKeyLeft = false
			break
		}
	}
	if onlyMasterKeyLeft {
		// Ignore the error deliberately: this cleanup is a defensive convenience,
		// not a correctness requirement. The next login will overwrite it anyway.
		_ = regKey.DeleteValue(MasterKeyAccount)
	}
	return nil
}

// purgeStaleRegistryEntries wipes every value under registryKeyPath. It is
// invoked as the last-resort self-healing action when the DPAPI master key
// has become permanently unusable (see isDPAPIKeyInvalid). All business
// ciphertexts share the same master key, so once the master key is dead every
// stored ciphertext is dead too; keeping them around would only produce
// misleading "decryption failed" errors on subsequent Get calls.
//
// The method is best-effort: any error while opening the registry or deleting
// individual values is swallowed, because the caller has already committed to
// returning ErrNotFound and letting the upper layer regenerate a fresh master
// key on the next login attempt.
func (k *windowsKeychain) purgeStaleRegistryEntries() {
	regKey, err := registry.OpenKey(registry.CURRENT_USER, registryKeyPath, registry.ALL_ACCESS)
	if err != nil {
		return
	}
	defer regKey.Close()
	names, err := regKey.ReadValueNames(-1)
	if err != nil {
		return
	}
	for _, n := range names {
		_ = regKey.DeleteValue(n)
	}
}
