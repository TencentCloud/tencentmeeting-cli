// exit.go — exit-code-2 plumbing for `event status` / `event stop`.
//
// `event status` / `event stop` require a non-zero, non-1 exit code when
// health-check flags (e.g. `event status --fail-on-orphan`) detect anomalies,
// so callers can distinguish "bus is healthy" (0) from "bus is in trouble" (2)
// from "the command itself failed to run" (1).
//
// The root cobra dispatcher (cmd/root.go) collapses all RunE errors to
// exit code 1, so we cannot signal exit 2 by returning a sentinel error.
// Instead, status / stop call exitWithCodeAfterJSON which:
//
//   1. Writes the final JSON output (so jq pipelines still get clean stdout).
//   2. Writes any final stderr hints.
//   3. Calls os.Exit(2) — bypassing root's exit-code-1 normalisation.
//
// This is admittedly a hard exit, but it's confined to two commands whose
// exit-code contract is part of their public API.  Tests substitute
// exitFunc with a panic-style sentinel (see exit_test_hook.go) so
// refused/errored paths can be asserted without taking down the test
// process.

package event

import (
	"os"

	"github.com/spf13/cobra"

	"tmeet/internal/output"
)

// exitCodeOrphan is the conventional code for "bus is in a bad state but
// the command ran successfully".
const exitCodeOrphan = 2

// exitFunc is the process-terminating call exitWithCodeAfterJSON uses to
// signal a non-zero exit code.  Defaulting to os.Exit keeps production
// behaviour unchanged; tests assign a panic-based stub via
// SetExitFuncForTest so they can recover and inspect the requested code.
//
// Made a var (not a const-like reference) so the assignment from tests
// is straightforward and the cost is one indirection per stop/status
// invocation, which is unmeasurable next to the IPC + JSON encoding the
// helper just performed.
var exitFunc = os.Exit

// exitWithCodeAfterJSON serialises v to stdout, optionally writes hint to
// stderr, and exits with the given non-zero code.  Returns nil when code==0
// so callers can chain it as `return exitWithCodeAfterJSON(...)`.
//
// The function intentionally does NOT defer log.Close() or any other root
// teardown — root's defer chain is what we want to skip.  Logs flushed via
// os.Stderr remain unbuffered.
func exitWithCodeAfterJSON(cmd *cobra.Command, v interface{}, hint string, code int) error {
	if err := output.EventPrint(cmd, v); err != nil {
		// If we can't even print the JSON, fall back to a plain exit-1 path
		// via cobra so the user sees the encoding error.
		return err
	}
	if hint != "" {
		output.EventStderr(cmd, "%s", hint)
	}
	if code == 0 {
		return nil
	}
	// Suppress cobra's "Error:" preamble — we've already emitted structured
	// output the consumer cares about.
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	exitFunc(code)
	return nil // unreachable in production; tests' replacement panics so
	// this `return` is similarly unreachable under test.
}
