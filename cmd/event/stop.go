// stop.go — `tmeet event stop`.
//
// stop has three high-level operating modes:
//
//	tmeet event stop                     # graceful: SendShutdown, wait, exit 0
//	                                     #   refuses if any consumer is still attached
//	tmeet event stop --force             # graceful first; on timeout/orphan/refused,
//	                                     #   scrub bus.pid/bus.meta/bus.sock and
//	                                     #   release any leftover state
//	tmeet event stop --timeout 10s       # tweak the graceful wait
//
// Output schema (matches the consume contract):
//
//	{
//	  "results": [
//	    { "state": "stopped"|"refused"|"errored"|"no_bus", ... }
//	  ]
//	}
//
// state is the single source of truth for downstream tooling; the
// per-state optional fields (consumers_evicted, consumer_count, forced,
// socket_cleaned, elapsed_ms, hint) only appear when meaningful.
//
// Exit codes:
//
//	0   state ∈ {stopped, no_bus}
//	2   state ∈ {refused, errored}   — stop did not (or could not) take action

package event

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"tmeet/internal"
	"tmeet/internal/core/filelock"
	eventruntime "tmeet/internal/event"
	"tmeet/internal/event/busctl"
	"tmeet/internal/event/transport"
	"tmeet/internal/exception"
	"tmeet/internal/output"
)

// stopOutput is the JSON shape returned to stdout regardless of exit code.
//
// `results` is always non-nil (encoded as `[]` rather than `null`) so
// downstream `jq '.results[]'` is always safe.  Length is at most 1 —
// tmeet has at most one bus per host — but the array shape mirrors the
// status command and leaves room for future multi-bus extensions
// without a JSON-schema bump.
type stopOutput struct {
	Results []stopResult `json:"results"`
}

// stopResult is one entry of stopOutput.results.  Fields are
// state-specific; see the Run/runGraceful/runForceCleanup branches for
// which fields each state populates.
type stopResult struct {
	State            string `json:"state"` // stopped | refused | errored | no_bus
	OpenIDHash       string `json:"openid_hash,omitempty"`
	PID              int    `json:"pid,omitempty"`
	ConsumersEvicted int    `json:"consumers_evicted,omitempty"`
	ConsumerCount    int    `json:"consumer_count,omitempty"`
	Forced           bool   `json:"forced,omitempty"`
	SocketCleaned    bool   `json:"socket_cleaned,omitempty"`
	ElapsedMs        int64  `json:"elapsed_ms,omitempty"`
	Hint             string `json:"hint,omitempty"`
}

// State values for stopResult.State.
const (
	stopStateStopped = "stopped" // bus exited (graceful or forced); ConsumersEvicted populated when known
	stopStateRefused = "refused" // active consumers present and --force not set; ConsumerCount populated
	stopStateErrored = "errored" // graceful timed out / scrub failed and no recovery path
	stopStateNoBus   = "no_bus"  // nothing on disk; no-op
)

// defaultStopTimeout is the deadline for waiting on a graceful Shutdown.
//
// The bus's own teardown path (close listener → drain conns → return from
// Run) typically completes in tens of milliseconds; 10 s gives plenty of
// margin for a slow source goroutine to honour ctx cancellation while
// staying well below user-perception "is this hung?" threshold.
const defaultStopTimeout = 10 * time.Second

// pingPollInterval — how often we re-probe Ping while waiting for the bus
// to exit.  50 ms keeps stop snappy without busy-looping on the lock file
// or hammering the IPC layer.
const pingPollInterval = 50 * time.Millisecond

// StopOptions holds the options for `tmeet event stop`.
type StopOptions struct {
	tmeet   *internal.Tmeet
	Force   bool          // skip the consumer-protection refusal; force-cleanup orphan / stale_owner buses; scrub leftover bus.pid / bus.meta / bus.sock
	Timeout time.Duration // max time to wait for the bus to exit gracefully
}

// newStopCmd builds the `tmeet event stop` command.
//
// skipPreCheck=true so admins can scrub orphan state after `auth logout`
// (the most common reason to invoke `event stop --force`).
func newStopCmd(tmeet *internal.Tmeet) *cobra.Command {
	opts := &StopOptions{tmeet: tmeet}
	cmd := &cobra.Command{
		Use:   "stop",
		Short: "Stop the local bus daemon",
		Long: `Ask the bus daemon to exit.

Without --force, stop refuses to terminate a bus that still has active
consumers (state=refused, exit 2) — pass --force to evict them.  Orphan
or stale_owner state likewise requires --force to confirm the user
understands they are scrubbing on-disk artefacts of a different (or
dead) bus.`,
		Annotations: map[string]string{"skipPreCheck": "true"},
		Args:        cobra.NoArgs,
		RunE:        opts.Run,
	}
	cmd.Flags().BoolVar(&opts.Force, "force", false,
		"skip the active-consumer protection; force-cleanup orphan / stale_owner buses; scrub leftover bus.pid / bus.meta / bus.sock")
	cmd.Flags().DurationVar(&opts.Timeout, "timeout", defaultStopTimeout,
		"max time to wait for the bus to exit gracefully")
	return cmd
}

