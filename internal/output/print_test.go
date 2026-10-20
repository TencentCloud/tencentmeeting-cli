package output

import (
	"bytes"
	"encoding/json"
	"io"
	"math"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// newTestCmd builds a cobra command tree that mimics the real CLI structure
// needed by print.go: the ROOT command must expose a "format" persistent flag
// (used by GetFormat) and its output/err streams must be redirectable to a
// buffer so we can inspect what got printed.
//
// Returns the child command (typical caller), plus stdout/stderr buffers.
func newTestCmd(t *testing.T, format string) (*cobra.Command, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()

	root := &cobra.Command{Use: "root"}
	root.PersistentFlags().String("format", "", "output format")
	if format != "" {
		if err := root.PersistentFlags().Set("format", format); err != nil {
			t.Fatalf("set format flag: %v", err)
		}
	}

	child := &cobra.Command{Use: "child"}
	root.AddCommand(child)

	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	child.SetOut(&stdout)
	child.SetErr(&stderr)

	return child, &stdout, &stderr
}

// captureStdStream redirects os.Stdout or os.Stderr for the duration of fn
// and returns everything written to it. This is required for the cmd==nil
// branches of PrintInfof / PrintErrorf which write directly to the process
// std streams.
func captureStdStream(t *testing.T, which string, fn func()) string {
	t.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}

	var orig **os.File
	switch which {
	case "stdout":
		orig = &os.Stdout
	case "stderr":
		orig = &os.Stderr
	default:
		t.Fatalf("unknown stream %q", which)
	}
	saved := *orig
	*orig = w
	defer func() { *orig = saved }()

	done := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		done <- buf.String()
	}()

	fn()

	_ = w.Close()
	return <-done
}

// -----------------------------------------------------------------------------
// marshalNoEscape
// -----------------------------------------------------------------------------

