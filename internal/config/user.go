package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"sync"

	"tmeet/internal/core/keychain"
	"tmeet/internal/exception"
)

// kc is the Keychain access instance, supports injection for testing.
var (
	kc   keychain.KeychainAccess
	kcMu sync.Mutex
)

// ClearUserConfigFunc clear user config func
type ClearUserConfigFunc func() error

// SetKeychain injects a custom Keychain implementation.
//
// When not called, GetUserConfig and other functions will automatically use the default platform implementation (keychain.New()).
// Primary use: inject keychain.NewMockKeychain() in unit tests to avoid real system calls.
//
// Usage:
//
//	config.SetKeychain(keychain.NewMockKeychain())
func SetKeychain(k keychain.KeychainAccess) {
	kcMu.Lock()
	defer kcMu.Unlock()
	kc = k
}

// getKeychain returns the Keychain instance (lazy initialization, thread-safe).
func getKeychain() keychain.KeychainAccess {
	kcMu.Lock()
	defer kcMu.Unlock()
	if kc == nil {
		kc = keychain.New()
	}
	return kc
}

// Only letters, digits, and underscores are allowed.
var openIdPattern = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)

// validateOpenId validates the OpenId format (whitelist mode) to prevent path traversal and filesystem anomalies.
// Also rejects values that conflict with keychain internal reserved names to avoid overwriting the master key.
func validateOpenId(openId string) error {
	if openId == "" {
		return fmt.Errorf("OpenId cannot be empty")
	}
	if !openIdPattern.MatchString(openId) {
		return fmt.Errorf("invalid OpenId format (only letters, digits, underscores allowed): %q", openId)
	}
	if openId == keychain.MasterKeyAccount {
		return fmt.Errorf("OpenId cannot use reserved name: %q", openId)
	}
	return nil
}

// GetUserConfig retrieves the user configuration for the currently active application.
//
// Internally decrypts and reads from the encrypted .enc file; callers do not need to know the encryption details.
// Returns (nil, nil) if not configured (config.json does not exist) or not logged in (.enc does not exist).
// Results are cached in memory; multiple calls will not repeat decryption.
//
// Note: userConfig cache is not concurrency-safe; this is fine for CLI single-threaded scenarios.
// If concurrent use is needed, additional locking is required to protect userConfig.
func GetUserConfig() (*UserConfig, error) {
	if userConfig != nil {
		return userConfig, nil
	}

	meta, err := loadMeta()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, exception.GetUserConfigUnknownError.With("failed to load app metadata: %v", err)
	}
	if meta.ActiveOpenId == "" {
		return nil, nil
	}

	if err = validateOpenId(meta.ActiveOpenId); err != nil {
		return nil, exception.GetUserConfigUnknownError.With("invalid OpenId in app metadata: %v", err)
	}

	data, err := getKeychain().Get(keychain.ServiceName, meta.ActiveOpenId)
	if err != nil {
		if errors.Is(err, keychain.ErrNotFound) {
			return nil, nil
		}
		return nil, exception.GetUserConfigUnknownError.With("failed to read encrypted config: %v", err)
	}
	if data == "" {
		return nil, nil
	}

	cfg := &UserConfig{}
	if err = json.Unmarshal([]byte(data), cfg); err != nil {
		return nil, exception.ParseUserConfigError.With("failed to parse user config: %v", err)
	}

	userConfig = cfg
	return userConfig, nil
}

// SaveUserConfig saves the user configuration (encrypted write to .enc file).
//
// Internally serializes UserConfig as JSON and stores it encrypted via keychain.
// Also updates active_open_id in config.json to support multi-user switching.
// cfg.OpenId must not be empty, otherwise an error is returned.
func SaveUserConfig(config *UserConfig) error {
	if config == nil {
		return exception.InvalidArgsError.With("config cannot be nil")
	}
	if err := validateOpenId(config.OpenId); err != nil {
		return exception.InvalidArgsError.With("invalid OpenId: %v", err)
	}

	data, err := json.Marshal(config)
	if err != nil {
		return exception.InitializeFailedError.With("failed to serialize user config: %v", err)
	}

	if err = getKeychain().Set(keychain.ServiceName, config.OpenId, string(data)); err != nil {
		return exception.InitializeFailedError.With("failed to save encrypted config: %v", err)
	}

	// Preserve the sub-account pointer when re-saving the SAME main account
	// (e.g. token refresh re-invokes SaveUserConfig and must not drop the
	// active sub-account). Switching to a DIFFERENT main account clears it,
	// since a sub-account belongs to a specific main account.
	newMeta := &AppMeta{ActiveOpenId: config.OpenId}
	if existing, metaErr := loadMeta(); metaErr == nil && existing != nil && existing.ActiveOpenId == config.OpenId {
		newMeta.ActiveAgentOpenId = existing.ActiveAgentOpenId
	}
	if err = saveMeta(newMeta); err != nil {
		return exception.InitializeFailedError.With("failed to update app metadata (encrypted data saved, please retry): %v", err)
	}

	userConfig = config
	return nil
}

