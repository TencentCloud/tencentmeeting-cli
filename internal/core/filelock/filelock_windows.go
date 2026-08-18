//go:build windows

// filelock_windows.go provides a cross-process exclusive file lock based on LockFileEx (Windows).

package filelock

import (
	"fmt"
	"os"
	"time"

	"golang.org/x/sys/windows"
)

// WithLock acquires an exclusive file lock, executes fn, and automatically releases the lock afterwards (SEC-012).
// Uses non-blocking LOCKFILE_FAIL_IMMEDIATELY polling (50ms interval); returns an error on timeout.
func WithLock(lockPath string, fn func() error) error {
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return fmt.Errorf("failed to open lock file: %w", err)
	}
	defer f.Close()

	ol := new(windows.Overlapped)
	deadline := time.Now().Add(defaultTimeout)
	for {
		err = windows.LockFileEx(
			windows.Handle(f.Fd()),
			windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY,
			0, 1, 0, ol,
		)
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("file lock timeout (%v): %s", defaultTimeout, lockPath)
		}
		time.Sleep(50 * time.Millisecond)
	}
	defer windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, ol)

	return fn()
}
