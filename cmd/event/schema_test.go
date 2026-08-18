// schema_test.go — coverage for `tmeet event schema`.
//
// Currently focused on the custom Args validator: schema replaced
// cobra.ExactArgs(1) with a hand-rolled validator so the "missing
// EventKey" error can point users at `tmeet event list` and `--help`
// (mirroring the tone of the "unknown EventKey" branch in Run).
//
// We assert three contract points per case:
//   - the error is exception.InvalidArgsError (not a generic cobra error),
//   - the message names `tmeet event list` (discovery hint),
//   - the message names `--help`           (usage hint).
//
// Tests flow through cmd.Execute() rather than calling Run() directly so
// the Args validator is actually exercised — Run() only runs after Args
// passes, so a unit test that bypasses cobra would skip the very layer
// under test.

package event

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"tmeet/internal"
	"tmeet/internal/exception"
)

// expectFriendlyArgsError asserts the three contract points listed in the
// file header.  Centralised so each test stays focused on the unique
// argv it passes.
func expectFriendlyArgsError(t *testing.T, err error, label string) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s: expected error, got nil", label)
	}
	if !exception.Is(err, exception.InvalidArgsError) {
		t.Errorf("%s: expected InvalidArgsError, got %T: %v", label, err, err)
	}
	msg := err.Error()
	if !strings.Contains(msg, "tmeet event list") {
		t.Errorf("%s: error must hint at 'tmeet event list', got: %q", label, msg)
	}
	if !strings.Contains(msg, "--help") {
		t.Errorf("%s: error must hint at '--help', got: %q", label, msg)
	}
}

// runSchemaArgs builds the real cobra command via newSchemaCmd and runs
// Execute() with the given argv.  internal.Tmeet is zero-valued because
// the Args validator runs strictly before RunE, so no field is touched.
//
// SilenceUsage / SilenceErrors keep Execute() from echoing cobra's
// default usage block to our captured stderr — we test the returned
// error, not the printed banner.
func runSchemaArgs(t *testing.T, argv []string) error {
	t.Helper()
	c := newSchemaCmd(&internal.Tmeet{CLIVersion: "test"})
	c.SetOut(&bytes.Buffer{})
	c.SetErr(&bytes.Buffer{})
	c.SetContext(context.Background())
	c.SilenceUsage = true
	c.SilenceErrors = true
	c.SetArgs(argv)
	return c.Execute()
}

func TestSchema_Args_MissingPositionalGivesFriendlyHint(t *testing.T) {
	err := runSchemaArgs(t, []string{})
	expectFriendlyArgsError(t, err, "event schema (no args)")
}

func TestSchema_Args_TooManyPositionalGivesFriendlyHint(t *testing.T) {
	err := runSchemaArgs(t, []string{"meeting.started", "extra.arg"})
	expectFriendlyArgsError(t, err, "event schema (2 args)")
}
