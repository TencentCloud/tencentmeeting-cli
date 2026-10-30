package cmdutil

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// newFlagRulesTestCmd builds a minimal cobra.Command carrying the flags
// referenced across the FlagRules test cases below. It also pre-registers
// --action so Dispatch has a flag to read against; tests set only the
// flags they care about to Changed=true.
func newFlagRulesTestCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("action", "", "")
	cmd.Flags().String("record-file-id", "", "")
	cmd.Flags().String("share-url", "", "")
	cmd.Flags().String("text", "", "")
	cmd.Flags().String("meeting-id", "", "")
	cmd.Flags().String("pid", "", "")
	cmd.Flags().String("limit", "", "")
	cmd.Flags().String("page-token", "", "")
	cmd.Flags().String("page-size", "", "")
	cmd.Flags().String("lang", "", "")
	cmd.Flags().String("pwd", "", "")
	return cmd
}

// runOK is a no-op Run suitable for handlers whose business logic is
// irrelevant to the test (we only care about Dispatch's FlagRules
// enforcement).
func runOK(cmd *cobra.Command) error { return nil }

func mustSetFlag(t *testing.T, cmd *cobra.Command, name, value string) {
	t.Helper()
	if err := cmd.Flags().Set(name, value); err != nil {
		t.Fatalf("set --%s = %q failed: %v", name, value, err)
	}
}

// -----------------------------------------------------------------------
// Unconditional rules (When == nil)
// -----------------------------------------------------------------------

func TestDispatch_Unconditional_NotTriggered(t *testing.T) {
	cmd := newFlagRulesTestCmd()
	reg := ActionRegistry{
		"foo": {
			FlagRules: []FlagRule{
				{Reject: []string{"text", "pid"}},
			},
			Run: runOK,
		},
	}
	// No disallowed flag is set; Dispatch must succeed.
	if err := reg.Dispatch(cmd, "foo", "action"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDispatch_Unconditional_Triggered(t *testing.T) {
	cmd := newFlagRulesTestCmd()
	mustSetFlag(t, cmd, "text", "hello")
	reg := ActionRegistry{
		"foo": {
			FlagRules: []FlagRule{
				{Reject: []string{"text", "pid"}},
			},
			Run: runOK,
		},
	}
	err := reg.Dispatch(cmd, "foo", "action")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	want := "--text is not supported by --action foo"
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("error = %q; want contains %q", err.Error(), want)
	}
	// Must NOT carry the (form: ...) suffix for unconditional rules.
	if strings.Contains(err.Error(), "(form:") {
		t.Fatalf("unconditional error must not carry (form: ...): %q", err.Error())
	}
}

func TestDispatch_Unconditional_MultipleIsInternalError(t *testing.T) {
	cmd := newFlagRulesTestCmd()
	reg := ActionRegistry{
		"foo": {
			FlagRules: []FlagRule{
				{Reject: []string{"text"}},
				{Reject: []string{"pid"}}, // second unconditional rule - programmer error
			},
			Run: runOK,
		},
	}
	err := reg.Dispatch(cmd, "foo", "action")
	if err == nil {
		t.Fatal("expected internal error for multiple unconditional FlagRules, got nil")
	}
	if !strings.Contains(err.Error(), "internal:") {
		t.Fatalf("error should be flagged as internal: %q", err.Error())
	}
	if !strings.Contains(err.Error(), "multiple unconditional FlagRules") {
		t.Fatalf("error should mention multiple unconditional FlagRules: %q", err.Error())
	}
}

// -----------------------------------------------------------------------
// Conditional rules (When != nil)
// -----------------------------------------------------------------------

func TestDispatch_Conditional_NoneMatched(t *testing.T) {
	cmd := newFlagRulesTestCmd()
	mustSetFlag(t, cmd, "pid", "123") // would be rejected only if a matching When fires
	reg := ActionRegistry{
		"foo": {
			FlagRules: []FlagRule{
				{
					When:   func(cmd *cobra.Command) bool { return false },
					Form:   "foo(x)",
					Reject: []string{"pid"},
				},
			},
			Run: runOK,
		},
	}
	// No conditional rule matches; --pid is NOT rejected.
	if err := reg.Dispatch(cmd, "foo", "action"); err != nil {
		t.Fatalf("unexpected error when no conditional matches: %v", err)
	}
}

func TestDispatch_Conditional_OneMatched_CarriesFormSuffix(t *testing.T) {
	cmd := newFlagRulesTestCmd()
	mustSetFlag(t, cmd, "page-size", "5")
	reg := ActionRegistry{
		"minutes": {
			FlagRules: []FlagRule{
				{
					When:   func(cmd *cobra.Command) bool { return true },
					Form:   "minutes(file)",
					Reject: []string{"page-token", "page-size"},
				},
			},
			Run: runOK,
		},
	}
	err := reg.Dispatch(cmd, "minutes", "action")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	want := "--page-size is not supported by --action minutes (form: minutes(file))"
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("error = %q; want contains %q", err.Error(), want)
	}
}

