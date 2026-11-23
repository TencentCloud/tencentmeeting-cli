package flagalias

import (
	"bytes"
	"strings"
	"testing"
	"tmeet/internal/exception"

	"github.com/spf13/cobra"
)

// newTestCmd builds a minimal cobra command with a --query string flag so
// the alias tests can exercise Bind without pulling in real business logic.
func newTestCmd() (*cobra.Command, *string) {
	var query string
	cmd := &cobra.Command{
		Use: "search",
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}
	cmd.Flags().StringVar(&query, "query", "", "search keyword")
	return cmd, &query
}

// runWithArgv executes cmd with the given argv tokens, stubbing argvFn so
// the conflict detector observes exactly these tokens rather than the real
// process argv (which `go test` populates with test-runner flags).
func runWithArgv(t *testing.T, cmd *cobra.Command, argv []string) error {
	t.Helper()
	orig := argvFn
	argvFn = func() []string { return argv }
	t.Cleanup(func() { argvFn = orig })

	// cobra.Command.Execute reads from cmd's internal args field rather
	// than os.Args; the flag tokens here drive pflag parsing while argvFn
	// supplies the same tokens to our conflict detector.
	cmd.SetArgs(argv)
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	return cmd.Execute()
}

// Baseline: supplying the canonical flag directly keeps working after Bind.
func TestBind_CanonicalPassthrough(t *testing.T) {
	cmd, query := newTestCmd()
	Bind(cmd, []Spec{{Canonical: "query", Aliases: []string{"keyword"}}})

	if err := runWithArgv(t, cmd, []string{"--query", "foo"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if *query != "foo" {
		t.Fatalf("query = %q, want %q", *query, "foo")
	}
}

// Core behaviour: an alias is silently rewritten to the canonical flag,
// its value lands on the canonical variable, and no deprecation prompt is
// emitted on either stdout or stderr.
func TestBind_AliasRewrittenSilently(t *testing.T) {
	cmd, query := newTestCmd()
	Bind(cmd, []Spec{{Canonical: "query", Aliases: []string{"keyword"}}})

	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)

	orig := argvFn
	argv := []string{"--keyword", "foo"}
	argvFn = func() []string { return argv }
	t.Cleanup(func() { argvFn = orig })
	cmd.SetArgs(argv)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if *query != "foo" {
		t.Fatalf("query = %q, want %q", *query, "foo")
	}
	if out.Len() != 0 || errOut.Len() != 0 {
		t.Fatalf("expected silent alias handling, got stdout=%q stderr=%q",
			out.String(), errOut.String())
	}
}

// --key=value form is handled identically to the separated form.
func TestBind_AliasEqualsForm(t *testing.T) {
	cmd, query := newTestCmd()
	Bind(cmd, []Spec{{Canonical: "query", Aliases: []string{"keyword"}}})

	if err := runWithArgv(t, cmd, []string{"--keyword=foo"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if *query != "foo" {
		t.Fatalf("query = %q, want %q", *query, "foo")
	}
}

// The alias must not appear anywhere in the help text.
func TestBind_AliasHiddenFromHelp(t *testing.T) {
	cmd, _ := newTestCmd()
	Bind(cmd, []Spec{{Canonical: "query", Aliases: []string{"keyword"}}})

	help := cmd.UsageString()
	if !strings.Contains(help, "--query") {
		t.Fatalf("canonical flag missing from help: %s", help)
	}
	if strings.Contains(help, "--keyword") {
		t.Fatalf("alias leaked into help: %s", help)
	}
}

// Supplying an alias and its canonical together must be rejected with an
// "unknown flag" error that names the alias, not the canonical.
func TestBind_AliasAndCanonicalConflict(t *testing.T) {
	cmd, _ := newTestCmd()
	Bind(cmd, []Spec{{Canonical: "query", Aliases: []string{"keyword"}}})

	err := runWithArgv(t, cmd, []string{"--query", "a", "--keyword", "b"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !exception.Is(err, exception.InvalidArgsError) {
		t.Fatalf("expected InvalidArgsError, got %v", err)
	}
	if !strings.Contains(err.Error(), "unknown flag: --keyword") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

// Multiple aliases bind to the same canonical; any single alias works, but
// any two distinct spellings in the same invocation (even two aliases) is
// a conflict.
func TestBind_MultipleAliases(t *testing.T) {
	t.Run("alias_kw", func(t *testing.T) {
		cmd, query := newTestCmd()
		Bind(cmd, []Spec{{Canonical: "query", Aliases: []string{"keyword", "kw"}}})
		if err := runWithArgv(t, cmd, []string{"--kw", "foo"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if *query != "foo" {
			t.Fatalf("query = %q, want %q", *query, "foo")
		}
	})

	t.Run("two_aliases_conflict", func(t *testing.T) {
		cmd, _ := newTestCmd()
		Bind(cmd, []Spec{{Canonical: "query", Aliases: []string{"keyword", "kw"}}})
		err := runWithArgv(t, cmd, []string{"--keyword", "a", "--kw", "b"})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !exception.Is(err, exception.InvalidArgsError) {
			t.Fatalf("expected InvalidArgsError, got %v", err)
		}
		// Either alias is acceptable as the offender; the important bit is
		// that the error points at an alias, not the canonical.
		if strings.Contains(err.Error(), "--query") ||
			!strings.Contains(err.Error(), "unknown flag: --") {
			t.Fatalf("unexpected error message: %v", err)
		}
	})
}

// detectConflict stops scanning after a bare "--" so a positional value
// that looks like a flag name cannot trigger a false conflict.
func TestDetectConflict_StopsAtTerminator(t *testing.T) {
	alias := map[string]string{"keyword": "query"}
	err := detectConflict([]string{"--query", "a", "--", "--keyword"}, alias)
	if err != nil {
		t.Fatalf("expected no conflict after --, got %v", err)
	}
}

// Short-form shorthand flags must not be considered by the alias scan.
func TestDetectConflict_IgnoresShorthand(t *testing.T) {
	alias := map[string]string{"keyword": "query"}
	err := detectConflict([]string{"-q", "a", "--keyword", "b"}, alias)
	if err != nil {
		t.Fatalf("shorthand must not count as canonical, got %v", err)
	}
}

// A previously installed PreRunE is preserved and still invoked when no
// alias conflict is present.
func TestBind_PreservesExistingPreRunE(t *testing.T) {
	cmd, _ := newTestCmd()
	called := false
	cmd.PreRunE = func(c *cobra.Command, args []string) error {
		called = true
		return nil
	}
	Bind(cmd, []Spec{{Canonical: "query", Aliases: []string{"keyword"}}})

	if err := runWithArgv(t, cmd, []string{"--keyword", "foo"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("pre-existing PreRunE was not invoked")
	}
}

// Misconfiguration must fail fast via panic at Bind time.
func TestBind_PanicsOnMisconfig(t *testing.T) {
	cases := []struct {
		name  string
		setup func() (*cobra.Command, []Spec)
	}{
		{
			name: "unknown_canonical",
			setup: func() (*cobra.Command, []Spec) {
				cmd, _ := newTestCmd()
				return cmd, []Spec{{Canonical: "nope", Aliases: []string{"x"}}}
			},
		},
		{
			name: "empty_alias",
			setup: func() (*cobra.Command, []Spec) {
				cmd, _ := newTestCmd()
				return cmd, []Spec{{Canonical: "query", Aliases: []string{""}}}
			},
		},
		{
			name: "alias_equals_canonical",
			setup: func() (*cobra.Command, []Spec) {
				cmd, _ := newTestCmd()
				return cmd, []Spec{{Canonical: "query", Aliases: []string{"query"}}}
			},
		},
		{
			name: "alias_collides_with_existing_flag",
			setup: func() (*cobra.Command, []Spec) {
				cmd, _ := newTestCmd()
				var other string
				cmd.Flags().StringVar(&other, "existing", "", "")
				return cmd, []Spec{{Canonical: "query", Aliases: []string{"existing"}}}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r == nil {
					t.Fatal("expected panic, got none")
				}
			}()
			cmd, specs := tc.setup()
			Bind(cmd, specs)
		})
	}
}
