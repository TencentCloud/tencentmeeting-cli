//go:build !windows

// filecheck_unix.go provides security checks before reading sensitive files such as .enc (Linux/macOS),
// preventing symlink attacks and cross-user file replacement.

package filecheck

import (
	"fmt"
	"os"
	"syscall"
)

// ValidateBeforeRead performs security checks before reading a file (SEC-010):
//   - Uses Lstat (not Stat) to reject symlinks
//   - Validates that the file owner UID matches the current user
//
// If Sys() cannot be cast to syscall.Stat_t, skips UID check and only retains the symlink check.
func ValidateBeforeRead(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}

	if !info.Mode().IsRegular() {
		return fmt.Errorf("security check failed: %s is not a regular file (possibly a symlink)", path)
	}

	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return nil
	}

	if stat.Uid != uint32(os.Getuid()) {
		return fmt.Errorf("security check failed: file %s owner (uid=%d) does not match current user (uid=%d)",
			path, stat.Uid, os.Getuid())
	}

	return nil
}