func TestDispatch_Conditional_MultipleMatchedIsInternalError(t *testing.T) {
	cmd := newFlagRulesTestCmd()
	reg := ActionRegistry{
		"transcript": {
			FlagRules: []FlagRule{
				{
					When:   func(cmd *cobra.Command) bool { return true },
					Form:   "transcript(url)",
					Reject: []string{"pid"},
				},
				{
					When:   func(cmd *cobra.Command) bool { return true },
					Form:   "transcript(search)",
					Reject: []string{"limit"},
				},
			},
			Run: runOK,
		},
	}
	err := reg.Dispatch(cmd, "transcript", "action")
	if err == nil {
		t.Fatal("expected internal error for multiple matching conditional rules, got nil")
	}
	if !strings.Contains(err.Error(), "internal:") {
		t.Fatalf("error should be flagged as internal: %q", err.Error())
	}
	if !strings.Contains(err.Error(), "multiple matching conditional FlagRules") {
		t.Fatalf("error should mention multiple matching conditional FlagRules: %q", err.Error())
	}
	// The error should name both matching forms to aid debugging.
	if !strings.Contains(err.Error(), "transcript(url)") ||
		!strings.Contains(err.Error(), "transcript(search)") {
		t.Fatalf("error should name both matched forms: %q", err.Error())
	}
}

// -----------------------------------------------------------------------
// Mixed unconditional + conditional
// -----------------------------------------------------------------------

func TestDispatch_UnconditionalCheckedBeforeConditional(t *testing.T) {
	cmd := newFlagRulesTestCmd()
	// Set a flag rejected by the unconditional rule AND a flag rejected
	// by the matching conditional rule. The unconditional pass should win
	// (reports first), because Dispatch runs Pass 1 before Pass 2.
	mustSetFlag(t, cmd, "lang", "zh")     // rejected by unconditional rule
	mustSetFlag(t, cmd, "page-size", "5") // rejected by conditional rule
	reg := ActionRegistry{
		"transcript": {
			FlagRules: []FlagRule{
				// Unconditional: action-level
				{Reject: []string{"lang", "pwd"}},
				// Conditional: transcript(details)
				{
					When:   func(cmd *cobra.Command) bool { return true },
					Form:   "transcript(details)",
					Reject: []string{"page-token", "page-size"},
				},
			},
			Run: runOK,
		},
	}
	err := reg.Dispatch(cmd, "transcript", "action")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	// Must be the action-level error (no form suffix), not the form-level one.
	if !strings.Contains(err.Error(), "--lang is not supported by --action transcript") {
		t.Fatalf("unconditional error should be reported first: %q", err.Error())
	}
	if strings.Contains(err.Error(), "(form:") {
		t.Fatalf("unconditional error must not carry (form: ...): %q", err.Error())
	}
}

// -----------------------------------------------------------------------
// Edge cases
// -----------------------------------------------------------------------

func TestDispatch_EmptyFlagRulesSkipped(t *testing.T) {
	cmd := newFlagRulesTestCmd()
	mustSetFlag(t, cmd, "text", "hello")
	reg := ActionRegistry{
		"foo": {Run: runOK}, // no FlagRules at all
	}
	if err := reg.Dispatch(cmd, "foo", "action"); err != nil {
		t.Fatalf("empty FlagRules should accept any flag combination: %v", err)
	}
}

func TestDispatch_RejectUnknownFlagIsIgnored(t *testing.T) {
	cmd := newFlagRulesTestCmd()
	reg := ActionRegistry{
		"foo": {
			FlagRules: []FlagRule{
				// "no-such-flag" is not registered on cmd; must not panic
				// nor report a false positive.
				{Reject: []string{"no-such-flag"}},
			},
			Run: runOK,
		},
	}
	if err := reg.Dispatch(cmd, "foo", "action"); err != nil {
		t.Fatalf("unknown flag names in Reject must be silently ignored: %v", err)
	}
}

func TestDispatch_UnknownActionReportsExpected(t *testing.T) {
	cmd := newFlagRulesTestCmd()
	reg := ActionRegistry{
		"foo": {Run: runOK},
		"bar": {Run: runOK},
	}
	err := reg.Dispatch(cmd, "zoo", "action")
	if err == nil {
		t.Fatal("expected error for unknown action, got nil")
	}
	if !strings.Contains(err.Error(), `invalid --action "zoo"`) {
		t.Fatalf("error should name the invalid value: %q", err.Error())
	}
	// Should list the known actions in sorted order.
	if !strings.Contains(err.Error(), "bar, foo") {
		t.Fatalf("error should list known actions in sorted order: %q", err.Error())
	}
}

func TestDispatch_NilRunIsInternalError(t *testing.T) {
	cmd := newFlagRulesTestCmd()
	reg := ActionRegistry{
		"foo": {Run: nil}, // programmer error
	}
	err := reg.Dispatch(cmd, "foo", "action")
	if err == nil {
		t.Fatal("expected internal error for nil Run, got nil")
	}
	if !strings.Contains(err.Error(), "internal:") {
		t.Fatalf("error should be flagged as internal: %q", err.Error())
	}
}