// Run executes `event stop`.
func (o *StopOptions) Run(cmd *cobra.Command, args []string) error {
	if o.Timeout <= 0 {
		o.Timeout = defaultStopTimeout
	}
	tr := transport.New()

	// 1. Discover state — reuse status.go's computeBusView so the JSON
	//    output is identical in shape and the logic stays single-sourced.
	view, present, err := computeBusView(tr)
	if err != nil {
		return exception.EventInternalError.With("event stop: probe state: %v", err)
	}
	if !present {
		// No bus on disk at all.
		return emitStop(cmd, stopResult{State: stopStateNoBus}, 0)
	}

	// 2. Branch on state.
	switch view.State {
	case stateRunning:
		return o.runGracefulRunning(cmd, tr, view)

	case stateStaleOwner:
		// stale_owner means the bus IS alive — we still try the graceful path
		// (it'll honour Shutdown regardless of who asked) but only when the
		// user passes --force, because stopping someone else's bus is the
		// kind of operation we want explicit consent for.
		if !o.Force {
			return emitStop(cmd, stopResult{
				State:         stopStateRefused,
				OpenIDHash:    view.OpenIDHash,
				PID:           view.PID,
				ConsumerCount: view.ConsumerCount,
				Hint:          "bus is bound to a different user; pass --force to stop it anyway",
			}, exitCodeOrphan)
		}
		return o.runGracefulRunning(cmd, tr, view)

	case stateOrphan:
		// Bus is already dead; --force scrubs the on-disk leftovers.
		if !o.Force {
			return emitStop(cmd, stopResult{
				State:      stopStateRefused,
				OpenIDHash: view.OpenIDHash,
				PID:        view.PID,
				Hint:       "orphan state on disk; pass --force to scrub bus.pid / bus.meta",
			}, exitCodeOrphan)
		}
		return o.runForceCleanup(cmd, view, false)

	default:
		// Defensive: unknown state.  Treat as errored so health checks
		// notice it.
		return emitStop(cmd, stopResult{
			State:      stopStateErrored,
			OpenIDHash: view.OpenIDHash,
			PID:        view.PID,
			Hint:       "unrecognised bus state; nothing to do",
		}, exitCodeOrphan)
	}
}

// runGracefulRunning handles state==running (and the --force branches of
// stale_owner that fall through here).
//
//   - With consumers attached AND --force not set ⇒ refused (exit 2).
//   - Otherwise: send Shutdown, poll until exit, optionally scrub on
//     timeout under --force.
func (o *StopOptions) runGracefulRunning(cmd *cobra.Command, tr transport.IPC, view busView) error {
	// Consumer-protection rail: refuse to stop while consumers are
	// attached unless --force is explicit.  view.ConsumerCount comes
	// from busctl.QueryStatus inside computeBusView, so it's the same
	// snapshot `event status` would print.
	if !o.Force && view.ConsumerCount > 0 {
		return emitStop(cmd, stopResult{
			State:         stopStateRefused,
			OpenIDHash:    view.OpenIDHash,
			PID:           view.PID,
			ConsumerCount: view.ConsumerCount,
			Hint:          "active consumers attached; pass --force to evict them or stop the consumers first",
		}, exitCodeOrphan)
	}

	startedAt := time.Now()

	// Send Shutdown.  ErrNotRunning here means the bus exited between our
	// status probe and the SendShutdown — treat as already-stopped.
	if err := busctl.SendShutdown(tr, o.Force); err != nil {
		if errors.Is(err, busctl.ErrNotRunning) {
			return emitStop(cmd, stopResult{
				State:      stopStateStopped,
				OpenIDHash: view.OpenIDHash,
				PID:        view.PID,
				Hint:       "bus had already exited before stop arrived",
			}, 0)
		}
		return exception.EventInternalError.With("event stop: send shutdown: %v", err)
	}

	// Poll until either Ping fails (bus is gone) or timeout elapses.
	deadline := time.Now().Add(o.Timeout)
	for time.Now().Before(deadline) {
		if err := busctl.Ping(tr); errors.Is(err, busctl.ErrNotRunning) {
			// Confirm the alive lock is also released — without that check, a
			// stale socket could fool Ping for a tick.
			if !aliveLockHeld() {
				return emitStop(cmd, stopResult{
					State:            stopStateStopped,
					OpenIDHash:       view.OpenIDHash,
					PID:              view.PID,
					ConsumersEvicted: view.ConsumerCount,
					Forced:           o.Force,
					ElapsedMs:        time.Since(startedAt).Milliseconds(),
				}, 0)
			}
		}
		time.Sleep(pingPollInterval)
	}

	// Timed out.  --force escalates to a scrub; otherwise we report errored.
	if o.Force {
		// runForceCleanup handles its own emit; pass through the
		// pre-scrub view so it can populate openid_hash / pid.
		return o.runForceCleanup(cmd, view, true)
	}
	return emitStop(cmd, stopResult{
		State:      stopStateErrored,
		OpenIDHash: view.OpenIDHash,
		PID:        view.PID,
		ElapsedMs:  time.Since(startedAt).Milliseconds(),
		Hint:       fmt.Sprintf("bus did not exit within %s; pass --force to scrub state", o.Timeout),
	}, exitCodeOrphan)
}

