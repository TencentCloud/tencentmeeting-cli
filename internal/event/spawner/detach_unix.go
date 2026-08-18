//go:build !windows

// detach_unix.go — POSIX detachment (Setsid).

package spawner

import (
	"os/exec"
	"syscall"
)

// applyDetachAttrs requests a fresh session for the child process so it
// survives the parent's terminal closing.  This matters because users often
// run `tmeet event consume` from an interactive shell; without Setsid the
// bus would receive SIGHUP when the user logs out and exit before the next
// `event consume` could attach to it.
func applyDetachAttrs(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
