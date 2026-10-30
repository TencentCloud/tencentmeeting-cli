package notice

import (
	"context"

	"tmeet/internal"
)

// This file exposes the thin, well-named facade that `cmd/update` and
// `internal/output` rely on. Keeping these in a dedicated file (rather
// than scattered across cache.go / client.go / preference.go) makes the
// public surface of the notice package trivial to audit — everything
// exported lives either here or in types.go.

// BuildWithOutcome mirrors Build but also returns the server-advertised
// notice_gap (seconds) alongside a bool telling the caller whether to
// mark the sentinel after a successful render.
//
// Returns (nil, 0, false) whenever no proactive notice should be shown,
// including the throttled "still inside the gap window" case. Callers
// that only need the payload can keep using Build.
//
// The one case where info != nil but markOnEmit == false is the feedback
// command targeted at a never_remind user: it is a product-driven re-
// prompt that must not consume the regular path's gap budget. See
// builder.go for the full decision table.
func BuildWithOutcome(ctx context.Context, tmeet *internal.Tmeet) (info *UpdateInfo, noticeGap int64, markOnEmit bool) {
	return buildWithOutcome(ctx, tmeet)
}

// MarkNotified records that a proactive `_notice.update` was just
// emitted for openId, arming the throttle gate for the next noticeGap
// seconds (defaultNoticeGap when noticeGap <= 0). Best-effort: errors
// are swallowed because the only visible consequence is at most one
// extra prompt later.
//
// Call this ONLY after the payload has been written into the response
// envelope, so a notice that never actually reaches stdout does not
// shift the window forward.
func MarkNotified(openId string, noticeGap int64) {
	markNotified(openId, noticeGap)
}

// ResetNoticeGate drops the proactive-notice sentinel for openId. Called
// by `tmeet update run` after a successful install so a stale "update
// available" nudge from the previous gap window cannot survive past the
// upgrade. Idempotent — a delete on a missing entry is a no-op.
//
// This is the ONLY local state `cmd/update` resets; there is no longer
// a version-info cache to invalidate (every fetch hits the server), so
// the gate sentinel is the sole piece of per-account notice state.
func ResetNoticeGate(openId string) error {
	return clearLastNotice(openId)
}

// LoadPreference returns the user's locally saved preference for openId.
// Never nil; an "unset" state is encoded as a Preference with empty Value.
func LoadPreference(openId string) *Preference {
	return loadPreference(openId)
}

// SavePreference persists value as the user's preference for openId. The
// value is expected to be one of PreferenceRemind / PreferenceNeverRemind —
// callers (notably `tmeet update set`) MUST validate before invoking.
func SavePreference(openId, value string) error {
	return savePreference(openId, value)
}

// FetchFromAPI performs a single live call to /v1/cli/version-info and
// returns the decoded payload. There is no longer a local version-info
// cache; `tmeet update {check,run}` call this directly so the user
// always acts on the freshest server state.
func FetchFromAPI(ctx context.Context, tmeet *internal.Tmeet) (*CachedPayload, error) {
	return fetchFromAPI(ctx, tmeet)
}

// ReportPreference uploads the just-chosen preference value to the server.
// Failure is non-fatal from the user's perspective; callers in `tmeet
// update set` log the error but still exit 0 because the local preference
// has already been persisted.
func ReportPreference(ctx context.Context, tmeet *internal.Tmeet, value string) error {
	return reportPreference(ctx, tmeet, value)
}

// AssembleForCheck is a helper used by `tmeet update check` to render a
// CheckInfo for the given payload + user preference, without hitting the
// network.
//
// It routes through buildForCheck (NOT buildForResponse) because check is
// a user-initiated query: the never_remind preference, which silences the
// proactive response-path prompt, must not silence an explicit query —
// otherwise the user has no way to revisit their choice. The returned
// payload is intentionally minimal (current_version, target_version,
// available, preference) so the Agent receives the raw facts and no
// embedded prompt.
//
// The *CheckInfo return type (as opposed to *UpdateInfo) is deliberate:
// prompt payloads and query results are different domain concepts and
// are kept in separate schemas to avoid conflating their field semantics.
//
// Returns a populated CheckInfo whenever a server payload is available —
// the "is there an upgrade?" verdict is encoded in CheckInfo.Available
// rather than by returning nil, so the caller can show a consistent
// shape (current + target + available) in every outcome, including
// "already on the latest" and "blacklisted". Returns nil ONLY when the
// payload itself is missing (p == nil); the caller may still want to
// surface current_version from its own state in that case.
func AssembleForCheck(current string, p *CachedPayload, pref *Preference) *CheckInfo {
	return buildForCheck(current, p, pref)
}
