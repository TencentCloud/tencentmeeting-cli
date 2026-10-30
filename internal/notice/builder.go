package notice

import (
	"context"
	"strings"

	"tmeet/internal"
)

// commandRunUpdate is the fixed command string surfaced in every UpdateInfo
// payload. Agents should execute `tmeet update run` to perform the upgrade.
const commandRunUpdate = "tmeet update run"

// feedbackCmdSuffix is the trailing fragment of cmd.CommandPath() for the
// feedback invocation. Matching on a suffix (rather than an exact string)
// keeps the detection resilient to a possible future rename of the root
// command binary (e.g. tmeet → tmeet-cli).
const feedbackCmdSuffix = "tshoot feedback"

// isFeedbackInvocation reports whether the active command is the feedback
// subcommand, by inspecting the CmdPath stashed on *Tmeet by root.go's
// PersistentPreRunE. We prefer this over a ctx marker because:
//   - the information is already available on *Tmeet,
//   - it survives context derivation inside middleware chains,
//   - callers in cmd/tshoot don't have to remember to tag the context.
func isFeedbackInvocation(tmeet *internal.Tmeet) bool {
	if tmeet == nil {
		return false
	}
	return strings.HasSuffix(tmeet.CmdPath, feedbackCmdSuffix)
}

// Build is the backwards-compatible facade kept so existing call sites
// that only need the payload (and don't care about the "please mark me as
// notified" signal) do not have to change. Internally it just unwraps the
// BuildWithOutcome result.
//
// New callers in internal/output rely on the full BuildWithOutcome tuple
// so they can refresh the throttle sentinel after a successful render.
func Build(ctx context.Context, tmeet *internal.Tmeet) *UpdateInfo {
	info, _, _ := buildWithOutcome(ctx, tmeet)
	return info
}

// buildWithOutcome is the single orchestrator for every proactive-notice
// decision. Its contract:
//
//	returns (info, noticeGap, markOnEmit)
//	  - info == nil         → caller must NOT render anything.
//	  - info != nil         → caller SHOULD render the payload as
//	                          `_notice.update`. If markOnEmit is true,
//	                          the caller MUST follow up with
//	                          markNotified(openId, noticeGap) so the
//	                          gap window starts counting.
//	  - markOnEmit == false → the sole case is the "feedback command AND
//	                          preference == never_remind" branch, which
//	                          is a product-driven re-prompt that must
//	                          NOT consume the regular path's gap budget.
//
// Flow:
//  1. Fast-reject when there's no way we could prompt: no tmeet, no user,
//     dev build, or empty CLI version.
//  2. Load the local preference (empty == "unset").
//  3. Throttle gate: for every non-feedback-never_remind path, if the
//     keyLastNotice sentinel is still fresh, short-circuit to nil WITHOUT
//     hitting the network. This is the whole point of the gap feature:
//     commands inside the window do no server work at all.
//  4. Second short-circuit: regular (non-feedback) + never_remind users
//     never see a prompt, so skip the fetch entirely. Net reduction vs.
//     the old 24h-cache scheme is 1 fetch / 24h for every never_remind
//     user.
//  5. Fetch the live version-info payload — the "always get the freshest
//     info before prompting" half of the spec.
//  6. Route to buildForFeedback or buildForResponse. Attach markOnEmit=true
//     unless we're on the feedback+never_remind exception path.
//
// buildWithOutcome never returns errors: a fetch failure, a corrupted
// cache, an invalid openId — all degrade to (nil, 0, false). The caller
// then simply omits `_notice.update` for this invocation and tries again
// next time.
func buildWithOutcome(ctx context.Context, tmeet *internal.Tmeet) (*UpdateInfo, int64, bool) {
	if tmeet == nil || tmeet.UserConfig == nil {
		return nil, 0, false
	}
	openId := tmeet.UserConfig.OpenId
	if openId == "" {
		return nil, 0, false
	}
	current := tmeet.CLIVersion
	if current == "" || current == "dev" {
		// Local/dev builds never prompt. The server-side version-info API
		// would also typically fail to resolve a channel for them, so we
		// short-circuit before even hitting the network.
		return nil, 0, false
	}

	pref := loadPreference(openId)
	feedback := isFeedbackInvocation(tmeet)
	// The one exception that bypasses BOTH the throttle gate AND the
	// never_remind short-circuit: a feedback command for a user who has
	// previously opted out. The feedback path is designed to re-engage
	// those users, so it must always have a shot at them.
	isFeedbackNeverRemind := feedback && pref != nil && pref.Value == PreferenceNeverRemind

	// Gate 1: still within the previous notice's gap window → stay silent.
	// Checked before any network call so throttled commands pay no
	// latency cost beyond a tiny stat() on the sentinel file.
	if !isFeedbackNeverRemind && isNoticeFresh(openId) {
		return nil, 0, false
	}

	// Gate 2: regular path + never_remind → the user has told us never
	// to prompt them again on the response channel, so there is no
	// outcome in which we would emit anything. Skip the fetch to avoid
	// gratuitously calling the version-info API for users who will
	// never consume its result.
	if !feedback && pref != nil && pref.Value == PreferenceNeverRemind {
		return nil, 0, false
	}

	// Only now do we talk to the server. By the time execution reaches
	// here, either we're outside the gap window (regular / feedback-
	// unset / feedback-remind) or we're the feedback-never_remind
	// exception — in all cases the server's answer is actionable.
	payload, err := fetchFromAPI(ctx, tmeet)
	if err != nil || payload == nil {
		return nil, 0, false
	}

	if feedback {
		info := buildForFeedback(current, payload, pref)
		if info == nil {
			return nil, 0, false
		}
		if isFeedbackNeverRemind {
			// Exception: emit the payload but do NOT consume the gap
			// budget. markNotified is deliberately skipped so an
			// immediately-following business command still gets its own
			// chance to prompt (which it will, because gap wasn't armed).
			return info, 0, false
		}
		// feedback-unset / feedback-remind fall back internally to the
		// regular shape (see buildForFeedback); they consume the gap
		// budget on the same terms as the response path.
		return info, payload.NoticeGap, true
	}

	info := buildForResponse(current, payload, pref)
	if info == nil {
		return nil, 0, false
	}
	return info, payload.NoticeGap, true
}