// ResourceReleaseHook describes a piece of teardown work that must run
// — and succeed — BEFORE ClearUserConfig is allowed to delete the
// active user's credentials.  The contract is deliberately strict
// because logout is the only opportunity the CLI has to stop background
// resources owned by the outgoing account (notably the event-bus
// daemon); leaking those into the next login session is worse than
// failing the logout outright.
//
// Semantics:
//
//   - Fn is called with the OpenId currently recorded as ActiveOpenId.
//     Empty/invalid OpenId callers never reach hook dispatch (see
//     ClearUserConfig).
//   - Returning a non-nil error aborts ClearUserConfig.  The keychain
//     entry and config.json's active_open_id MUST remain intact so the
//     user (or a retry) can attempt logout again with full state.
//   - A panic is treated as a hook error: it is recovered, converted to
//     an error, and surfaces with the same abort semantics above.
//   - Name is a short stable identifier used only for diagnostics
//     (stderr / wrapped errors).  It is NOT a uniqueness key — duplicate
//     names are accepted and run independently.
//
// Hooks MUST NOT block on unbounded remote IO; ClearUserConfig is in
// the hot path of `tmeet auth logout` and any token-refresh fallback.
// Implementations should impose their own short timeouts.
type ResourceReleaseHook struct {
	Name string
	Fn   func(openId string) error
}

var (
	resourceReleaseHooks   []ResourceReleaseHook
	resourceReleaseHooksMu sync.Mutex
)

// RegisterResourceReleaseHook installs h so it runs every time
// ClearUserConfig is asked to tear down a user's credentials.
//
// Decoupling design: this lives in `internal/config` (the lowest-level
// package in the dependency graph) so packages further up — notably
// internal/event — can register resource-release logic without
// internal/config having to import them (which would form a cycle:
// event already depends on config for paths.GetConfigDir).
//
// Registrations are append-only and run in registration order.  A nil
// Fn is silently ignored to keep call sites free of nil-checks.  Tests
// using the keychain mock should NOT register here unless they
// explicitly ResetResourceReleaseHooksForTest in their cleanup.
func RegisterResourceReleaseHook(h ResourceReleaseHook) {
	if h.Fn == nil {
		return
	}
	resourceReleaseHooksMu.Lock()
	defer resourceReleaseHooksMu.Unlock()
	resourceReleaseHooks = append(resourceReleaseHooks, h)
}

// ResetResourceReleaseHooksForTest clears every registered hook.
// Test-only hatch — production callers should not invoke this.
func ResetResourceReleaseHooksForTest() {
	resourceReleaseHooksMu.Lock()
	defer resourceReleaseHooksMu.Unlock()
	resourceReleaseHooks = nil
}

// snapshotResourceReleaseHooks returns a slice snapshot of the
// registered hook list, taken under the lock.  We snapshot rather than
// holding the lock during dispatch so a hook that itself registers
// further hooks (rare but possible during init) can't deadlock.
func snapshotResourceReleaseHooks() []ResourceReleaseHook {
	resourceReleaseHooksMu.Lock()
	defer resourceReleaseHooksMu.Unlock()
	if len(resourceReleaseHooks) == 0 {
		return nil
	}
	out := make([]ResourceReleaseHook, len(resourceReleaseHooks))
	copy(out, resourceReleaseHooks)
	return out
}

// runResourceReleaseHooks dispatches every registered hook in order
// with the OpenId currently being cleared.  Returns the first hook
// error (subsequent hooks are NOT executed — fail-fast keeps the
// "all-or-nothing" contract simple to reason about: either every
// resource is released and we proceed to credential teardown, or we
// abort with state intact).
//
// A panicking hook is recovered and converted to an error so a buggy
// hook cannot hijack the deferred clearMeta path in ClearUserConfig.
func runResourceReleaseHooks(openId string) error {
	for _, h := range snapshotResourceReleaseHooks() {
		if err := invokeResourceReleaseHook(h, openId); err != nil {
			return err
		}
	}
	return nil
}

// invokeResourceReleaseHook runs a single hook with panic-to-error
// translation.  Extracted so the recover() lives in its own frame and
// can't accidentally short-circuit the surrounding loop.
func invokeResourceReleaseHook(h ResourceReleaseHook, openId string) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("resource-release hook %q panicked: %v", h.Name, r)
		}
	}()
	if hookErr := h.Fn(openId); hookErr != nil {
		return fmt.Errorf("resource-release hook %q failed: %w", h.Name, hookErr)
	}
	return nil
}

