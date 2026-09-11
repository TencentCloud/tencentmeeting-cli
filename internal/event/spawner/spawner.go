// Package spawner — fork the `_bus` daemon when no bus is running.
//
// Split out from cmd/event so non-cobra callers (future SDK / library form)
// can also bring up a bus without depending on the cobra layer.  The
// public surface is a single function:
//
//	forked, err := spawner.EnsureBus(transport.New())
//
// EnsureBus is idempotent: if a bus is already accepting it returns
// (false, nil); otherwise it forks `<self> event _bus` and polls
// busctl.Ping until the new daemon is reachable or a deadline expires.
//
// The package locates the running binary via os.Executable() and runs it
// with argv = ["event", "_bus"].  Detachment specifics (Setsid on POSIX,
// CREATE_NEW_PROCESS_GROUP on Windows) live in detach_unix.go /
// detach_windows.go behind build tags so this file stays cross-platform.

package spawner

import (
	"errors"
	"os"
	"os/exec"
	"runtime"
	"time"

	"tmeet/internal/event/busctl"
	"tmeet/internal/event/transport"
	"tmeet/internal/exception"
)

// busReadyTimeout — how long EnsureBus waits for the freshly forked daemon
// to come up.  In practice the bus is accepting in <100 ms; 5 s gives lots of
// headroom for slow CI / cold-cache disk reads without making "bus is hung"
// scenarios drag on forever.
const busReadyTimeout = 5 * time.Second

// busPingPollInterval — how often we re-Dial while waiting.  20 ms is well
// below the bus's actual startup latency so we don't add noticeable jitter.
const busPingPollInterval = 20 * time.Millisecond

// EnsureBus brings the bus daemon online if it isn't already.
//
// Returns:
//   - true,  nil — we forked a fresh bus (caller's stderr should announce it).
//   - false, nil — bus was already running; nothing to do.
//   - _,     err — fork failed or the freshly-forked bus didn't become
//     reachable within busReadyTimeout.
func EnsureBus(tr transport.IPC) (forked bool, err error) {
	// Already running? Fast path.
	if pingErr := busctl.Ping(tr); pingErr == nil {
		return false, nil
	} else if !errors.Is(pingErr, busctl.ErrNotRunning) {
		// Dialed something but it returned an unexpected error.  We treat
		// this as "running but unhealthy" and let the caller surface a more
		// descriptive message after Hello fails.
		return false, nil
	}

	// Not running — fork.
	if err := forkBus(); err != nil {
		return false, exception.EventInternalError.With("fork bus: %v", err)
	}

	// Wait until the new daemon is accepting.
	deadline := time.Now().Add(busReadyTimeout)
	for time.Now().Before(deadline) {
		if pingErr := busctl.Ping(tr); pingErr == nil {
			return true, nil
		}
		time.Sleep(busPingPollInterval)
	}
	return true, exception.EventInternalError.With("bus did not become ready within %s", busReadyTimeout)
}

// forkBus spawns the daemon process.
//
// We rely on os.Executable() to find the current binary; this works for the
// installed tmeet (a real binary) and for `go test`-built binaries (which
// also report a real path).  The child runs `<self> event _bus` and inherits
// the parent's environment so TMEET_CLI_CONFIG_DIR and any keychain hooks
// stay aligned.
//
// Detachment specifics:
//
//   - On POSIX we set Setsid=true so the bus survives the consumer's
//     terminal closing (Ctrl+D / SSH disconnect).  Without this, a SIGHUP to
//     the parent terminal session would kill the bus too.
//   - On Windows there's no Setsid; CreateProcess defaults already detach
//     the child from the parent's job object as long as we don't share a
//     console handle, which we ensure by setting Stdin/Stdout/Stderr to nil
//     (Go's exec package then opens NUL handles).
//
// stdio:
//   - Stdin  = nil (no controlling input).
//   - Stdout = nil (NDJSON output of the daemon is bus.log, not stdout).
//   - Stderr = nil (errors go to bus.log too — see internal/event/bus/log.go).
//
// We deliberately do NOT inherit the consumer's stderr; the bus is its own
// process and bleeding its log into consume's diagnostic stream confuses
// users and breaks the "[event] ..." prefix contract.
func forkBus() error {
	exe, err := os.Executable()
	if err != nil {
		return exception.EventInternalError.With("locate self: %v", err)
	}
	cmd := exec.Command(exe, "event", "_bus")
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Env = os.Environ()
	applyDetachAttrs(cmd)
	if err := cmd.Start(); err != nil {
		return err
	}
	// Release immediately — we do NOT Wait().  The child is independent
	// and will outlive us; Wait would block until the bus exits, defeating
	// the whole point of forking.
	if cmd.Process != nil {
		_ = cmd.Process.Release()
	}
	_ = runtime.GOOS // referenced indirectly via applyDetachAttrs build tags
	return nil
}
