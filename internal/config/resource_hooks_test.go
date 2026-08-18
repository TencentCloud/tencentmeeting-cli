// resource_hooks_test.go — ResourceReleaseHook regression coverage for
// ClearUserConfig.
//
// We pin the strict-ordering / fail-fast contract documented on
// ResourceReleaseHook and ClearUserConfig:
//
//  1. Hooks run BEFORE the keychain entry is removed.  When a hook
//     returns an error (or panics) ClearUserConfig must abort with the
//     keychain entry AND active_open_id still intact, so a retry has
//     full state to address the resource again.
//  2. Hooks run in registration order and fail-fast: a failing hook
//     short-circuits subsequent hooks.
//  3. When every hook succeeds, the credential teardown proceeds
//     normally and the user is fully logged out.
//  4. When there is no active user, hooks must NOT fire (no OpenId to
//     pass; protects implementations from defensive empty-string
//     handling).
//  5. RegisterResourceReleaseHook must accept (and ignore) a hook with
//     a nil Fn.
//
// All tests register via the production API and must always cleanup
// with ResetResourceReleaseHooksForTest in t.Cleanup to avoid
// bleed-through to other tests in the same package.

package config

import (
	"errors"
	"os"
	"sync"
	"testing"

	"tmeet/internal/core/keychain"
)

// hookOpenId is the OpenId reused across the table-style tests below.
// Kept short and whitelist-safe per validateOpenId.
const hookOpenId = "alice"

// saveUser is a tiny helper to land a credential under hookOpenId so
// the subsequent ClearUserConfig has something to tear down.
func saveUser(t *testing.T, openId string) {
	t.Helper()
	if err := SaveUserConfig(&UserConfig{OpenId: openId, AccessToken: "tok"}); err != nil {
		t.Fatalf("SaveUserConfig: %v", err)
	}
}

// assertCredentialPresent verifies the keychain entry AND active_open_id
// are both intact — the post-condition we expect after a hook abort.
func assertCredentialPresent(t *testing.T, mock *keychain.MockKeychain, openId string) {
	t.Helper()
	data, err := mock.Get(keychain.ServiceName, openId)
	if err != nil || data == "" {
		t.Fatalf("expected keychain entry for %q to remain after hook abort, got data=%q err=%v", openId, data, err)
	}
	meta, err := loadMeta()
	if err != nil {
		t.Fatalf("loadMeta after abort: %v", err)
	}
	if meta == nil || meta.ActiveOpenId != openId {
		t.Fatalf("expected active_open_id=%q to remain after hook abort, meta=%+v", openId, meta)
	}
}

// assertCredentialGone verifies a clean teardown — keychain entry
// removed AND active_open_id cleared.  clearMeta() deletes the whole
// config.json file, so loadMeta returning os.ErrNotExist is the
// canonical "logged out" signal here.
func assertCredentialGone(t *testing.T, mock *keychain.MockKeychain, openId string) {
	t.Helper()
	if data, err := mock.Get(keychain.ServiceName, openId); err == nil && data != "" {
		t.Fatalf("expected keychain entry for %q to be removed, got data=%q", openId, data)
	}
	meta, err := loadMeta()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return
		}
		t.Fatalf("loadMeta after success: %v", err)
	}
	if meta != nil && meta.ActiveOpenId != "" {
		t.Fatalf("expected active_open_id cleared, meta=%+v", meta)
	}
}

func TestClearUserConfig_HookErrorAbortsAndPreservesCredential(t *testing.T) {
	mock, cleanup := setupTestEnv(t)
	defer cleanup()
	t.Cleanup(ResetResourceReleaseHooksForTest)

	saveUser(t, hookOpenId)

	wantErr := errors.New("bus still alive")
	var gotOpenId string
	RegisterResourceReleaseHook(ResourceReleaseHook{
		Name: "fake-bus",
		Fn: func(openId string) error {
			gotOpenId = openId
			return wantErr
		},
	})

	err := ClearUserConfig()
	if err == nil {
		t.Fatal("ClearUserConfig must surface hook error, got nil")
	}
	if gotOpenId != hookOpenId {
		t.Errorf("hook openId = %q, want %q", gotOpenId, hookOpenId)
	}

	// Critical post-condition: credentials remain so the user can retry.
	assertCredentialPresent(t, mock, hookOpenId)
}

