// event.go — output helpers tailored to the `tmeet event` command family.
//
// The event command family emits a *bare* JSON array/object on stdout (so
// downstream `jq` consumers can use direct paths like `.[].key`), and a
// prefix-free, Agent-grep-able stderr line for diagnostics like
// "[event] ready event_key=...".  Neither contract fits FormatPrint's
// {trace_id, message, data} envelope or PrintErrorf's hard-coded "Error:"
// preamble, so this file ships dedicated entry points.
//
// Naming convention: the `Event` prefix is a discoverability hint — when
// adding a new `tmeet event ...` subcommand, grep for "output.Event" and
// you find every channel the family writes to.  Other command families
// should keep using FormatPrint / PrintInfof / PrintErrorf.

package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
)

// EventPrint writes v to cmd.OutOrStdout() as raw JSON (no envelope),
// honouring the inherited --format flag via GetFormat.
//
// "json-pretty" → indented; anything else → compact.  A trailing newline is
// always appended so consumers reading line-by-line (jq, NDJSON tools) see
// a complete record.
//
// opts is the EventOption variadic — currently no built-in options ship
// (see event_options.go).  Callers that don't need to tweak anything pass
// no opts and the call site reads identically to a plain JSON write.
// Returns an error only on JSON encoding or IO failure; callers typically
// propagate it back to cobra as the command's RunE result.
func EventPrint(cmd *cobra.Command, v interface{}, opts ...EventOption) error {
	msg := &eventOptionsMsg{cmd: cmd}
	applyEventOptions(msg, opts...)

	var (
		buf []byte
		err error
	)
	switch GetFormat(cmd) {
	case "json-pretty":
		buf, err = json.MarshalIndent(v, "", "  ")
	default:
		buf, err = json.Marshal(v)
	}
	if err != nil {
		return fmt.Errorf("encode output: %w", err)
	}
	w := cmd.OutOrStdout()
	if _, err = w.Write(buf); err != nil {
		return err
	}
	_, err = io.WriteString(w, "\n")
	return err
}

// EventStderr writes a single newline-terminated line to cmd's stderr stream
// without any prefix.  Use this for informational diagnostics that must
// not pollute stdout — Agent / `jq` pipelines depend on stdout being
// machine-clean.
//
// For surfaced exceptions where an "Error:" preamble is appropriate, use
// PrintErrorf instead.
func EventStderr(cmd *cobra.Command, format string, args ...interface{}) {
	var w io.Writer = os.Stderr
	if cmd != nil {
		if cw := cmd.ErrOrStderr(); cw != nil {
			w = cw
		}
	}
	_, _ = fmt.Fprintf(w, format+"\n", args...)
}
