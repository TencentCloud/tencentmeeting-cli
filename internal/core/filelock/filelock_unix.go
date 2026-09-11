//go:build !windows

// filelock_unix.go provides cross-process exclusive file locks based on flock(2)
// (Linux/macOS).  Two public entry points share a single locking primitive:
//
//	WithLock        — short-lived, polling acquire with timeout.
//	ProcessLock     — process-lifetime, non-blocking TryLock returning ErrHeld
//	                  on contention.  Defined in processlock.go; the platform
//	                  primitives lockFDNonBlocking / unlockFD live here.

package filelock

import (
	"errors"
	"fmt"
	"os"
	"syscall"
	"time"
)

// lockFDNonBlocking attempts a single non-blocking exclusive flock.
//
// Returns:
//   - nil           on success
//   - ErrHeld       when another holder has the lock (EWOULDBLOCK / EAGAIN)
//   - other error   on a real syscall failure (EBADF, EINVAL, ...)
//
// Non-blocking on purpose so callers can choose between "poll until timeout"
// (WithLock) and "fail fast" (ProcessLock.TryLock) without re-implementing
// the syscall.
func lockFDNonBlocking(f *os.File) error {
	err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err == nil {
		return nil
	}
	if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
		return fmt.Errorf("%w (lock: %s)", ErrHeld, f.Name())
	}
	return err
}

// unlockFD releases an exclusive flock.  Called from defer on the success
// path; safe to call on an already-unlocked fd (returns nil/ENOENT-ish).
func unlockFD(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
}

// WithLock acquires an exclusive file lock, executes fn, and automatically
// releases the lock afterwards (SEC-012).
//
// Implementation strategy: non-blocking LOCK_NB polling at 50 ms intervals so
// the wait is cancellable by the deadline.  flock(2) does not honour
// SetDeadline; polling is the standard Go workaround.  On no contention the
// first attempt succeeds; on contention, waits up to defaultTimeout before
// returning a timeout error.  The .lock file is retained after release
// (empty inode, no disk impact) to avoid an inode-reuse race between
// unlock-then-rm and a concurrent open+flock.
func WithLock(lockPath string, fn func() error) error {
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return fmt.Errorf("failed to open lock file: %w", err)
	}
	defer f.Close()

	deadline := time.Now().Add(defaultTimeout)
	for {
		err = lockFDNonBlocking(f)
		if err == nil {
			break
		}
		// Genuine syscall failures (not contention) are surfaced immediately:
		// retrying won't help if e.g. the fd became invalid.
		if !errors.Is(err, ErrHeld) {
			return fmt.Errorf("flock: %w", err)
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("file lock timeout (%v): %s", defaultTimeout, lockPath)
		}
		time.Sleep(50 * time.Millisecond)
	}
	defer func() { _ = unlockFD(f) }()

	return fn()
}
