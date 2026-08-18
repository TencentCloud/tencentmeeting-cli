//go:build windows

// filelock_windows.go provides cross-process exclusive file locks based on
// LockFileEx (Windows).  Two public entry points share a single locking
// primitive:
//
//	WithLock        — short-lived, polling acquire with timeout.
//	ProcessLock     — process-lifetime, non-blocking TryLock returning ErrHeld
//	                  on contention.  Defined in processlock.go; the platform
//	                  primitives lockFDNonBlocking / unlockFD live here.
//
// We use a single fixed 1-byte region (offset 0, length 1) for every lock so
// LockFileEx and UnlockFileEx see matching ranges.  An on-disk size of 0 is
// fine: LockFileEx happily locks bytes past EOF and that's actually how
// "advisory" Windows file locks are commonly built.

package filelock

import (
	"errors"
	"fmt"
	"os"
	"time"

	"golang.org/x/sys/windows"
)

// lockFDNonBlocking attempts a single non-blocking exclusive LockFileEx.
//
// Returns:
//   - nil      on success
//   - ErrHeld  when another holder has the lock (ERROR_LOCK_VIOLATION /
//     ERROR_IO_PENDING with FAIL_IMMEDIATELY)
//   - other    on a real syscall failure
//
// We allocate a fresh OVERLAPPED per call (LockFileEx requires it) so the
// caller doesn't need to manage that state across the lock/unlock pair —
// Windows tracks the lock by (handle, offset, length), not by OVERLAPPED.
func lockFDNonBlocking(f *os.File) error {
	ol := new(windows.Overlapped)
	err := windows.LockFileEx(
		windows.Handle(f.Fd()),
		windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY,
		0, 1, 0, ol,
	)
	if err == nil {
		return nil
	}
	if errors.Is(err, windows.ERROR_LOCK_VIOLATION) || errors.Is(err, windows.ERROR_IO_PENDING) {
		return fmt.Errorf("%w (lock: %s)", ErrHeld, f.Name())
	}
	return err
}

// unlockFD releases the 1-byte lock taken by lockFDNonBlocking.  Errors are
// returned but typically ignored on the defer path: the OS releases the lock
// on handle close anyway.
func unlockFD(f *os.File) error {
	ol := new(windows.Overlapped)
	return windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, ol)
}

// WithLock acquires an exclusive file lock, executes fn, and automatically
// releases the lock afterwards (SEC-012).
//
// Uses non-blocking LOCKFILE_FAIL_IMMEDIATELY polling at 50 ms intervals to
// keep the wait cancellable by deadline.
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
		if !errors.Is(err, ErrHeld) {
			return fmt.Errorf("LockFileEx: %w", err)
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("file lock timeout (%v): %s", defaultTimeout, lockPath)
		}
		time.Sleep(50 * time.Millisecond)
	}
	defer func() { _ = unlockFD(f) }()

	return fn()
}
