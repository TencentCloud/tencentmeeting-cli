// atomic_write.go — shared helper for "publish a small file atomically".
//
// Several artefacts under BusDir() (bus.meta, ws.state, ...) share the exact
// same publication contract:
//
//   - readers see either the previous version of the file or the new one,
//     never a partially-written byte stream;
//   - the destination directory has 0700 perms and is created on demand;
//   - the file itself is 0600 (single-user runtime state);
//   - on any error the sibling .tmp is best-effort removed so the next
//     successful write isn't shadowed by a stale half-baked tmp.
//
// The actual atomicity comes from os.Rename within the same directory, which
// is atomic on POSIX and on NTFS when the destination already exists. The
// fsync before rename closes the "rename made it to the directory but the
// file's data didn't" window on power loss; not strictly required for the
// "concurrent reader" semantics callers care about, but cheap insurance for
// the at-most-one writer we have.
//
// Kept package-private on purpose: callers outside event/ have their own
// notions of error wrapping and permission and should not be tempted to
// reach in here for a generic "write file" primitive.

package event

import (
	"fmt"
	"os"
	"path/filepath"
)

// atomicWriteFile publishes data at dst by way of a sibling "<dst>.tmp"
// file plus rename. errPrefix is woven into every wrapped error so the
// caller's existing log messages ("bus meta: ...", "ws state: ...") stay
// intact when the helper is adopted.
//
// Pre-conditions:
//   - filepath.Dir(dst) is a path the current process is allowed to mkdir
//     (we create it with 0700 if missing);
//   - no other writer is racing to write the same dst (true for all current
//     callers: bus.meta is written only by the bus daemon; ws.state only by
//     the WSS source goroutine).
//
// Post-condition on success: dst contains exactly data, with mode 0600,
// and no "<dst>.tmp" sibling is left behind. On failure, dst is unchanged
// and the tmp file is best-effort removed.
func atomicWriteFile(dst string, data []byte, errPrefix string) error {
	dir := filepath.Dir(dst)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("%s: mkdir %s: %w", errPrefix, dir, err)
	}

	tmp := dst + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("%s: open tmp: %w", errPrefix, err)
	}
	if _, err = f.Write(data); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return fmt.Errorf("%s: write tmp: %w", errPrefix, err)
	}
	if err = f.Sync(); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return fmt.Errorf("%s: sync tmp: %w", errPrefix, err)
	}
	if err = f.Close(); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("%s: close tmp: %w", errPrefix, err)
	}
	if err = os.Rename(tmp, dst); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("%s: rename: %w", errPrefix, err)
	}
	return nil
}
