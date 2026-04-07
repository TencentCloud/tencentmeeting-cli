//go:build !windows

// filelock_unix.go provides a cross-process exclusive file lock based on flock(2) (Linux/macOS).

package filelock

import (
	"fmt"
	"os"
	"syscall"
	"time"
)

// WithLock acquires an exclusive file lock, executes fn, and automatically releases the lock afterwards (SEC-012).
//
// Implementation strategy: non-blocking LOCK_NB polling (50ms interval) to avoid blocking flock which cannot be cancelled by context in Go.
// On no contention, the first attempt succeeds; on contention, waits up to defaultTimeout before returning a timeout error.
// The .lock file is retained after the lock is released (empty file, no disk impact).
func WithLock(lockPath string, fn func() error) error {
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return fmt.Errorf("failed to open lock file: %w", err)
	}
	defer f.Close()

	deadline := time.Now().Add(defaultTimeout)
	for {
		err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("file lock timeout (%v): %s", defaultTimeout, lockPath)
		}
		time.Sleep(50 * time.Millisecond)
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)

	return fn()
}