// buildForResponse implements the regular (non-feedback) output policy.
//
// Filter order (any match → return nil):
//  1. Blacklist hit.
//  2. Preference == never_remind — regular path is suppressed.
//  3. target_version empty OR current already >= target — nothing to prompt.
//
// Shape rules once we pass the filter:
//   - Preference unset  → full payload with 2-choice prompt.
//   - Preference remind → compact payload (4 fields, no choices).
func buildForResponse(current string, p *CachedPayload, pref *Preference) *UpdateInfo {
	if p == nil || p.BlacklistHit {
		return nil
	}
	if pref != nil && pref.Value == PreferenceNeverRemind {
		return nil
	}
	if !IsNewer(current, p.TargetVersion) {
		return nil
	}

	switch pref.Value {
	case PreferenceRemind:
		// Explicit opt-in: user already picked, don't re-ask. Only ship
		// the four core fields the Agent needs to render the hint.
		return &UpdateInfo{
			CurrentVersion: current,
			TargetVersion:  p.TargetVersion,
			Message:        p.RemindPreferenceText.TextMessage,
			Command:        commandRunUpdate,
		}
	default:
		// Preference unset → full payload with the 2 choices rendered by
		// the server (text + command literal come straight from the API).
		return &UpdateInfo{
			CurrentVersion:        current,
			TargetVersion:         p.TargetVersion,
			Message:               p.NoPreferenceText.TextMessage,
			Command:               commandRunUpdate,
			PreferenceInstruction: p.NoPreferenceText.TextPreferenceInstruction,
			Choices:               cloneChoices(p.NoPreferenceText.Choices),
		}
	}
}

// cloneChoices defensively copies the slice we hand back to the caller so a
// downstream mutation (JSON marshalling never mutates, but future code
// might) cannot corrupt the payload shared across calls.
func cloneChoices(src []Choice) []Choice {
	if len(src) == 0 {
		return nil
	}
	out := make([]Choice, len(src))
	copy(out, src)
	return out
}

// buildForCheck implements the output policy for `tmeet update check`.
//
// Unlike buildForResponse, this path is a *user-initiated query*: the user
// explicitly asked whether a new version exists, so a locally saved
// never_remind preference MUST NOT suppress the result (never_remind
// controls proactive prompts, not explicit queries — otherwise the user
// is locked out of ever revisiting their choice).
//
// Also unlike buildForResponse, this path does NOT return nil just because
// "there's nothing to prompt". The user asked a question; they deserve an
// answer, including a definitive "you're already on the latest" or
// "you're blacklisted". We therefore always construct a CheckInfo as long
// as we have a current version to report, and encode the three possible
// outcomes via the Available boolean:
//
//   - Available=true  → an upgrade is both newer AND permitted.
//   - Available=false AND current < target → blacklist hit (upgrade exists
//     but is not permitted). TargetVersion is still shown so the user
//     knows what's being withheld.
//   - Available=false AND current >= target → already on the latest.
//
// nil is reserved for the degenerate case of a completely missing cached
// payload (p == nil), where we have no server-side information to project
// at all. Even then, the caller is free to still surface current_version
// via its own fallback; buildForCheck simply refuses to invent a wire
// shape without any server input.
//
// Preference is the raw stored value ("remind" / "never_remind" / "" for
// unset). The empty case is dropped by json omitempty, which doubles as
// the on-the-wire signal for "no preference yet".
//
// Returns *CheckInfo (NOT *UpdateInfo) because check is a query, not a
// prompt — see the CheckInfo doc for the type-split rationale.
func buildForCheck(current string, p *CachedPayload, pref *Preference) *CheckInfo {
	if p == nil {
		return nil
	}
	var prefValue string
	if pref != nil {
		prefValue = pref.Value
	}
	// Available = "there exists a strictly newer version AND the server
	// permits this account to move to it". Blacklist is a server-side
	// veto on actionability, not on visibility, so TargetVersion is still
	// surfaced below regardless of its value.
	available := !p.BlacklistHit && IsNewer(current, p.TargetVersion)
	return &CheckInfo{
		CurrentVersion: current,
		TargetVersion:  p.TargetVersion,
		Available:      available,
		Preference:     prefValue,
	}
}
