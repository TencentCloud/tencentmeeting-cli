package notice

import (
	"context"
	"testing"

	"tmeet/internal"
	"tmeet/internal/config"
)

// -----------------------------------------------------------------------------
// buildForResponse — all five filters + the three preference shapes.
// -----------------------------------------------------------------------------

// newCachedPayload is a convenient fixture; tests pick fields to override.
func newCachedPayload() *CachedPayload {
	return &CachedPayload{
		TargetVersion: "v1.0.20",
		TargetCDN:     "https://example.com/tmeet",
		NoPreferenceText: PreferenceText{
			TextMessage:               "please upgrade",
			TextPreferenceInstruction: "pick one",
			Choices: []Choice{
				{Value: PreferenceRemind, Label: "remind me", Command: "tmeet update set --preference remind"},
				{Value: PreferenceNeverRemind, Label: "stop", Command: "tmeet update set --preference never_remind"},
			},
		},
		RemindPreferenceText: PreferenceText{
			TextMessage: "a newer version is ready",
		},
		FeedbackText: PreferenceText{
			TextMessage:               "feedback filed; also an upgrade is available",
			TextPreferenceInstruction: "reopen prompts?",
			Choices: []Choice{
				{Value: PreferenceRemind, Label: "reopen", Command: "tmeet update set --preference remind"},
				// server-side MAY ship a command; the CLI strips it defensively.
				{Value: PreferenceNeverRemind, Label: "stay closed", Command: "should-be-stripped"},
			},
		},
	}
}

// Filter 1: no payload at all → nil.
func TestBuildForResponse_NilPayload(t *testing.T) {
	if info := buildForResponse("v1.0.19", nil, &Preference{}); info != nil {
		t.Fatalf("expected nil, got %+v", info)
	}
}

// Filter 2: blacklist_hit → nil regardless of preference / version.
func TestBuildForResponse_BlacklistSuppresses(t *testing.T) {
	p := newCachedPayload()
	p.BlacklistHit = true
	if info := buildForResponse("v1.0.19", p, &Preference{}); info != nil {
		t.Fatalf("blacklist should suppress, got %+v", info)
	}
}

// Filter 3: preference == never_remind on the regular path → nil.
func TestBuildForResponse_NeverRemindSuppresses(t *testing.T) {
	p := newCachedPayload()
	pref := &Preference{Value: PreferenceNeverRemind}
	if info := buildForResponse("v1.0.19", p, pref); info != nil {
		t.Fatalf("never_remind should suppress on regular path, got %+v", info)
	}
}

// Filter 4: current == target (nothing newer) → nil.
func TestBuildForResponse_AlreadyLatest(t *testing.T) {
	p := newCachedPayload()
	if info := buildForResponse("v1.0.20", p, &Preference{}); info != nil {
		t.Fatalf("expected nil when already on target, got %+v", info)
	}
}

// Filter 4b: current newer than target → nil (also exercises the ordering
// strand of IsNewer).
func TestBuildForResponse_AheadOfTarget(t *testing.T) {
	p := newCachedPayload()
	if info := buildForResponse("v2.0.0", p, &Preference{}); info != nil {
		t.Fatalf("expected nil when ahead of target, got %+v", info)
	}
}

// Shape 1: preference unset → full payload with 2 choices + instruction.
func TestBuildForResponse_UnsetFullPayload(t *testing.T) {
	p := newCachedPayload()
	info := buildForResponse("v1.0.19", p, &Preference{})
	if info == nil {
		t.Fatalf("expected non-nil")
	}
	if info.CurrentVersion != "v1.0.19" || info.TargetVersion != "v1.0.20" {
		t.Fatalf("versions: %+v", info)
	}
	if info.Command != commandRunUpdate {
		t.Fatalf("Command=%q, want %q", info.Command, commandRunUpdate)
	}
	if info.Message != "please upgrade" {
		t.Fatalf("Message=%q", info.Message)
	}
	if info.PreferenceInstruction != "pick one" {
		t.Fatalf("Instruction=%q", info.PreferenceInstruction)
	}
	if len(info.Choices) != 2 {
		t.Fatalf("want 2 choices, got %d", len(info.Choices))
	}
}

// Shape 2: preference == remind → compact 4-field payload only.
func TestBuildForResponse_RemindCompactPayload(t *testing.T) {
	p := newCachedPayload()
	info := buildForResponse("v1.0.19", p, &Preference{Value: PreferenceRemind})
	if info == nil {
		t.Fatalf("expected non-nil")
	}
	if info.Message != "a newer version is ready" {
		t.Fatalf("Message=%q (should pull from RemindPreferenceText)", info.Message)
	}
	if info.PreferenceInstruction != "" {
		t.Fatalf("remind path must omit instruction, got %q", info.PreferenceInstruction)
	}
	if info.Choices != nil {
		t.Fatalf("remind path must omit choices, got %+v", info.Choices)
	}
	if info.Command != commandRunUpdate {
		t.Fatalf("Command=%q", info.Command)
	}
}

