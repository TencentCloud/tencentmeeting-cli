//go:build windows

// detach_windows.go — Windows detachment helpers.
//
// Windows doesn't have Setsid; CreateProcess detaches by default as long as
// the child doesn't share a console handle with the parent.  We pass
// CREATE_NEW_PROCESS_GROUP so the child runs in its own group and Ctrl+C
// from the parent shell doesn't propagate to the bus daemon.

package spawner

import (
	"os/exec"
	"syscall"
)

const createNewProcessGroup = 0x00000200

func applyDetachAttrs(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: createNewProcessGroup,
		HideWindow:    true,
	}
}
