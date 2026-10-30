package output

import (
	"sync/atomic"

	"tmeet/internal"
)

// activeTmeet holds the process-wide *Tmeet instance that FormatPrint
// consults when deciding whether to append `_notice.update` to a
// response. It is populated once at CLI startup from cmd/root.go after
// internal.NewTmeet() succeeds, and read on every FormatPrint call.
//
// Design rationale:
//
//   - *Tmeet is a *process-level singleton*: internal.NewTmeet() is
//     invoked exactly once from Execute() and never swapped out while
//     the CLI runs. Threading it through context.Context would conflate
//     "per-request cancellation / deadline / trace" (what context is
//     for) with "per-process service dependency" (what context is
//     explicitly NOT for — see the Go blog's "Contexts and structs").
//
//   - A package-level setter mirrors how the standard library ships
//     http.DefaultClient / log.Default() and how cobra itself exposes
//     cobra.OnFinalize — long-lived process-wide dependencies live on
//     the package, not in each request's context.
//
//   - atomic.Pointer avoids a mutex on the hot read path. FormatPrint
//     is called on every successful business response, which is far
//     hotter than the single Set performed at startup; tests that
//     mutate it do so from a single goroutine. The atomic read is
//     lock-free and the write is a plain pointer-sized store.
//
//   - A nil pointer is the "feature-disabled" signal, mirroring the
//     previous context-based design: when FormatPrint sees nil it just
//     omits `_notice.update`, so unit tests of unrelated packages that
//     never call SetTmeet get a clean, deterministic response shape.
var activeTmeet atomic.Pointer[internal.Tmeet]

// SetTmeet installs t as the process-wide *Tmeet seen by FormatPrint.
//
// Called:
//   - exactly once from cmd/root.go's Execute(), right after a
//     successful internal.NewTmeet(), so the pointer is in place before
//     any subcommand runs; and
//   - from tests that need to override or clear the installed value;
//     passing nil is the documented way to tear the pointer back down
//     between test cases.
//
// Safe for concurrent use with getTmeet.
func SetTmeet(t *internal.Tmeet) {
	activeTmeet.Store(t)
}

// getTmeet returns the installed *Tmeet, or nil when none has been
// registered. The nil case must stay a legitimate "feature disabled"
// signal rather than an error — FormatPrint intentionally stays a
// no-op in that case so callers never have to care about the update
// notice when they only want to print a plain response.
func getTmeet() *internal.Tmeet {
	return activeTmeet.Load()
}
