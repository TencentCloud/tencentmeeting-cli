// Package notice builds the `_notice.update` payload that is appended by
// internal/output.FormatPrint to successful command responses, and backs the
// `tmeet update` subcommand family.
//
// The package owns all policy around:
//   - When to prompt a user to upgrade the CLI (local version compare,
//     blacklist flag, local preference, feedback-specific overrides).
//   - How to shape the JSON payload for each of the three preference states
//     (unset, remind, never_remind).
//   - How to throttle proactive prompts so that, within the server-advertised
//     notice_gap window, each account sees at most one `_notice.update`
//     across all commands (feedback+never_remind is the one documented
//     exception — see builder.go).
//   - How to persist the user's update preference "forever".
//
// Nothing in this package depends on internal/output. The dependency arrow
// is deliberately one-way (output -> notice) to avoid import cycles.
package notice

import "time"

// noExpire expresses "no TTL" on top of filecache's TTL-based primitive.
//
// filecache treats ttl <= 0 as "do not write" on Set and as "already expired"
// on Get (see internal/core/filecache/{filecache.go,envelope.go}). We do NOT
// want to patch filecache's core semantics, so for records that must survive
// indefinitely (the user preference) we pass a TTL so large that it is
// equivalent to "never expires" within the lifetime of this software.
//
// 100 years is deliberately comfortable: well beyond any realistic retention
// horizon, well below time.Duration's 290-year signed overflow limit.
const noExpire = 100 * 365 * 24 * time.Hour

// keyLastNotice is the filecache key for the per-account "last proactive
// notice emitted" sentinel. Its mere freshness is the throttle gate: as
// long as this entry is fresh, no further proactive _notice.update will
// be emitted for the account.
//
// The TTL applied at write time is the server-advertised notice_gap
// (falling back to defaultNoticeGap when the server ships 0 / a negative
// value), so there is no separate "last_notified_at" field to compare
// against — the filecache envelope's own TTL IS the window.
const keyLastNotice = "last_notice.json"

// keyPreference is the filecache key for the per-account update preference.
// Stored under the same per-account directory as keyLastNotice.
const keyPreference = "preference.json"

// defaultNoticeGap is the fallback window used when the server does not
// advertise a positive notice_gap. Keeps parity with the historical 24h
// cadence that the retired `update.json` cache implicitly enforced.
const defaultNoticeGap = 24 * time.Hour

// Preference enumerates the legal values for a user's update preference.
// They match the enum accepted by the preference-report API verbatim.
const (
	PreferenceRemind      = "remind"
	PreferenceNeverRemind = "never_remind"
)

// Choice is one entry in the `choices` array of an UpdateInfo payload.
//
// Command is optional: for the feedback-channel "keep never_remind" option
// no command should be executed (the user is simply confirming the existing
// preference), so we omit the field via omitempty.
type Choice struct {
	Value   string `json:"value,omitempty"`
	Label   string `json:"label,omitempty"`
	Command string `json:"command,omitempty"`
}

// UpdateInfo is the JSON payload rendered at response_root._notice.update.
//
// Field shaping rules (see builder.go for the authoritative implementation):
//   - When the user has no preference recorded: all fields populated,
//     Choices has 2 entries (remind + never_remind).
//   - When preference == remind: only CurrentVersion, TargetVersion, Message,
//     Command are populated; PreferenceInstruction and Choices are nil.
//   - When preference == never_remind (regular path): UpdateInfo itself is
//     not emitted (Build returns nil).
//   - When preference == never_remind (feedback path): all fields populated,
//     Choices has 2 entries; the never_remind entry has no Command.
type UpdateInfo struct {
	CurrentVersion        string   `json:"current_version,omitempty"`
	TargetVersion         string   `json:"target_version,omitempty"`
	Message               string   `json:"message,omitempty"`
	Command               string   `json:"command,omitempty"`
	PreferenceInstruction string   `json:"preference_instruction,omitempty"`
	Choices               []Choice `json:"choices,omitempty"`
}