func TestMarshalNoEscape(t *testing.T) {
	t.Run("does not escape HTML special chars", func(t *testing.T) {
		v := map[string]string{"url": "https://x.com/a?b=1&c=2<d>"}
		b, err := marshalNoEscape(v, false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got := string(b)
		for _, tok := range []string{"&", "<", ">"} {
			if !strings.Contains(got, tok) {
				t.Errorf("expected raw %q in output, got %q", tok, got)
			}
		}
		for _, bad := range []string{`\u0026`, `\u003c`, `\u003e`} {
			if strings.Contains(got, bad) {
				t.Errorf("expected NOT to contain escape %q, got %q", bad, got)
			}
		}
	})

	t.Run("no trailing newline in compact mode", func(t *testing.T) {
		b, err := marshalNoEscape(map[string]int{"a": 1}, false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if strings.HasSuffix(string(b), "\n") {
			t.Errorf("compact output should not end with newline, got %q", string(b))
		}
		if got := string(b); got != `{"a":1}` {
			t.Errorf("compact output mismatch: got %q", got)
		}
	})

	t.Run("indent mode produces pretty output without trailing newline", func(t *testing.T) {
		b, err := marshalNoEscape(map[string]int{"a": 1}, true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got := string(b)
		if strings.HasSuffix(got, "\n") {
			t.Errorf("pretty output should not end with trailing newline, got %q", got)
		}
		if !strings.Contains(got, "\n") {
			t.Errorf("pretty output should contain newline between fields, got %q", got)
		}
		if !strings.Contains(got, "  \"a\": 1") {
			t.Errorf("pretty output should be indented with two spaces, got %q", got)
		}
	})

	t.Run("returns error for unmarshalable value", func(t *testing.T) {
		// math.Inf cannot be marshaled by encoding/json.
		if _, err := marshalNoEscape(math.Inf(1), false); err == nil {
			t.Errorf("expected error for +Inf, got nil")
		}
	})
}

// -----------------------------------------------------------------------------
// PrintInfof
// -----------------------------------------------------------------------------

func TestPrintInfof(t *testing.T) {
	t.Run("no args: percent signs are preserved verbatim (regression)", func(t *testing.T) {
		cmd, stdout, _ := newTestCmd(t, "")
		// Content that would previously be corrupted by fmt.Sprintf.
		payload := `{"url":"http://x.com/a%20b%2Fc","progress":"50%","fmt":"%s %d %!(MISSING)"}`
		PrintInfof(cmd, payload)

		got := strings.TrimRight(stdout.String(), "\n")
		if got != payload {
			t.Errorf("percent sign was mangled\n got: %q\nwant: %q", got, payload)
		}
		if strings.Contains(got, "!(MISSING)") && !strings.Contains(payload, "!(MISSING)") {
			t.Errorf("unexpected MISSING marker in output: %q", got)
		}
	})

	t.Run("no args: bare percent verbs are preserved", func(t *testing.T) {
		cmd, stdout, _ := newTestCmd(t, "")
		// Build the format via a variable so `go vet` doesn't complain about
		// the intentional "verb without arg" pattern we want to exercise.
		payload := "raw %s %d %v %%"
		PrintInfof(cmd, payload)
		got := strings.TrimRight(stdout.String(), "\n")
		if got != payload {
			t.Errorf("got %q, want %q", got, payload)
		}
	})

	t.Run("with args: format like fmt.Sprintf", func(t *testing.T) {
		cmd, stdout, _ := newTestCmd(t, "")
		PrintInfof(cmd, "hello %s, id=%d", "world", 42)
		got := strings.TrimRight(stdout.String(), "\n")
		want := "hello world, id=42"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("appends a single trailing newline", func(t *testing.T) {
		cmd, stdout, _ := newTestCmd(t, "")
		PrintInfof(cmd, "abc")
		if got := stdout.String(); got != "abc\n" {
			t.Errorf("got %q, want %q", got, "abc\n")
		}
	})

	t.Run("cmd is nil: falls back to os.Stdout without formatting", func(t *testing.T) {
		payload := `plain 50% %s %!x(MISSING)`
		out := captureStdStream(t, "stdout", func() {
			PrintInfof(nil, payload)
		})
		if strings.TrimRight(out, "\n") != payload {
			t.Errorf("got %q, want %q", out, payload+"\n")
		}
	})

	t.Run("cmd is nil with args: still formats", func(t *testing.T) {
		out := captureStdStream(t, "stdout", func() {
			PrintInfof(nil, "x=%d", 7)
		})
		if strings.TrimRight(out, "\n") != "x=7" {
			t.Errorf("got %q, want %q", out, "x=7\n")
		}
	})
}

// -----------------------------------------------------------------------------
// PrintErrorf
//
// PrintErrorf routes to cmd.ErrOrStderr() (NOT OutOrStderr), so error messages
// go to the ERR stream regardless of whether SetOut is configured. The tests
// below lock that contract in: setting SetOut alone must not divert errors
// onto stdout.
// -----------------------------------------------------------------------------

// newTestErrCmd builds a cobra command tree with independent OUT and ERR
// buffers so tests can assert which stream a message landed on.
func newTestErrCmd(t *testing.T) (cmd *cobra.Command, stdout, stderr *bytes.Buffer) {
	t.Helper()
	root := &cobra.Command{Use: "root"}
	root.PersistentFlags().String("format", "", "output format")
	child := &cobra.Command{Use: "child"}
	root.AddCommand(child)

	var out, err bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&err)
	child.SetOut(&out)
	child.SetErr(&err)
	return child, &out, &err
}

func TestPrintErrorf(t *testing.T) {
	t.Run("routes to stderr, not stdout, even when SetOut is configured", func(t *testing.T) {
		cmd, stdout, stderr := newTestErrCmd(t)
		PrintErrorf(cmd, "boom")
		if stdout.Len() != 0 {
			t.Errorf("stdout must be empty for errors, got %q", stdout.String())
		}
		got := strings.TrimRight(stderr.String(), "\n")
		if got != "Error: boom" {
			t.Errorf("got %q, want %q", got, "Error: boom")
		}
	})

	t.Run("no args: percent signs are preserved (regression)", func(t *testing.T) {
		cmd, _, stderr := newTestErrCmd(t)
		payload := `failed at 50% - path %2F broken %!(MISSING)`
		PrintErrorf(cmd, payload)
		got := strings.TrimRight(stderr.String(), "\n")
		want := "Error: " + payload
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
		if strings.Contains(got, "%!x(MISSING)") {
			t.Errorf("unexpected fmt MISSING marker in output: %q", got)
		}
	})

	t.Run("with args: format like fmt.Sprintf", func(t *testing.T) {
		cmd, _, stderr := newTestErrCmd(t)
		PrintErrorf(cmd, "code=%d reason=%s", 500, "oops")
		got := strings.TrimRight(stderr.String(), "\n")
		want := "Error: code=500 reason=oops"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("no SetErr configured: falls back to os.Stderr", func(t *testing.T) {
		root := &cobra.Command{Use: "root"}
		root.PersistentFlags().String("format", "", "output format")
		cmd := &cobra.Command{Use: "child"}
		root.AddCommand(cmd)

		out := captureStdStream(t, "stderr", func() {
			PrintErrorf(cmd, "boom %d", 1)
		})
		if strings.TrimRight(out, "\n") != "Error: boom 1" {
			t.Errorf("got %q, want %q", out, "Error: boom 1\n")
		}
	})

	t.Run("cmd is nil: falls back to os.Stderr with Error: prefix", func(t *testing.T) {
		payload := `oh no 50%`
		out := captureStdStream(t, "stderr", func() {
			PrintErrorf(nil, payload)
		})
		want := "Error: " + payload + "\n"
		if out != want {
			t.Errorf("got %q, want %q", out, want)
		}
	})
}

// -----------------------------------------------------------------------------
// FormatPrint
// -----------------------------------------------------------------------------

// decodeFormatPrint parses the JSON emitted by FormatPrint back into a
// formatOutput-like structure so tests can assert on the semantic content
// instead of brittle string equality.
type decodedOutput struct {
	TraceId string      `json:"trace_id,omitempty"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	Hints   []string    `json:"hints,omitempty"`
}

func mustDecode(t *testing.T, raw string) decodedOutput {
	t.Helper()
	var out decodedOutput
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatalf("output is not valid JSON: %v\nraw=%q", err, raw)
	}
	return out
}

func TestFormatPrint(t *testing.T) {
	t.Run("default format is compact JSON with newline", func(t *testing.T) {
		cmd, stdout, _ := newTestCmd(t, "")
		FormatPrint(cmd, "trace-1", "ok", `{"a":1}`)

		raw := stdout.String()
		if !strings.HasSuffix(raw, "\n") {
			t.Errorf("output should end with a newline, got %q", raw)
		}
		trimmed := strings.TrimRight(raw, "\n")
		// Compact JSON must be a single line.
		if strings.Contains(trimmed, "\n") {
			t.Errorf("default (json) output should be single-line, got %q", trimmed)
		}

		got := mustDecode(t, trimmed)
		if got.TraceId != "trace-1" || got.Message != "ok" {
			t.Errorf("trace/message mismatch: %+v", got)
		}
		dataMap, ok := got.Data.(map[string]interface{})
		if !ok {
			t.Fatalf("expected data as object, got %T (%v)", got.Data, got.Data)
		}
		if v, _ := dataMap["a"].(float64); v != 1 {
			t.Errorf("expected data.a=1, got %v", dataMap["a"])
		}
	})

	t.Run("json-pretty produces multi-line indented output", func(t *testing.T) {
		cmd, stdout, _ := newTestCmd(t, "json-pretty")
		FormatPrint(cmd, "trace-2", "ok", `{"a":1}`)

		trimmed := strings.TrimRight(stdout.String(), "\n")
		if !strings.Contains(trimmed, "\n") {
			t.Errorf("pretty output should span multiple lines, got %q", trimmed)
		}
		if !strings.Contains(trimmed, "  \"trace_id\": \"trace-2\"") {
			t.Errorf("pretty output should be indented, got %q", trimmed)
		}
		// Even pretty output must be valid JSON.
		_ = mustDecode(t, trimmed)
	})

	t.Run("invalid JSON data is kept as a raw string in the data field", func(t *testing.T) {
		cmd, stdout, _ := newTestCmd(t, "")
		FormatPrint(cmd, "t", "m", "not-a-json")

		got := mustDecode(t, strings.TrimRight(stdout.String(), "\n"))
		s, ok := got.Data.(string)
		if !ok {
			t.Fatalf("expected data as string, got %T (%v)", got.Data, got.Data)
		}
		if s != "not-a-json" {
			t.Errorf("got %q, want %q", s, "not-a-json")
		}
	})

	t.Run("regression: percent signs in message and data survive", func(t *testing.T) {
		cmd, stdout, _ := newTestCmd(t, "")
		msg := "进度 50% 完成 %s %!(MISSING)"
		data := `{"url":"http://x.com/a%20b%2Fc","note":"100% done %d"}`
		FormatPrint(cmd, "trace-%v", msg, data)

		raw := strings.TrimRight(stdout.String(), "\n")

		// Must never leak fmt's MISSING marker.
		if strings.Contains(raw, "%!(") && !strings.Contains(msg, "%!(") && !strings.Contains(data, "%!(") {
			t.Errorf("output contains fmt MISSING marker: %q", raw)
		}
		// The specific injected "%!(MISSING)" is part of the message payload
		// itself; make sure it round-trips instead of being consumed.
		if !strings.Contains(raw, "%!(MISSING)") {
			t.Errorf("literal %%!(MISSING) from message payload was dropped: %q", raw)
		}

		got := mustDecode(t, raw)
		if got.TraceId != "trace-%v" {
			t.Errorf("trace_id got %q, want %q", got.TraceId, "trace-%v")
		}
		if got.Message != msg {
			t.Errorf("message got %q, want %q", got.Message, msg)
		}
		dataMap, ok := got.Data.(map[string]interface{})
		if !ok {
			t.Fatalf("expected data as object, got %T (%v)", got.Data, got.Data)
		}
		if v, _ := dataMap["url"].(string); v != "http://x.com/a%20b%2Fc" {
			t.Errorf("url got %q, want %q", v, "http://x.com/a%20b%2Fc")
		}
		if v, _ := dataMap["note"].(string); v != "100% done %d" {
			t.Errorf("note got %q, want %q", v, "100% done %d")
		}
	})

	t.Run("HTML special chars are not escaped", func(t *testing.T) {
		cmd, stdout, _ := newTestCmd(t, "")
		data := `{"html":"<a href=\"?x=1&y=2\">go</a>"}`
		FormatPrint(cmd, "t", "m", data)

		raw := stdout.String()
		for _, bad := range []string{`\u0026`, `\u003c`, `\u003e`} {
			if strings.Contains(raw, bad) {
				t.Errorf("output should not contain escape %q, got %q", bad, raw)
			}
		}
		for _, want := range []string{"<a href=", "&y=2", "</a>"} {
			if !strings.Contains(raw, want) {
				t.Errorf("output should contain raw %q, got %q", want, raw)
			}
		}
	})

	t.Run("WithHints option is honored", func(t *testing.T) {
		cmd, stdout, _ := newTestCmd(t, "")
		FormatPrint(cmd, "t", "m", `{"a":1}`, WithHints(func(_ string) []string {
			return []string{"hint-A", "hint-B"}
		}))

		got := mustDecode(t, strings.TrimRight(stdout.String(), "\n"))
		if len(got.Hints) != 2 || got.Hints[0] != "hint-A" || got.Hints[1] != "hint-B" {
			t.Errorf("hints mismatch: %+v", got.Hints)
		}
	})

	t.Run("empty traceId and message are omitted from JSON", func(t *testing.T) {
		cmd, stdout, _ := newTestCmd(t, "")
		FormatPrint(cmd, "", "", `{"a":1}`)

		raw := strings.TrimRight(stdout.String(), "\n")
		if strings.Contains(raw, "trace_id") {
			t.Errorf("empty trace_id should be omitted, got %q", raw)
		}
		if strings.Contains(raw, "\"message\"") {
			t.Errorf("empty message should be omitted, got %q", raw)
		}
		if !strings.Contains(raw, "\"data\"") {
			t.Errorf("data should be present, got %q", raw)
		}
	})

	t.Run("unknown format value falls back to compact JSON", func(t *testing.T) {
		cmd, stdout, _ := newTestCmd(t, "yaml") // not supported
		FormatPrint(cmd, "t", "m", `{"a":1}`)

		trimmed := strings.TrimRight(stdout.String(), "\n")
		if strings.Contains(trimmed, "\n") {
			t.Errorf("unknown format should fall back to compact JSON, got %q", trimmed)
		}
		_ = mustDecode(t, trimmed) // must still be valid JSON
	})
}

// -----------------------------------------------------------------------------
// GetFormat
// -----------------------------------------------------------------------------

func TestGetFormat(t *testing.T) {
	tests := []struct {
		name       string
		flagValue  string
		setFlag    bool
		wantFormat string
	}{
		{name: "flag not set -> default json", setFlag: false, wantFormat: "json"},
		{name: "flag set to empty -> default json", setFlag: true, flagValue: "", wantFormat: "json"},
		{name: "flag set to json", setFlag: true, flagValue: "json", wantFormat: "json"},
		{name: "flag set to json-pretty", setFlag: true, flagValue: "json-pretty", wantFormat: "json-pretty"},
		{name: "flag set to arbitrary value is returned as-is", setFlag: true, flagValue: "yaml", wantFormat: "yaml"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := &cobra.Command{Use: "root"}
			root.PersistentFlags().String("format", "", "output format")
			if tt.setFlag {
				if err := root.PersistentFlags().Set("format", tt.flagValue); err != nil {
					t.Fatalf("set flag: %v", err)
				}
			}
			child := &cobra.Command{Use: "child"}
			root.AddCommand(child)

			if got := GetFormat(child); got != tt.wantFormat {
				t.Errorf("GetFormat = %q, want %q", got, tt.wantFormat)
			}
		})
	}

	t.Run("missing flag on root returns default json without panic", func(t *testing.T) {
		// No "format" persistent flag registered at all.
		root := &cobra.Command{Use: "root"}
		child := &cobra.Command{Use: "child"}
		root.AddCommand(child)

		if got := GetFormat(child); got != "json" {
			t.Errorf("GetFormat = %q, want %q", got, "json")
		}
	})
}
