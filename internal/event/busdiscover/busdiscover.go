// Package busdiscover provides liveness probing for the per-host bus daemon.
//
// The probe is built on a single primitive — try to acquire bus.alive.lock
// non-blockingly:
//
//	ErrHeld   ⇒ another process holds the lock ⇒ bus is alive
//	success   ⇒ no live holder; we just took a stale lock, release it
//	ENOENT    ⇒ lock file never existed ⇒ no bus has ever run
//
// This is more reliable than checking the PID file's content (a PID can be
// recycled by an unrelated process) and avoids any platform-specific
// "is this PID alive" syscall.
//
// Unlike lark-cli's busdiscover, wemeet has exactly ONE global bus per host
// , so this package returns at most a single Process.
package busdiscover

import (
	"errors"
	"os"
	"time"

	"tmeet/internal/core/filelock"
	eventruntime "tmeet/internal/event"
	"tmeet/internal/exception"
)

// Process describes the live bus daemon if one is running.  All fields are
// best-effort: PID/StartedAt come from bus.pid which is written separately
// from the alive-lock acquire, so a brief race window can yield (alive=true,
// pid=0) — callers should tolerate that and treat it as "alive but
// metadata not yet populated".
type Process struct {
	PID       int
	StartedAt time.Time
}

// Scanner is the public interface so tests can swap in a fake.
type Scanner interface {
	// Scan probes the bus dir.
	//
	// Returns:
	//   - (proc, true,  nil)  — bus is alive; proc is best-effort populated.
	//   - (nil,  false, nil)  — no bus running (either nothing on disk, or
	//                            stale lock that we successfully grabbed and
	//                            immediately released).
	//   - (nil,  false, err)  — real failure: permission denied on the bus
	//                            dir, malformed pid file the caller cares
	//                            about, etc.
	Scan() (*Process, bool, error)
}

// Default returns a Scanner that probes the canonical BusDir() / BusAliveLock().
func Default() Scanner { return &fsScanner{} }

type fsScanner struct{}

// Scan implements Scanner.Scan using the filesystem layout from event/paths.go.
func (s *fsScanner) Scan() (*Process, bool, error) {
	lockPath := eventruntime.BusAliveLock()

	// Fast path: lock file does not exist => bus has never been started here
	// (or stop already cleaned up the directory).  We deliberately do NOT
	// auto-create the file — that would race with a concurrent bus start
	// which is the only legitimate writer of this path.
	if _, err := os.Stat(lockPath); err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, exception.EventInternalError.With("busdiscover: stat %s: %v", lockPath, err)
	}

	probe := filelock.NewProcessLock(lockPath)
	err := probe.TryLock()
	switch {
	case err == nil:
		// Got the lock: previous holder is gone.  Release immediately so we
		// don't masquerade as a bus ourselves; the lock file is left on disk
		// (NewProcessLock policy) and any subsequent legitimate bus start
		// will reuse it.
		_ = probe.Unlock()
		return nil, false, nil

	case errors.Is(err, filelock.ErrHeld):
		// Live bus.  Best-effort decorate with PID/StartedAt from bus.pid;
		// readPIDFile failure is downgraded to a partial result so status
		// can still report state=running with pid=0 instead of failing hard.
		pid, started, readErr := readPIDFile()
		if readErr != nil {
			return &Process{}, true, nil
		}
		return &Process{PID: pid, StartedAt: started}, true, nil

	default:
		return nil, false, exception.EventInternalError.With("busdiscover: probe %s: %v", lockPath, err)
	}
}