func TestClearUserConfig_HookPanicAbortsAndPreservesCredential(t *testing.T) {
	mock, cleanup := setupTestEnv(t)
	defer cleanup()
	t.Cleanup(ResetResourceReleaseHooksForTest)

	saveUser(t, hookOpenId)

	RegisterResourceReleaseHook(ResourceReleaseHook{
		Name: "panicky",
		Fn: func(openId string) error {
			panic("boom")
		},
	})

	err := ClearUserConfig()
	if err == nil {
		t.Fatal("panicking hook must surface as error, got nil")
	}
	assertCredentialPresent(t, mock, hookOpenId)
}

func TestClearUserConfig_HooksRunInOrderAndShortCircuit(t *testing.T) {
	mock, cleanup := setupTestEnv(t)
	defer cleanup()
	t.Cleanup(ResetResourceReleaseHooksForTest)

	saveUser(t, hookOpenId)

	var (
		mu    sync.Mutex
		order []string
	)
	record := func(name string, fail bool) ResourceReleaseHook {
		return ResourceReleaseHook{
			Name: name,
			Fn: func(openId string) error {
				mu.Lock()
				order = append(order, name)
				mu.Unlock()
				if fail {
					return errors.New("stop here")
				}
				return nil
			},
		}
	}

	RegisterResourceReleaseHook(record("first", false))
	RegisterResourceReleaseHook(record("second", true))
	// Third hook MUST NOT run — fail-fast short-circuit.
	RegisterResourceReleaseHook(record("third", false))

	if err := ClearUserConfig(); err == nil {
		t.Fatal("expected error from second hook")
	}

	mu.Lock()
	defer mu.Unlock()
	if len(order) != 2 || order[0] != "first" || order[1] != "second" {
		t.Errorf("hook order = %v, want [first second]", order)
	}
	assertCredentialPresent(t, mock, hookOpenId)
}

func TestClearUserConfig_AllHooksSucceed_CredentialFullyCleared(t *testing.T) {
	mock, cleanup := setupTestEnv(t)
	defer cleanup()
	t.Cleanup(ResetResourceReleaseHooksForTest)

	saveUser(t, hookOpenId)

	var fired int
	RegisterResourceReleaseHook(ResourceReleaseHook{
		Name: "ok-1",
		Fn: func(openId string) error {
			fired++
			return nil
		},
	})
	RegisterResourceReleaseHook(ResourceReleaseHook{
		Name: "ok-2",
		Fn: func(openId string) error {
			fired++
			return nil
		},
	})

	if err := ClearUserConfig(); err != nil {
		t.Fatalf("ClearUserConfig: %v", err)
	}
	if fired != 2 {
		t.Errorf("hooks fired = %d, want 2", fired)
	}
	assertCredentialGone(t, mock, hookOpenId)
}

func TestClearUserConfig_NoActiveUser_HooksSkipped(t *testing.T) {
	// When there is no logged-in user, ClearUserConfig must be a no-op
	// AND must NOT call any hook (no OpenId to pass).  Protects hooks
	// from having to defensively handle empty input.
	_, cleanup := setupTestEnv(t)
	defer cleanup()
	t.Cleanup(ResetResourceReleaseHooksForTest)

	var fired bool
	RegisterResourceReleaseHook(ResourceReleaseHook{
		Name: "should-not-fire",
		Fn: func(openId string) error {
			fired = true
			return nil
		},
	})

	if err := ClearUserConfig(); err != nil {
		t.Fatalf("ClearUserConfig (no user): %v", err)
	}
	if fired {
		t.Error("hook fired despite no active user")
	}
}

func TestRegisterResourceReleaseHook_NilFnIgnored(t *testing.T) {
	t.Cleanup(ResetResourceReleaseHooksForTest)
	RegisterResourceReleaseHook(ResourceReleaseHook{Name: "nil-fn", Fn: nil})
	if got := len(snapshotResourceReleaseHooks()); got != 0 {
		t.Errorf("nil-Fn hook accepted; registry size = %d, want 0", got)
	}
}

// _ asserts the keychain mock is the one we expect, catching API drift.
var _ keychain.KeychainAccess = (*keychain.MockKeychain)(nil)
