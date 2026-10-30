package notice

// buildForFeedback implements the feedback-channel output policy.
//
// Branch matrix:
//
//	Preference  │ Behaviour
//	────────────┼───────────────────────────────────────────────────────
//	unset       │ fall back to the regular rules (no feedback-specific
//	            │ nudging — the user has not opted out so they already
//	            │ get a regular prompt via buildForResponse)
//	remind      │ same as unset — fall back to regular rules.
//	never_remind│ emit the feedback-specific payload: full fields plus
//	            │ two choices, where the "stay opted out" choice carries
//	            │ no command (keep-closed = no command executed).
//
// All regular-path hard-stops (blacklist, dev build, already-latest,
// cache-missed-with-fetch-failure) still apply. In particular the
// blacklist takes precedence over the feedback branch — blacklisted
// users must never be nudged, even on feedback.
func buildForFeedback(current string, p *CachedPayload, pref *Preference) *UpdateInfo {
	if p == nil || p.BlacklistHit {
		return nil
	}
	if !IsNewer(current, p.TargetVersion) {
		return nil
	}

	// Only never_remind travels down the feedback-specific branch; for
	// unset / remind the regular rules already DTRT.
	if pref == nil || pref.Value != PreferenceNeverRemind {
		return buildForResponse(current, p, pref)
	}

	// Feedback-specific payload. We re-emit the full set of fields plus
	// an opt-back-in prompt, mirroring the "feedback_text" bundle the
	// server shipped. The never_remind choice is intentionally rendered
	// WITHOUT a command — keep-closed must not execute anything.
	choices := cloneChoices(p.FeedbackText.Choices)
	stripNeverRemindCommand(choices)

	return &UpdateInfo{
		CurrentVersion:        current,
		TargetVersion:         p.TargetVersion,
		Message:               p.FeedbackText.TextMessage,
		Command:               commandRunUpdate,
		PreferenceInstruction: p.FeedbackText.TextPreferenceInstruction,
		Choices:               choices,
	}
}

// stripNeverRemindCommand enforces the contract that the never_remind
// option in the feedback bundle MUST NOT carry a command. The server is
// expected to ship it empty, but defence in depth keeps the contract
// local to the CLI and makes any server-side regression visible in CLI
// output rather than silently violated.
func stripNeverRemindCommand(choices []Choice) {
	for i := range choices {
		if choices[i].Value == PreferenceNeverRemind {
			choices[i].Command = ""
		}
	}
}