// CheckInfo is the JSON payload rendered by `tmeet update check` as its
// `update` body. It is intentionally a separate type from UpdateInfo
// because the two serve different roles:
//
//   - UpdateInfo is a *prompt*: it rides along with other command
//     responses to nudge the user to upgrade, and its shape is driven by
//     the user's preference (unset / remind / never_remind suppresses or
//     expands the prompt accordingly).
//   - CheckInfo is a *query result*: the user explicitly asked "is there
//     an update?", so the output must be purely informational and MUST
//     NOT be suppressed by a never_remind preference. The current
//     preference value is surfaced so the caller can decide whether to
//     flip it via `tmeet update set`.
//
// Field semantics:
//   - CurrentVersion: the local CLI version. Always populated for a
//     non-dev build.
//   - TargetVersion: the latest version advertised by the server. Empty
//     only when the server payload is missing entirely.
//   - Available: whether an upgrade is actually actionable, i.e.
//     IsNewer(current, target) AND NOT blacklist_hit. Rendered as a plain
//     bool (NO omitempty) so the caller can rely on it being present and
//     unambiguous even when the answer is false.
//   - Preference: the raw stored value ("remind" / "never_remind" / "" for
//     unset). omitempty maps the unset case to a missing field on the wire,
//     which is the agreed signal for "no preference yet".
//
// Note on blacklist: when the server has blacklisted this account/version,
// Available is false but TargetVersion is still populated — the user is
// entitled to see what the latest advertised version is, even if they
// aren't currently allowed to move to it.
type CheckInfo struct {
	CurrentVersion string `json:"current_version,omitempty"`
	TargetVersion  string `json:"target_version,omitempty"`
	Available      bool   `json:"available"`
	Preference     string `json:"preference,omitempty"`
}

// PreferenceText mirrors the per-preference text block returned by the
// version-info API (`no_preference_text` / `remind_preference_text` /
// `feedback_text`). The CLI selects one at render time based on the local
// preference and the ctx branch — the payload is NOT persisted locally;
// it is consumed within a single Build call and discarded.
type PreferenceText struct {
	TextMessage               string   `json:"text_message,omitempty"`
	TextPreferenceInstruction string   `json:"text_preference_instruction,omitempty"`
	Choices                   []Choice `json:"choices,omitempty"`
}

// CachedPayload is the in-memory projection of the /v1/cli/version-info
// response. It is NOT persisted locally anymore — every proactive notice
// path fetches a fresh copy on the fly (gated by the keyLastNotice
// throttle), which guarantees the user always sees the latest server-side
// state once a window elapses.
//
// Unknown fields on the wire are ignored by json.Unmarshal, which is the
// behaviour we want: additive server-side changes do not break old CLIs
// that were built before them.
type CachedPayload struct {
	TargetFromNPM          bool   `json:"target_from_npm,omitempty"`
	TargetVersion          string `json:"target_version,omitempty"`
	TargetCDN              string `json:"target_cdn,omitempty"`
	TargetCDNChecksum      string `json:"target_cdn_checksum,omitempty"`
	TargetSkillCDN         string `json:"target_skill_cdn,omitempty"`
	TargetSkillCDNChecksum string `json:"target_skill_cdn_checksum,omitempty"`
	BlacklistHit           bool   `json:"blacklist_hit,omitempty"`
	// NoticeGap is the server-advertised minimum interval (seconds)
	// between two proactive update notices. A non-positive value falls
	// back to defaultNoticeGap, matching the pre-notice_gap cadence.
	//
	// Used as the TTL applied to keyLastNotice; see markNotified in
	// cache.go.
	NoticeGap            int64          `json:"notice_gap,omitempty"`
	NoPreferenceText     PreferenceText `json:"no_preference_text,omitempty"`
	RemindPreferenceText PreferenceText `json:"remind_preference_text,omitempty"`
	FeedbackText         PreferenceText `json:"feedback_text,omitempty"`
}

// LastNotice is the shape persisted at
// ~/.tmeet/data/<open_id>/last_notice.json.
//
// Content is intentionally minimal: the actual throttle signal is the
// filecache envelope's own freshness (TTL == notice_gap), not any field
// inside this struct. The timestamp is kept purely for diagnostics (e.g.
// a future `tmeet update debug` surface) and is NOT consulted by the
// gating logic.
type LastNotice struct {
	At int64 `json:"at,omitempty"`
}

// Preference is the shape persisted at ~/.tmeet/data/<open_id>/preference.json.
//
// Value is one of PreferenceRemind / PreferenceNeverRemind, or empty when
// the user has not explicitly chosen. UpdatedAt is a Unix epoch second kept
// purely for diagnostics (e.g. `tmeet update get`).
type Preference struct {
	Value     string `json:"value,omitempty"`
	UpdatedAt int64  `json:"updated_at,omitempty"`
}
