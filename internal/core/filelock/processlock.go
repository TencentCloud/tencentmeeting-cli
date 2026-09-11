// processlock.go — process-lifetime exclusive file lock.
//
// Distinct from WithLock (which is short-lived and auto-releases via defer),
// a ProcessLock is acquired non-blockingly and held for the rest of the
// process: callers stash the *ProcessLock* somewhere it won't be GC'd, and
// either call Unlock() at graceful shutdown or rely on the OS to drop the
// kernel-side lock when the process exits.
//
// This is the foundation for "is the bus alive?" probes: the bus daemon
// takes a ProcessLock on bus.alive.lock at startup and never releases it;
// any subsequent TryLock on the same path returns ErrHeld iff the daemon
// is still alive.  A successfully-acquired probe means the previous holder
// died — the file is stale and the caller should proceed to clean up /
// fork a new bus.

package filelock

import (
	"fmt"
	"os"
	"sync"
)

// ProcessLock is a long-lived exclusive lock on a single file.
//
// Zero value is not usable; call NewProcessLock(path) to construct.  All
// methods are goroutine-safe; the typical usage is single-threaded though
// (acquire at startup, release at shutdown).
type ProcessLock struct {
	path string

	mu   sync.Mutex
	file *os.File // non-nil iff currently held
}

// NewProcessLock returns a ProcessLock object referencing path.  The lock
// file is NOT created or opened until TryLock is called, so the constructor
// is cheap and side-effect-free.
//
// path is typically a *.lock file inside the bus dir (BusAliveLock,
// BusForkLock).  Caller is responsible for ensuring the parent directory
// exists with appropriate permissions.
func NewProcessLock(path string) *ProcessLock {
	return &ProcessLock{path: path}
}

// Path returns the on-disk path the lock targets.  Useful for diagnostic
// logging and for status code that wants to display where the bus thinks
// its alive lock lives.
func (l *ProcessLock) Path() string { return l.path }

// TryLock attempts to acquire the lock without blocking.
//
// Possible returns:
//   - nil           — lock acquired; the *os.File is now owned by this
//     ProcessLock and will live until Unlock or process exit.
//   - ErrHeld       — another process owns the lock (use errors.Is).
//   - other error   — failed to open the lock file (permission, IO, ENOENT
//     on parent dir, ...) or a non-contention syscall failure.
//
// Calling TryLock on an already-held ProcessLock is a programming error and
// returns ErrHeld with the path so test failures point to the bug.
func (l *ProcessLock) TryLock() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.file != nil {
		return fmt.Errorf("%w: %s (re-lock)", ErrHeld, l.path)
	}

	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return fmt.Errorf("processlock: open %s: %w", l.path, err)
	}

	if err := lockFDNonBlocking(f); err != nil {
		_ = f.Close()
		return err // ErrHeld or platform failure, already wrapped
	}
	l.file = f
	return nil
}

// Unlock releases the lock and closes the underlying file handle.
//
// The lock file is left on disk on purpose: deleting it here would race with
// a competing TryLock that already opened the file but hadn't yet called
// flock/LockFileEx, leaving two processes thinking they each own the lock.
// Stale lock files are harmless (they're 0 bytes) and detected by their
// associated probe TryLock succeeding.
//
// Unlock is idempotent: calling on a non-held lock is a no-op.
func (l *ProcessLock) Unlock() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.file == nil {
		return nil
	}
	unlockErr := unlockFD(l.file)
	closeErr := l.file.Close()
	l.file = nil
	if unlockErr != nil {
		return fmt.Errorf("processlock: unlock %s: %w", l.path, unlockErr)
	}
	if closeErr != nil {
		return fmt.Errorf("processlock: close %s: %w", l.path, closeErr)
	}
	return nil
}