// ClearUserConfig tears down the currently active user's session.
//
// Sequence (strict — every step must succeed before the next runs):
//
//  1. Read meta to capture the OpenId being cleared.  Missing/invalid
//     meta means there is nothing to log out from; return nil.
//  2. Run every ResourceReleaseHook with the captured OpenId.  These
//     hooks own resources scoped to the outgoing account (e.g. the
//     event-bus daemon).  If ANY hook errors / panics, ClearUserConfig
//     aborts immediately: the keychain entry and active_open_id are
//     left intact so the user retains the credentials needed to retry
//     teardown.  The deferred clearMeta() is suppressed in this case.
//  3. Remove the keychain entry for the OpenId.  On failure, the
//     deferred clearMeta() still runs — meta pointing at a nonexistent
//     keychain entry is the documented "logged out, force-clean meta"
//     state and is preferable to leaking an active_open_id pointer.
//  4. Clear the active_open_id pointer in config.json (deferred).
//
// The master key is not deleted — it remains for other users on the
// same machine.
func ClearUserConfig() (retErr error) {
	// Track whether we're aborting in step 2 so the deferred meta-clear
	// is suppressed: if resource release failed we MUST keep the
	// credentials addressable (active_open_id + keychain entry both
	// intact) so the user can retry.
	resourceReleaseFailed := false
	defer func() {
		userConfig = nil
		agentAccountConfig = nil
		if resourceReleaseFailed {
			return
		}
		if err := clearMeta(); err != nil && retErr == nil {
			retErr = exception.LogoutFailedError.With("failed to clear app metadata: %v", err)
		}
	}()

	meta, err := loadMeta()
	if err != nil || meta == nil || meta.ActiveOpenId == "" {
		return nil
	}

	if err = validateOpenId(meta.ActiveOpenId); err != nil {
		return nil
	}

	clearedOpenId := meta.ActiveOpenId

	// Step 2: stop every resource owned by this account BEFORE deleting
	// the credentials it needs to identify itself.  Fail-fast — see
	// ResourceReleaseHook contract.
	if err = runResourceReleaseHooks(clearedOpenId); err != nil {
		resourceReleaseFailed = true
		return exception.LogoutFailedError.With("failed to release user resources: %v", err)
	}

	// Step 3: delete the keychain entry.  ErrNotFound is benign — the
	// entry was already gone (e.g. a previous partial logout) and we
	// proceed to clear the dangling meta pointer.
	if err = getKeychain().Remove(keychain.ServiceName, clearedOpenId); err != nil && !errors.Is(err, keychain.ErrNotFound) {
		return exception.LogoutFailedError.With("failed to delete encrypted config: %v", err)
	}

	// Step 3.5: delete the agent (sub-account) encrypted entry owned by
	// this main account, if any, to avoid orphaned agent files after logout.
	if meta.ActiveAgentOpenId != "" && validateAgentOpenId(meta.ActiveAgentOpenId) == nil {
		if err = getKeychain().Remove(keychain.ServiceName, agentKey(meta.ActiveAgentOpenId)); err != nil &&
			!errors.Is(err, keychain.ErrNotFound) {
			return exception.LogoutFailedError.With("failed to delete encrypted agent config: %v", err)
		}
	}

	return nil
}

// ClearUserConfigUnResource clears the configuration for the currently active user.
//
// Deletes the corresponding .enc encrypted file and active_open_id from config.json.
// The master key is not deleted (for use by other users).
func ClearUserConfigUnResource() (retErr error) {
	defer func() {
		userConfig = nil
		agentAccountConfig = nil
		if err := clearMeta(); err != nil && retErr == nil {
			retErr = exception.LogoutFailedError.With("failed to clear app metadata: %v", err)
		}
	}()

	meta, err := loadMeta()
	if err != nil || meta == nil || meta.ActiveOpenId == "" {
		return nil
	}

	if err = validateOpenId(meta.ActiveOpenId); err != nil {
		return nil
	}

	if err = getKeychain().Remove(keychain.ServiceName, meta.ActiveOpenId); err != nil && !errors.Is(err, keychain.ErrNotFound) {
		return exception.LogoutFailedError.With("failed to delete encrypted config: %v", err)
	}

	// Also delete the agent (sub-account) encrypted entry owned by this main
	// account, if any, to avoid orphaned agent files.
	if meta.ActiveAgentOpenId != "" && validateAgentOpenId(meta.ActiveAgentOpenId) == nil {
		if err = getKeychain().Remove(keychain.ServiceName, agentKey(meta.ActiveAgentOpenId)); err != nil &&
			!errors.Is(err, keychain.ErrNotFound) {
			return exception.LogoutFailedError.With("failed to delete encrypted agent config: %v", err)
		}
	}
	return nil
}

// ResetCache clears in-memory config cache; the next call to GetUserConfig / GetAgentAccountConfig
// will re-read from the encrypted store.
func ResetCache() {
	userConfig = nil
	agentAccountConfig = nil
}
