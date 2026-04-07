//go:build windows

package keychain

import (
	"fmt"
	"os"
	"syscall"
)

const semNOGPFaultErrorBox = 0x0002

// disableCoreDump disables Windows error reporting popups and crash dumps (SEC-008).
func disableCoreDump() error {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	setErrorMode := kernel32.NewProc("SetErrorMode")
	if err := setErrorMode.Find(); err != nil {
		return fmt.Errorf("locate SetErrorMode: %w", err)
	}
	setErrorMode.Call(uintptr(semNOGPFaultErrorBox))
	return nil
}

func init() {
	if err := disableCoreDump(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to disable core dump: %v\n", err)
	}
}
