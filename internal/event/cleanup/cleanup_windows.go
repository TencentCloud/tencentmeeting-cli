// cleanup_windows.go — Windows implementation of killProcess.
//
// Windows lacks SIGKILL; the equivalent operation is os.Process.Kill,
// which under the hood calls TerminateProcess.  Like the unix counterpart
// we treat "process already gone" as success.

//go:build windows

package cleanup

import (
	"errors"
	"os"
)

// killProcess force-terminates pid.  pid must be > 0 (callers verify).
//
// os.FindProcess on Windows returns a Process handle even for non-existent
// PIDs, so the only authoritative signal of "process gone" is the Kill
// call itself failing with os.ErrProcessDone.
func killProcess(pid int) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		// Best-effort: surface the underlying error.
		return err
	}
	if err := proc.Kill(); err != nil {
		if errors.Is(err, os.ErrProcessDone) {
			return nil
		}
		return err
	}
	// Release the handle so we don't leak a Win32 process object.
	_ = proc.Release()
	return nil
}
