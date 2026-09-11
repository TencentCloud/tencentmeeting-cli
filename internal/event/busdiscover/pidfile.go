// pidfile.go — bus.pid management & alive-lock acquisition.
//
// WritePIDFile is the bus daemon's "I'm alive" handshake at startup:
//
//	1. Open BusAliveLock and TryLock it as a ProcessLock — the lock travels
//	   with the process and is released by the OS at exit.  ErrHeld here
//	   means another bus already won the race; the caller (cmd/event/_bus.go)
//	   should exit cleanly without disturbing the running daemon.
//	2. Atomically write BusPIDFile (`pid\nRFC3339-start\n`) so external
//	   probes (`event status`, busctl) can read it without locking.
//
// Returns the *Handle so the bus can defer Release() in graceful-shutdown
// paths; production code typically just lets process exit drop the lock.

package busdiscover

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"tmeet/internal/core/filelock"
	eventruntime "tmeet/internal/event"
	"tmeet/internal/exception"
)

// Handle wraps the alive-lock so its lifetime matches the daemon's.  Once a
// Handle is returned, the caller MUST keep it referenced (e.g. as a package
// var) for the duration of the process; if it gets GC'd the underlying
// *os.File could in theory be finalized and the lock prematurely released.
//
// Practically the bus's main goroutine always parks waiting on accept(), so
// the Handle is reachable from the goroutine stack; the explicit reference
// is a belt-and-braces measure.
type Handle struct {
	lock *filelock.ProcessLock
}

// Release explicitly drops the alive lock.  Used by graceful shutdown and
// by tests; production usually relies on process exit.  Idempotent.
func (h *Handle) Release() error {
	if h == nil || h.lock == nil {
		return nil
	}
	return h.lock.Unlock()
}

// Path returns the on-disk path of the alive lock.  Useful for log messages.
func (h *Handle) Path() string {
	if h == nil || h.lock == nil {
		return ""
	}
	return h.lock.Path()
}

// WritePIDFile claims the alive lock and writes bus.pid atomically.
//
// Possible returns:
//   - (handle, nil)        — exclusive ownership won; caller is the new bus.
//   - (nil, filelock.ErrHeld) — another bus is already running; caller should
//     back off (typical: exit 0 from the
//     fork-loser branch in cmd/event/_bus.go).
//   - (nil, other error)   — disk full, permission denied, etc.; caller
//     surfaces the error and exits non-zero.
//
// Writing the PID file AFTER taking the lock means a stale bus.pid never
// out-lives the alive lock: if the bus crashes, the OS drops the lock; if a
// new bus starts, it overwrites bus.pid before any reader sees it.
func WritePIDFile(pid int) (*Handle, error) {
	dir := eventruntime.BusDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, exception.EventInternalError.With("busdiscover: mkdir %s: %v", dir, err)
	}

	lock := filelock.NewProcessLock(eventruntime.BusAliveLock())
	if err := lock.TryLock(); err != nil {
		// ErrHeld is expected and propagated as-is for errors.Is() callers.
		return nil, err
	}

	pidPath := eventruntime.BusPIDFile()
	tmpPath := pidPath + ".tmp"
	payload := fmt.Sprintf("%d\n%s\n", pid, time.Now().UTC().Format(time.RFC3339))

	if err := os.WriteFile(tmpPath, []byte(payload), 0600); err != nil {
		_ = lock.Unlock()
		return nil, exception.EventInternalError.With("busdiscover: write pid tmp %s: %v", tmpPath, err)
	}
	if err := os.Rename(tmpPath, pidPath); err != nil {
		_ = os.Remove(tmpPath)
		_ = lock.Unlock()
		return nil, exception.EventInternalError.With("busdiscover: rename pid file %s: %v", pidPath, err)
	}
	return &Handle{lock: lock}, nil
}

// RemovePIDFile deletes bus.pid if present.  Idempotent.  Used by graceful
// shutdown and `event stop --force`.  Note: this does NOT release the alive
// lock — that's a Handle.Release() / process-exit concern.
func RemovePIDFile() error {
	err := os.Remove(eventruntime.BusPIDFile())
	if err == nil || os.IsNotExist(err) {
		return nil
	}
	return exception.EventInternalError.With("busdiscover: remove pid file: %v", err)
}

// readPIDFile reads & parses bus.pid.  Returns (0, time.Time{}, error) when
// the file is missing or malformed; callers downgrade error to "alive but
// metadata not yet populated".
func readPIDFile() (int, time.Time, error) {
	path := eventruntime.BusPIDFile()
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, time.Time{}, err
	}
	lines := strings.SplitN(strings.TrimSpace(string(data)), "\n", 2)
	if len(lines) < 2 {
		return 0, time.Time{}, exception.EventInternalError.With("busdiscover: malformed pid file %s", path)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(lines[0]))
	if err != nil {
		return 0, time.Time{}, exception.EventInternalError.With("busdiscover: malformed pid in %s: %v", path, err)
	}
	startTime, err := time.Parse(time.RFC3339, strings.TrimSpace(lines[1]))
	if err != nil {
		return 0, time.Time{}, exception.EventInternalError.With("busdiscover: malformed timestamp in %s: %v", path, err)
	}
	return pid, startTime, nil
}

// ReadPIDFile is the exported version of readPIDFile for callers outside the
// scanner (e.g. status / stop).  Distinguishes "file absent" from "file
// malformed" via errors.Is(err, os.ErrNotExist).
func ReadPIDFile() (pid int, started time.Time, err error) {
	pid, started, err = readPIDFile()
	if err != nil && errors.Is(err, os.ErrNotExist) {
		// re-wrap with the canonical sentinel so callers can use os.IsNotExist
		err = fmt.Errorf("%w: %s", os.ErrNotExist, eventruntime.BusPIDFile())
	}
	return
}

// stale-lock cleanup helper kept package-private; not currently exported but
// useful for `event stop --force` once we wire it in batch 2.3.
var _ = filepath.Join
