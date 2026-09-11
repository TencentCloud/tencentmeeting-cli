// Package cleanup provides the resource-release hook the rest of the
// CLI registers via config.RegisterResourceReleaseHook.
//
// Why a sub-package and not a function inside `internal/event` directly?
// `internal/event/busctl` (and `transport`) already import
// `internal/event` for path constants like BusSockPath; adding a hook to
// the parent package that calls back into busctl would form an import
// cycle.  Pulling the hook into a leaf sub-package keeps the dependency
// arrows pointing one way (cleanup → busctl → event).
//
// Wiring point: cmd.Execute() in cmd/root.go calls
//
//	config.RegisterResourceReleaseHook(config.ResourceReleaseHook{
//	    Name: "event-bus",
//	    Fn:   cleanup.OnUserCleared,
//	})
//
// at process start.  config does NOT import this package — registration
// is one-way data flow.  The bus daemon process (`tmeet event _bus`) is
// not affected: its own ClearUserConfig call sites don't exist (it only
// reads tokens, never clears them), so even if the hook were registered
// inside _bus it would never fire.
//
// Sequence (matches the consume contract):
//
//  1. Hash the cleared OpenId via eventruntime.OpenIDHash.
//  2. Read bus.meta — owner mismatch ⇒ noop (don't kill someone else's bus).
//  3. Try graceful Shutdown via the IPC socket.
//  4. If the socket is unreachable but bus.alive.lock is held, the bus is
//     orphaned (process alive but not accepting connections); SIGKILL by
//     PID and scrub bus.sock + bus.meta.
//  5. If neither holds (no socket, no live lock), best-effort scrub any
//     leftover bus.sock / bus.meta and return.
//
// Error contract (NEW — was previously fire-and-forget):
//
// OnUserCleared returns an error iff after exhausting the graceful path
// AND issuing SIGKILL the bus PROCESS is still detectably alive (the
// alive lock remains held AND Ping does not yet report ErrNotRunning).
// The upstream config.ClearUserConfig treats that as logout-failed and
// keeps the user's credentials intact so a retry has the OpenId it
// needs to address the bus again.  All other diagnostics — graceful
// timeout falling back to SIGKILL, scrub IO errors, missing PID — go
// to stderr with the stable `[event] OnUserCleared:` prefix and do NOT
// abort logout: those are recoverable conditions where the bus is in
// fact gone (or never existed) by the time we return.
package cleanup

import (
	"errors"
	"fmt"
	"os"
	"time"

	"tmeet/internal/core/filelock"
	eventruntime "tmeet/internal/event"
	"tmeet/internal/event/busctl"
	"tmeet/internal/event/transport"
)

// shutdownGracePeriod is how long OnUserCleared waits after sending
// Shutdown before falling back to SIGKILL.  Aligned with the documented
// stop --timeout default (10 s) but shorter, because we are blocking the
// logout command and a slow bus exit shouldn't make `tmeet auth logout`
// itself feel sluggish.
const shutdownGracePeriod = 3 * time.Second

// shutdownPollInterval — how often we re-probe Ping while waiting for
// the graceful exit.  Matches stop.go's pingPollInterval to keep the two
// shutdown paths visually consistent.
const shutdownPollInterval = 50 * time.Millisecond

