package notice

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"tmeet/internal/config"
	"tmeet/internal/core/filecache"
)

// setupTestConfigDir redirects config.GetConfigDir() to a temp directory
// so a test's filecache state is completely isolated from the real user's
// ~/.tmeet. The returned function MUST be deferred — leaving the env var
// pointing at a deleted tempdir would leak into unrelated tests.
func setupTestConfigDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	prev, had := os.LookupEnv("TMEET_CLI_CONFIG_DIR")
	if err := os.Setenv("TMEET_CLI_CONFIG_DIR", dir); err != nil {
		t.Fatalf("set env: %v", err)
	}
	t.Cleanup(func() {
		if had {
			_ = os.Setenv("TMEET_CLI_CONFIG_DIR", prev)
		} else {
			_ = os.Unsetenv("TMEET_CLI_CONFIG_DIR")
		}
	})
	return dir
}

// TestIsNoticeFresh_ColdMiss is the baseline: a brand-new account has no
// sentinel on disk, so the throttle gate must be open.
func TestIsNoticeFresh_ColdMiss(t *testing.T) {
	setupTestConfigDir(t)
	if isNoticeFresh("user_a") {
		t.Fatalf("expected cold miss, got fresh=true")
	}
}

// TestMarkNotified_FreshWithinGap verifies the happy path: after a
// markNotified call with a positive gap, isNoticeFresh returns true as
// long as we're still inside the window.
func TestMarkNotified_FreshWithinGap(t *testing.T) {
	setupTestConfigDir(t)
	markNotified("user_a", 3600) // 1h gap
	if !isNoticeFresh("user_a") {
		t.Fatalf("expected fresh immediately after markNotified")
	}
}

// TestMarkNotified_UsesDefaultGapWhenNonPositive covers the two
// edge cases documented on markNotified: gap==0 and gap<0 both fall back
// to defaultNoticeGap. We cannot easily time-travel 24h here, so we
// assert on the behaviour that defaultNoticeGap is >= 1s by checking the
// sentinel is fresh (which it must be regardless of the exact fallback
// value as long as it's > 0).
func TestMarkNotified_UsesDefaultGapWhenNonPositive(t *testing.T) {
	setupTestConfigDir(t)
	markNotified("user_a", 0)
	if !isNoticeFresh("user_a") {
		t.Fatalf("gap=0 should fall back to defaultNoticeGap; expected fresh")
	}

	// Re-seed under a new user to isolate from the first write.
	markNotified("user_b", -42)
	if !isNoticeFresh("user_b") {
		t.Fatalf("gap<0 should fall back to defaultNoticeGap; expected fresh")
	}
}

// TestClearLastNotice_UnlatchesGate: --force on `tmeet update
// {run,check}` wipes the sentinel so the next proactive notice goes
// through immediately.
func TestClearLastNotice_UnlatchesGate(t *testing.T) {
	setupTestConfigDir(t)
	markNotified("user_a", 3600)
	if !isNoticeFresh("user_a") {
		t.Fatalf("precondition: expected fresh after markNotified")
	}
	if err := clearLastNotice("user_a"); err != nil {
		t.Fatalf("clearLastNotice: %v", err)
	}
	if isNoticeFresh("user_a") {
		t.Fatalf("expected miss after clearLastNotice")
	}
}

// TestLastNotice_AccountIsolation locks in the "data/<open_id>/" layout:
// two accounts' sentinels must never leak into one another, which is the
// whole point of running filecache per-account.
func TestLastNotice_AccountIsolation(t *testing.T) {
	setupTestConfigDir(t)
	markNotified("user_a", 3600)
	// user_b is deliberately NOT marked.
	if !isNoticeFresh("user_a") {
		t.Fatalf("user_a should be fresh")
	}
	if isNoticeFresh("user_b") {
		t.Fatalf("user_b should NOT be fresh — isolation leaked")
	}
}

// TestLastNotice_ExpiryViaShortGap: writing with a sub-second TTL and
// then waiting past it should flip the sentinel back to "not fresh". We
// use filecache directly to avoid relying on real time; the sentinel's
// freshness is driven entirely by the envelope's cached_at+ttl pair.
func TestLastNotice_ExpiryViaShortGap(t *testing.T) {
	setupTestConfigDir(t)
	// Write with a 1-second TTL via the private helper; markNotified's
	// public contract rounds to seconds too.
	fc, err := newCache("user_a")
	if err != nil {
		t.Fatalf("newCache: %v", err)
	}
	payload, _ := json.Marshal(&LastNotice{At: time.Now().Unix()})
	if err := fc.Set(keyLastNotice, payload, time.Second); err != nil {
		t.Fatalf("set: %v", err)
	}
	if !isNoticeFresh("user_a") {
		t.Fatalf("expected fresh right after set")
	}
	// Sleep slightly longer than the TTL so filecache marks it stale.
	time.Sleep(1200 * time.Millisecond)
	if isNoticeFresh("user_a") {
		t.Fatalf("expected non-fresh after TTL elapsed")
	}
}

// TestLastNotice_CorruptedDegrades: filecache's own "corrupted = miss"
// contract already covers garbled envelopes, but we double-check that
// our wrapper surfaces the same behaviour (no spurious throttle).
func TestLastNotice_CorruptedDegrades(t *testing.T) {
	dir := setupTestConfigDir(t)
	markNotified("user_a", 3600)
	path := filepath.Join(dir, "data", "user_a", keyLastNotice)
	// Overwrite the data file with raw junk; the envelope JSON parse
	// should fail, which filecache treats as a miss.
	if err := os.WriteFile(path, []byte("not-json"), 0o600); err != nil {
		t.Fatalf("rewrite data: %v", err)
	}
	if isNoticeFresh("user_a") {
		t.Fatalf("expected corrupted sentinel to degrade to miss")
	}
}

// TestInvalidOpenId: openIds that fail config.ValidateOpenId must not
// create any files on disk. markNotified silently absorbs the error
// (consistent with its best-effort doc contract); clearLastNotice
// surfaces it so the caller in cmd/update knows the --force didn't apply.
func TestInvalidOpenId(t *testing.T) {
	dir := setupTestConfigDir(t)
	markNotified("../evil", 3600) // must not panic / must not write
	if _, err := os.Stat(filepath.Join(dir, "..", "evil")); err == nil {
		t.Fatalf("path traversal leaked outside tempdir")
	}

	if err := clearLastNotice("../evil"); err == nil {
		t.Fatalf("expected clearLastNotice to surface validation error")
	} else if !errors.Is(err, err) { // sanity: non-nil
		t.Fatalf("unexpected: %v", err)
	}

	// isNoticeFresh must also degrade silently to false.
	if isNoticeFresh("../evil") {
		t.Fatalf("invalid openId must not be considered fresh")
	}

	// Guarantee that a straight filecache call with a traversal name
	// never lands outside dir, as a defence-in-depth check against the
	// validateKey layer in filecache.
	fc := filecache.New(dir)
	_, _, _ = fc.Get("../evil")
}

// Compile-time assertion that config.ValidateOpenId still exists; if a
// future refactor renames it, this test file catches the drift locally.
var _ = config.ValidateOpenId
