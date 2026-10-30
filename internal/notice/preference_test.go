package notice

import (
	"testing"
	"time"
)

// TestLoadPreference_ColdReturnsEmpty: a brand new account has no file
// on disk. loadPreference MUST return a non-nil, empty Preference so
// callers do not have to nil-check.
func TestLoadPreference_ColdReturnsEmpty(t *testing.T) {
	setupTestConfigDir(t)
	p := LoadPreference("user_a")
	if p == nil {
		t.Fatalf("expected non-nil Preference, got nil")
	}
	if p.Value != "" {
		t.Fatalf("expected empty Value for cold cache, got %q", p.Value)
	}
}

// TestSaveLoadPreference_RoundTrip verifies the write/read cycle and that
// the saved UpdatedAt is captured within a reasonable window of "now".
func TestSaveLoadPreference_RoundTrip(t *testing.T) {
	setupTestConfigDir(t)
	before := time.Now().Unix()
	if err := SavePreference("user_a", PreferenceNeverRemind); err != nil {
		t.Fatalf("save: %v", err)
	}
	after := time.Now().Unix()
	p := LoadPreference("user_a")
	if p.Value != PreferenceNeverRemind {
		t.Fatalf("Value = %q, want %q", p.Value, PreferenceNeverRemind)
	}
	if p.UpdatedAt < before || p.UpdatedAt > after {
		t.Fatalf("UpdatedAt=%d outside [%d,%d]", p.UpdatedAt, before, after)
	}
}

// TestSavePreference_Overwrites: `tmeet update set` is designed to toggle
// between remind / never_remind freely, so Save must be an upsert, not
// an append.
func TestSavePreference_Overwrites(t *testing.T) {
	setupTestConfigDir(t)
	_ = SavePreference("user_a", PreferenceNeverRemind)
	if err := SavePreference("user_a", PreferenceRemind); err != nil {
		t.Fatalf("overwrite: %v", err)
	}
	p := LoadPreference("user_a")
	if p.Value != PreferenceRemind {
		t.Fatalf("after overwrite: Value=%q, want %q", p.Value, PreferenceRemind)
	}
}

// TestSavePreference_InvalidOpenId: the OpenId whitelist must reject
// path-traversal attempts before we touch the filesystem.
func TestSavePreference_InvalidOpenId(t *testing.T) {
	setupTestConfigDir(t)
	if err := SavePreference("../evil", PreferenceRemind); err == nil {
		t.Fatalf("expected error for path-traversal openId, got nil")
	}
}

// TestLoadPreference_AccountIsolation: separate openIds live in separate
// directories; `data/<open_id>/preference.json` must not leak between
// accounts even under identical values.
func TestLoadPreference_AccountIsolation(t *testing.T) {
	setupTestConfigDir(t)
	_ = SavePreference("user_a", PreferenceNeverRemind)
	_ = SavePreference("user_b", PreferenceRemind)
	if got := LoadPreference("user_a").Value; got != PreferenceNeverRemind {
		t.Fatalf("user_a leaked: got %q", got)
	}
	if got := LoadPreference("user_b").Value; got != PreferenceRemind {
		t.Fatalf("user_b leaked: got %q", got)
	}
}
