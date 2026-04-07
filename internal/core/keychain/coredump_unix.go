//go:build !windows

package keychain

import (
	"fmt"
	"os"
	"syscall"
)

// disableCoreDump disables core dump generation for the process (SEC-008).
// A panic core dump may contain the master key and plaintext tokens in memory.
func disableCoreDump() error {
	return syscall.Setrlimit(syscall.RLIMIT_CORE, &syscall.Rlimit{Cur: 0, Max: 0})
}

func init() {
	if err := disableCoreDump(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to disable core dump: %v\n", err)
	}
}
