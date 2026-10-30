package notice

import "testing"

// Feedback branch hard-stops: blacklist takes precedence over the whole
// feedback-specific behaviour.
func TestBuildForFeedback_BlacklistSuppresses(t *testing.T) {
	p := newCachedPayload()
	p.BlacklistHit = true
	pref := &Preference{Value: PreferenceNeverRemind}
	if info := buildForFeedback("v1.0.19", p, pref); info != nil {
		t.Fatalf("blacklist must take precedence over feedback branch, got %+v", info)
	}
}

// Already-latest short-circuits on the feedback branch too.
func TestBuildForFeedback_AlreadyLatestSuppresses(t *testing.T) {
	p := newCachedPayload()
	pref := &Preference{Value: PreferenceNeverRemind}
	if info := buildForFeedback("v1.0.20", p, pref); info != nil {
		t.Fatalf("already-latest must suppress, got %+v", info)
	}
}

// Nil payload: nothing to say. Same contract as the regular path.
func TestBuildForFeedback_NilPayload(t *testing.T) {
	if info := buildForFeedback("v1.0.19", nil, &Preference{Value: PreferenceNeverRemind}); info != nil {
		t.Fatalf("nil payload should yield nil, got %+v", info)
	}
}

// Unset preference on the feedback branch defers to regular rules → the
// Message must come from the regular path's "no preference" text block,
// not from feedback_text.
func TestBuildForFeedback_UnsetFallsBackToRegular(t *testing.T) {
	p := newCachedPayload()
	info := buildForFeedback("v1.0.19", p, &Preference{})
	if info == nil {
		t.Fatalf("expected regular-rule output")
	}
	if info.Message != "please upgrade" {
		t.Fatalf("Message=%q, want regular path message", info.Message)
	}
	// The regular unset path returns 2 choices.
	if len(info.Choices) != 2 {
		t.Fatalf("want 2 regular-path choices, got %d", len(info.Choices))
	}
}

// remind on the feedback branch defers to the regular remind shape (4
// fields, no choices).
func TestBuildForFeedback_RemindFallsBackToRegular(t *testing.T) {
	p := newCachedPayload()
	info := buildForFeedback("v1.0.19", p, &Preference{Value: PreferenceRemind})
	if info == nil {
		t.Fatalf("expected regular-rule output")
	}
	if info.Message != "a newer version is ready" {
		t.Fatalf("Message=%q", info.Message)
	}
	if info.Choices != nil {
		t.Fatalf("remind path must omit choices, got %+v", info.Choices)
	}
	if info.PreferenceInstruction != "" {
		t.Fatalf("remind path must omit instruction, got %q", info.PreferenceInstruction)
	}
}

// never_remind on the feedback branch returns the feedback payload with
// the choices intact AND the never_remind choice's Command stripped
// (keep closed = no command).
func TestBuildForFeedback_NeverRemind_StripsNeverRemindCommand(t *testing.T) {
	p := newCachedPayload()
	info := buildForFeedback("v1.0.19", p, &Preference{Value: PreferenceNeverRemind})
	if info == nil {
		t.Fatalf("expected feedback payload")
	}
	if info.Message != "feedback filed; also an upgrade is available" {
		t.Fatalf("Message=%q", info.Message)
	}
	if info.PreferenceInstruction != "reopen prompts?" {
		t.Fatalf("Instruction=%q", info.PreferenceInstruction)
	}
	if len(info.Choices) != 2 {
		t.Fatalf("want 2 feedback choices, got %d", len(info.Choices))
	}

	var remind, neverRemind *Choice
	for i := range info.Choices {
		switch info.Choices[i].Value {
		case PreferenceRemind:
			remind = &info.Choices[i]
		case PreferenceNeverRemind:
			neverRemind = &info.Choices[i]
		}
	}
	if remind == nil || remind.Command == "" {
		t.Fatalf("remind choice must retain command, got %+v", remind)
	}
	if neverRemind == nil {
		t.Fatalf("never_remind choice missing")
	}
	if neverRemind.Command != "" {
		t.Fatalf("never_remind command must be stripped, got %q", neverRemind.Command)
	}
}

// Defence-in-depth: stripNeverRemindCommand must leave an already-blank
// Command alone and must only touch the never_remind entry.
func TestStripNeverRemindCommand_PreservesOthers(t *testing.T) {
	choices := []Choice{
		{Value: PreferenceRemind, Command: "keep-me"},
		{Value: PreferenceNeverRemind, Command: "strip-me"},
		{Value: "custom", Command: "untouched"},
	}
	stripNeverRemindCommand(choices)
	if choices[0].Command != "keep-me" {
		t.Fatalf("remind got mutated: %+v", choices[0])
	}
	if choices[1].Command != "" {
		t.Fatalf("never_remind not stripped: %+v", choices[1])
	}
	if choices[2].Command != "untouched" {
		t.Fatalf("unrelated value mutated: %+v", choices[2])
	}
}