// cloneChoices must defensively copy. Mutating the returned slice must
// not corrupt the shared cache payload.
func TestBuildForResponse_ChoicesAreCloned(t *testing.T) {
	p := newCachedPayload()
	info := buildForResponse("v1.0.19", p, &Preference{})
	if info == nil || len(info.Choices) == 0 {
		t.Fatalf("precondition failed")
	}
	info.Choices[0].Label = "mutated"
	if p.NoPreferenceText.Choices[0].Label == "mutated" {
		t.Fatalf("payload Choices leaked through — defensive copy missing")
	}
}

// -----------------------------------------------------------------------------
// Build — CmdPath-driven feedback routing + "no tmeet / no user"
// short-circuits.
// -----------------------------------------------------------------------------

// Build must gracefully no-op when the Tmeet / UserConfig / version
// pre-conditions are not met. This keeps FormatPrint's call site simple.
func TestBuild_ShortCircuits(t *testing.T) {
	ctx := context.Background()

	if info := Build(ctx, nil); info != nil {
		t.Fatalf("nil tmeet should yield nil, got %+v", info)
	}

	t1 := &internal.Tmeet{} // nil UserConfig
	if info := Build(ctx, t1); info != nil {
		t.Fatalf("nil UserConfig should yield nil, got %+v", info)
	}

	t2 := &internal.Tmeet{UserConfig: &config.UserConfig{}} // empty openId
	if info := Build(ctx, t2); info != nil {
		t.Fatalf("empty openId should yield nil, got %+v", info)
	}

	t3 := &internal.Tmeet{
		UserConfig: &config.UserConfig{OpenId: "user_a"},
		CLIVersion: "dev",
	}
	if info := Build(ctx, t3); info != nil {
		t.Fatalf("dev build should yield nil, got %+v", info)
	}

	t4 := &internal.Tmeet{
		UserConfig: &config.UserConfig{OpenId: "user_a"},
		CLIVersion: "",
	}
	if info := Build(ctx, t4); info != nil {
		t.Fatalf("empty CLI version should yield nil, got %+v", info)
	}
}

// -----------------------------------------------------------------------------
// Pure-function coverage of the feedback branch routing.
//
// Previously these were integration tests that seeded a local
// update.json cache and relied on Build to synthesise the payload
// without hitting the network. The `last_notice.json`-only design has
// no local version-info cache anymore — Build always fetches — so the
// warm-cache fixture is no longer representable. The routing decisions
// those tests exercised still live in the pure helpers (buildForFeedback
// / buildForResponse) and are covered here directly, which is both
// faster and insulated from rest-proxy wiring.
// -----------------------------------------------------------------------------

// TestBuildForFeedback_NeverRemind_StripsCommand: on the feedback
// branch with pref=never_remind, we emit the dedicated FeedbackText
// payload and the never_remind choice must have its Command stripped so
// clicking "stay closed" does not re-execute `tmeet update set`.
func TestBuildForFeedback_NeverRemind_StripsCommand(t *testing.T) {
	p := newCachedPayload()
	info := buildForFeedback("v1.0.19", p, &Preference{Value: PreferenceNeverRemind})
	if info == nil {
		t.Fatalf("expected non-nil feedback payload")
	}
	if info.Message != "feedback filed; also an upgrade is available" {
		t.Fatalf("Message=%q (should pull from FeedbackText)", info.Message)
	}
	if len(info.Choices) != 2 {
		t.Fatalf("want 2 feedback choices, got %d", len(info.Choices))
	}
	var nr *Choice
	for i := range info.Choices {
		if info.Choices[i].Value == PreferenceNeverRemind {
			nr = &info.Choices[i]
		}
	}
	if nr == nil {
		t.Fatalf("never_remind choice missing from feedback payload")
	}
	if nr.Command != "" {
		t.Fatalf("never_remind choice must omit command, got %q", nr.Command)
	}
}

// TestBuildForFeedback_Remind_FallsBackToRegular: on the feedback
// branch, unset / remind defer to the regular path. We discriminate
// via the Message — if the fallback is missed we'd see FeedbackText
// (the "feedback filed..." line) instead of RemindPreferenceText.
func TestBuildForFeedback_Remind_FallsBackToRegular(t *testing.T) {
	p := newCachedPayload()
	info := buildForFeedback("v1.0.19", p, &Preference{Value: PreferenceRemind})
	if info == nil {
		t.Fatalf("expected non-nil")
	}
	if info.Message != "a newer version is ready" {
		t.Fatalf("expected remind-text message, got %q", info.Message)
	}
}

// TestBuildForFeedback_Unset_FallsBackToRegular: identical rationale to
// the remind case, but with the unset preference. The fallback should
// produce the full "unset" payload (2 choices + instruction).
func TestBuildForFeedback_Unset_FallsBackToRegular(t *testing.T) {
	p := newCachedPayload()
	info := buildForFeedback("v1.0.19", p, &Preference{})
	if info == nil {
		t.Fatalf("expected non-nil")
	}
	if info.PreferenceInstruction != "pick one" {
		t.Fatalf("Instruction=%q (should pull from NoPreferenceText)",
			info.PreferenceInstruction)
	}
	if len(info.Choices) != 2 {
		t.Fatalf("want 2 choices, got %d", len(info.Choices))
	}
}

// Keep imports used; `context`, `internal`, and `config` are still
// consumed by TestBuild_ShortCircuits above.
var _ = context.Background
var _ = (*internal.Tmeet)(nil)
var _ = (*config.UserConfig)(nil)