// OnUserCleared is invoked by config.ClearUserConfig with the OpenId of
// the user whose credentials are about to be removed.  Empty input is
// a no-op (defensive — config validates non-empty before reaching us,
// but tests or future call sites might bypass that).
//
// Returns nil iff the bus owned by `openId` is provably gone by the
// time we return — concretely: Ping yields ErrNotRunning AND the alive
// lock is no longer held — OR there was nothing to stop in the first
// place (no meta, owner mismatch, no live lock).  Otherwise an error
// describing the residual state is returned and ClearUserConfig will
// keep the user's credentials intact so the operation can be retried.
//
// Scrub IO failures (bus.sock / bus.meta / bus.pid removal) and stale
// PID lookups are deliberately NOT promoted to errors: they describe
// leftover on-disk artefacts, not a still-running process, and the
// next bus startup will overwrite them.  Surfacing them here would
// fail logout for harmless conditions.
func OnUserCleared(openId string) error {
	if openId == "" {
		return nil
	}

	clearedHash := eventruntime.OpenIDHash(openId)

	// 1. Owner check.  bus.meta absent / corrupt is treated as "no bus to
	//    target", because we can't safely identify the running bus.  Note
	//    that we still fall through to the orphan-cleanup branch below
	//    when `metaPresent==false`, since stale bus.sock could still
	//    linger from a previous SIGKILL.
	meta, metaPresent, _ := eventruntime.ReadBusMeta()
	if metaPresent && meta.OpenIDHash != clearedHash {
		// Different account's bus — leave it alone.  This is the documented
		// double-safety rail in §3.9: even if a logout/login race somehow
		// produced a meta written by a different account, we never touch
		// state we don't own.
		return nil
	}

	tr := transport.New()

	// 2. Graceful shutdown via socket.
	if err := busctl.SendShutdown(tr, false); err == nil {
		// Frame is on the wire — poll until Ping fails AND the alive lock
		// releases.  Same sequence as stop.go's runGraceful but with a
		// shorter grace period.
		if waitForBusExit(tr, shutdownGracePeriod) {
			// Bus exited cleanly; bus.sock / bus.meta cleanup happens on
			// the bus side or stays on disk for the next bus to overwrite
			// (matches §3.8: meta is not deleted on graceful exit).
			return nil
		}
		// Graceful timed out — fall through to orphan cleanup.
		stderrLog("[event] OnUserCleared: graceful shutdown timed out after %s, escalating to force cleanup", shutdownGracePeriod)
	} else if !errors.Is(err, busctl.ErrNotRunning) {
		// Real protocol failure (Dial succeeded but write failed).  Log
		// and continue to the orphan path; we'd rather over-clean than
		// leave state behind.
		stderrLog("[event] OnUserCleared: send shutdown: %v", err)
	}

	// 3. Orphan / not-running path.  Distinguish via the alive lock:
	//    held ⇒ process is alive but unresponsive ⇒ SIGKILL by PID;
	//    free ⇒ no bus is running ⇒ scrub any leftover artefacts.
	if aliveLockProbe() {
		killBusByPID(meta, metaPresent)
		// After SIGKILL we re-verify: only if the bus is now provably
		// gone do we return success.  A still-held alive lock means the
		// kernel hasn't released the OS resources yet OR the kill
		// failed to land — either way logout cannot proceed because the
		// bus is still consuming the about-to-be-deleted credentials.
		if !waitForBusExit(tr, shutdownGracePeriod) {
			return fmt.Errorf("event bus for openId still alive after SIGKILL within %s", shutdownGracePeriod)
		}
	}

	// 4. Always best-effort scrub leftover artefacts.  Idempotent —
	//    Remove returns nil for nonexistent files.  Errors are logged
	//    but do NOT abort logout: residual files describe disk state,
	//    not a live process.
	scrubBusArtefacts()
	return nil
}

// waitForBusExit polls Ping + alive-lock until both indicate the bus has
// fully exited, or until timeout.  Returns true iff the bus exited
// before the deadline.
//
// Why two checks?  The bus closes its accept loop before its main
// goroutine returns, so for a brief window Ping fails but the OS hasn't
// yet released the alive lock; declaring "exit" too early would race
// with `tr.Cleanup(sock)` and produce spurious "address already in use"
// when a new bus tries to start.
func waitForBusExit(tr transport.IPC, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		err := busctl.Ping(tr)
		if errors.Is(err, busctl.ErrNotRunning) && !aliveLockProbe() {
			return true
		}
		time.Sleep(shutdownPollInterval)
	}
	return false
}