// runForceCleanup unlinks bus.pid / bus.meta / bus.sock.  Used when:
//   - state==orphan and --force was passed, or
//   - graceful stop timed out under --force (escalatedFromGraceful=true).
//
// We do NOT delete bus.alive.lock or bus.fork.lock.  Lock files are 0-byte
// inodes whose semantics depend on flock() being called against them; a
// stale empty file is harmless to the next bus startup (it will reuse the
// inode), and removing them races with a concurrent (e.g. fresh consume)
// open-then-flock pair.
func (o *StopOptions) runForceCleanup(cmd *cobra.Command, view busView, escalatedFromGraceful bool) error {
	cleaned := []string{}
	failed := []string{}

	for _, p := range []string{eventruntime.BusPIDFile(), eventruntime.BusMetaFile(), eventruntime.BusSockPath()} {
		err := os.Remove(p)
		if err == nil {
			cleaned = append(cleaned, p)
			continue
		}
		if os.IsNotExist(err) {
			continue
		}
		failed = append(failed, fmt.Sprintf("%s: %v", p, err))
	}

	if len(failed) > 0 {
		return emitStop(cmd, stopResult{
			State:      stopStateErrored,
			OpenIDHash: view.OpenIDHash,
			PID:        view.PID,
			Forced:     true,
			Hint:       fmt.Sprintf("failed to remove: %v", failed),
		}, exitCodeOrphan)
	}

	hint := ""
	if escalatedFromGraceful {
		hint = fmt.Sprintf("graceful stop timed out; scrubbed %d artefact(s)", len(cleaned))
	}
	return emitStop(cmd, stopResult{
		State:            stopStateStopped,
		OpenIDHash:       view.OpenIDHash,
		PID:              view.PID,
		ConsumersEvicted: view.ConsumerCount,
		Forced:           true,
		SocketCleaned:    containsString(cleaned, eventruntime.BusSockPath()),
		Hint:             hint,
	}, 0)
}

// emitStop is the single emit point for all stop output.  Centralising
// it keeps the JSON shape consistent and makes the exit-code policy
// auditable in one place: any non-zero `code` goes through
// exitWithCodeAfterJSON (which os.Exit's after writing JSON), while
// `code==0` returns nil and lets cobra's defer chain run.
func emitStop(cmd *cobra.Command, r stopResult, code int) error {
	out := stopOutput{Results: []stopResult{r}}
	if code != 0 {
		return exitWithCodeAfterJSON(cmd, out, "", code)
	}
	return output.EventPrint(cmd, out)
}

// containsString reports whether haystack contains needle (string slice
// membership test).  Tiny helper kept local to avoid importing slices
// just for one comparison.  Named distinctly from the substring `contains`
// in consume_runner.go to keep package-level scope unambiguous.
func containsString(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

// aliveLockHeld is a fast probe used by the graceful poll: even after the
// bus closes its accept loop (so Ping fails) the alive lock might briefly
// linger if the OS hasn't yet released the fd's locks.  Wait for both
// conditions before declaring success.
func aliveLockHeld() bool {
	probe := filelock.NewProcessLock(eventruntime.BusAliveLock())
	if err := probe.TryLock(); err != nil {
		return errors.Is(err, filelock.ErrHeld)
	}
	_ = probe.Unlock()
	return false
}

// runStop is a thin shim retained so status_stop_test.go can keep its
// `runStop(cmd, force, timeout)` calls without touching the test code.
// Production code reaches Run via newStopCmd's RunE.
func runStop(cmd *cobra.Command, force bool, timeout time.Duration) error {
	return (&StopOptions{Force: force, Timeout: timeout}).Run(cmd, nil)
}
