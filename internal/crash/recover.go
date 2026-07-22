package crash

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"runtime/debug"
	"tmeet/internal"
	"tmeet/internal/exception"
)

// PanicExitCode is the process exit code used when the CLI is terminated
// by an uncaught panic. It is intentionally distinct from the exit code
// used for regular business failures (which is 1) so that operators can
// distinguish crashes from expected errors via the exit code alone.
const PanicExitCode = 2

// maxCrashStackLen caps the length (in bytes) of the crash_stack field
// reported to the server. Anything beyond this is truncated to avoid
// oversized request bodies. The top of a Go panic stack already contains
// the most valuable frames, so head-truncation is sufficient.
const maxCrashStackLen = 8192

// RecoverAndReport handles a panic that has already been recovered by the
// caller's deferred function.
//
// It must be invoked from within a deferred closure that itself calls
// recover(), passing the recovered value in as `r`. Doing the recover()
// here would NOT work: Go's recover() only takes effect when called
// directly inside a deferred function -- one extra level of indirection
// (i.e. defer closure -> RecoverAndReport -> recover()) makes recover()
// return nil and the panic keeps propagating.
//
// Behaviour:
//   - If r == nil (no panic), returns (0, false) and is otherwise a no-op.
//   - Otherwise, captures the stack, reports it to the server, prints a
//     single-line "tmeet crashed: xxx" message to stderr, and returns
//     (PanicExitCode, true). The caller is expected to propagate the exit
//     code (typically via a named return value) so that OTHER defers --
//     most importantly log.Close() -- still get a chance to run before
//     the process exits.
//
// The panic is intentionally NOT re-thrown; the caller does not need to
// worry about propagating it. This function must never call os.Exit
// itself, because that would skip sibling defers such as log.Close().
func RecoverAndReport(ctx context.Context, tmeet *internal.Tmeet, r any) (exitCode int, crashed bool) {
	if r == nil {
		return 0, false
	}

	// Prepend Go runtime info (version/os/arch) so the server can attribute
	// the crash to a specific toolchain / platform without needing an extra
	// body field. OS/arch is also derivable from restproxy headers, but
	// duplicating them here keeps the stack self-contained for offline
	// forensic viewing.
	crashStack := fmt.Sprintf("goversion=%s goos=%s goarch=%s\n%v\n%s",
		runtime.Version(), runtime.GOOS, runtime.GOARCH, r, debug.Stack())
	if len(crashStack) > maxCrashStackLen {
		crashStack = crashStack[:maxCrashStackLen]
	}
	Report(ctx, tmeet, exception.ClientCodePanic, crashStack)

	_, _ = fmt.Fprintf(os.Stderr, "Error: tmeet crashed: %v\n", r)
	return PanicExitCode, true
}
