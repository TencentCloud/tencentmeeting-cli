// Package filelock provides a file-based cross-process exclusive lock for atomic write scenarios (SEC-012).
//
// Two flavours are exposed:
//
//	WithLock(path, fn)         — short-lived: acquire (poll up to defaultTimeout)
//	                             → run fn → release.  Used for atomic writes
//	                             that hold the lock for milliseconds.
//	NewProcessLock(path).TryLock()
//	                           — process-lifetime: non-blocking acquire; the
//	                             caller keeps the *os.File alive for the rest
//	                             of the process and the OS releases the lock
//	                             at exit.  Used by the bus daemon to expose
//	                             "am I alive?" as a TryLock probe.
//
// The platform-specific lockFD/unlockFD primitives live in filelock_unix.go /
// filelock_windows.go and are shared by both flavours so we have one
// well-tested locking implementation per OS.
package filelock

import (
	"errors"
	"time"
)

// defaultTimeout is the maximum wait time for acquiring an exclusive lock
// in WithLock's polling loop.  ProcessLock.TryLock is non-blocking and never
// uses this constant.
const defaultTimeout = 5 * time.Second

// ErrHeld is returned by ProcessLock.TryLock when another process already
// owns the lock.  Callers errors.Is(err, ErrHeld) to distinguish this
// retryable contention from a real syscall failure (permission denied,
// EIO, etc.) that warrants giving up.
//
// WithLock does NOT surface ErrHeld — it polls until the lock is acquired
// or defaultTimeout elapses, returning a generic timeout error in the
// latter case.
var ErrHeld = errors.New("filelock: lock already held")