// aliveLockProbe returns true iff bus.alive.lock is currently held by
// some process — i.e., a bus is alive.  Mirrors stop.go's aliveLockHeld
// but kept as a separate copy here to avoid pulling cmd/event into
// internal/event/cleanup (which would invert the dependency direction).
func aliveLockProbe() bool {
	probe := filelock.NewProcessLock(eventruntime.BusAliveLock())
	if err := probe.TryLock(); err != nil {
		return errors.Is(err, filelock.ErrHeld)
	}
	_ = probe.Unlock()
	return false
}

// killBusByPID issues SIGKILL to the bus PID recorded in bus.meta (or, as
// a fallback, the PID file).  Best-effort — a missing/recycled PID is
// not surfaced as an error because the OS will soon release the alive
// lock anyway, after which scrubBusArtefacts can complete cleanup.
//
// Implementation lives in cleanup_unix.go / cleanup_windows.go for
// cross-platform support: Windows lacks SIGKILL and uses Process.Kill().
func killBusByPID(meta eventruntime.BusMeta, metaPresent bool) {
	pid := 0
	if metaPresent && meta.PID > 0 {
		pid = meta.PID
	}
	if pid <= 0 {
		// Last-ditch: read bus.pid directly.  We don't import busdiscover
		// here to avoid pulling its filelock probe redundantly; reading
		// the file straight is cheaper.
		if data, err := os.ReadFile(eventruntime.BusPIDFile()); err == nil {
			// First line is the PID per §3.3.  Use a forgiving parse.
			pid = parseFirstIntLine(data)
		}
	}
	if pid <= 0 {
		stderrLog("[event] OnUserCleared: cannot determine bus PID for SIGKILL; relying on OS lock release")
		return
	}
	if err := killProcess(pid); err != nil {
		stderrLog("[event] OnUserCleared: kill pid=%d: %v", pid, err)
	}
}

// scrubBusArtefacts removes bus.sock + bus.meta + bus.pid.  We do NOT
// remove bus.alive.lock or bus.fork.lock — both are 0-byte coordination
// inodes whose semantics depend on flock() being applied; removing them
// races with a concurrent fresh bus's open-then-flock pair.
//
// Idempotent: missing files are silently skipped.  Real IO failures are
// logged but never abort the caller — credential teardown has already
// happened upstream and partial scrub leftovers will be re-cleaned by
// `event stop --force` or the next bus startup.
func scrubBusArtefacts() {
	for _, p := range []string{eventruntime.BusSockPath(), eventruntime.BusMetaFile(), eventruntime.BusPIDFile()} {
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			stderrLog("[event] OnUserCleared: remove %s: %v", p, err)
		}
	}
}

// parseFirstIntLine extracts the first run of ASCII decimal digits from
// data and returns it as an int.  Returns 0 on no-match.  Used by the
// bus.pid fallback above; kept tiny to avoid pulling strconv just for
// this.
func parseFirstIntLine(data []byte) int {
	n := 0
	started := false
	for _, b := range data {
		if b >= '0' && b <= '9' {
			started = true
			n = n*10 + int(b-'0')
			if n > 1<<30 {
				// guard against absurd values
				return 0
			}
			continue
		}
		if started {
			break
		}
		// Skip leading whitespace.
		if b == ' ' || b == '\t' || b == '\r' || b == '\n' {
			continue
		}
		// Non-digit before any digit — bail.
		return 0
	}
	return n
}

// stderrLog writes a single line to os.Stderr with a trailing newline.
// Centralised so the prefix `[event] OnUserCleared:` stays grep-stable.
// We write directly to os.Stderr (not via output.EventStderr) because
// this code path runs under config.ClearUserConfig which has no cobra
// command in scope.
func stderrLog(format string, args ...interface{}) {
	_, _ = fmt.Fprintf(os.Stderr, format+"\n", args...)
}
