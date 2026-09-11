// busdiscover_test.go — verifies the scanner agrees with ProcessLock's
// view of "alive vs stale" across the three states described in §3.3:
//
//   1. lock file absent      → scanner reports no bus
//   2. lock file present, no holder → scanner reports no bus and leaves the
//                                     stale lock alone (releases the probe)
//   3. lock file held         → scanner reports alive

package busdiscover

import (
	"os"
	"path/filepath"
	"testing"

	"tmeet/internal/core/filelock"
)

// withTempBusDir redirects BusDir() (which lives in tmeet/internal/event) to
// a t.TempDir() for the duration of the test and restores the env var on
// teardown.  This relies on internal/event/paths.go honouring TMEET_CLI_CONFIG_DIR.
func withTempBusDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	prev, hadPrev := os.LookupEnv("TMEET_CLI_CONFIG_DIR")
	t.Setenv("TMEET_CLI_CONFIG_DIR", dir)
	t.Cleanup(func() {
		if hadPrev {
			_ = os.Setenv("TMEET_CLI_CONFIG_DIR", prev)
		} else {
			_ = os.Unsetenv("TMEET_CLI_CONFIG_DIR")
		}
	})
	// Pre-create event/ so callers don't have to.
	if err := os.MkdirAll(filepath.Join(dir, "event"), 0700); err != nil {
		t.Fatalf("mkdir event: %v", err)
	}
	return dir
}

func TestScan_NoLockFile(t *testing.T) {
	withTempBusDir(t)
	proc, alive, err := Default().Scan()
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if alive || proc != nil {
		t.Fatalf("expected no bus, got alive=%v proc=%+v", alive, proc)
	}
}

func TestScan_StaleLockFile(t *testing.T) {
	dir := withTempBusDir(t)
	// Create the lock file but don't take the flock — that's a "stale" state.
	lockPath := filepath.Join(dir, "event", "bus.alive.lock")
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		t.Fatalf("create lock file: %v", err)
	}
	_ = f.Close()

	proc, alive, err := Default().Scan()
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if alive || proc != nil {
		t.Fatalf("expected no live bus, got alive=%v proc=%+v", alive, proc)
	}
	// The probe must have released the stale lock; verify by re-acquiring.
	probe := filelock.NewProcessLock(lockPath)
	if err := probe.TryLock(); err != nil {
		t.Fatalf("post-scan lock unexpectedly held: %v", err)
	}
	_ = probe.Unlock()
}

func TestScan_LiveLockFile(t *testing.T) {
	withTempBusDir(t)
	// Simulate a running bus by holding the alive lock for the test duration.
	handle, err := WritePIDFile(os.Getpid())
	if err != nil {
		t.Fatalf("WritePIDFile: %v", err)
	}
	t.Cleanup(func() { _ = handle.Release() })

	proc, alive, err := Default().Scan()
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if !alive {
		t.Fatalf("expected alive bus")
	}
	if proc == nil {
		t.Fatalf("expected non-nil Process when alive=true")
	}
	if proc.PID != os.Getpid() {
		t.Errorf("PID mismatch: want %d got %d", os.Getpid(), proc.PID)
	}
	if proc.StartedAt.IsZero() {
		t.Errorf("StartedAt unexpectedly zero")
	}
}

func TestWritePIDFile_Conflict(t *testing.T) {
	withTempBusDir(t)
	first, err := WritePIDFile(os.Getpid())
	if err != nil {
		t.Fatalf("first WritePIDFile: %v", err)
	}
	t.Cleanup(func() { _ = first.Release() })

	if _, err := WritePIDFile(os.Getpid()); err == nil {
		t.Fatalf("second WritePIDFile should fail with ErrHeld")
	}
}
