// cleanup_unix.go — POSIX implementation of killProcess.
//
// SIGKILL is the documented mechanism for the orphan-recovery path
//.  We accept ESRCH (process already dead) and
// os.ErrProcessDone as success since both indicate the orphan condition
// has already cleared itself; any other error is surfaced to the caller
// for stderr logging.

//go:build !windows

package cleanup

import (
	"errors"
	"os"
	"syscall"
)

// killProcess sends SIGKILL to pid.  pid must be > 0 (callers verify).
//
// Returns nil if the kernel accepted the signal OR the process was
// already gone.  Any other error (EPERM on a foreign-uid process, etc.)
// is returned verbatim for the caller to log.
func killProcess(pid int) error {
	err := syscall.Kill(pid, syscall.SIGKILL)
	if err == nil {
		return nil
	}
	if errors.Is(err, os.ErrProcessDone) || errors.Is(err, syscall.ESRCH) {
		// Already gone — counts as success for our cleanup goal.
		return nil
	}
	return err
}
