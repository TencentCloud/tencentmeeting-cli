package update

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"tmeet/internal"
	"tmeet/internal/config"
	"tmeet/internal/core/filelock"
	"tmeet/internal/output"

	"github.com/spf13/cobra"
)

// -----------------------------------------------------------------------------
// Test helpers kept here because the slim cmd layer only needs these two.
// All the broader archive / download / npm / prefix / skill helpers now
// live in internal/upgrade and are tested there.
// -----------------------------------------------------------------------------

// newUpdateTestCmd returns a child cobra command whose root exposes
// the --format persistent flag that output.FormatPrint / GetFormat
// require, with both stdout and stderr pointed at caller-owned
// buffers.
//
// The structure mirrors output/print_test.go's newTestCmd — if that
// helper is kept in sync (persistent flag + out/err redirection),
// these tests will stay compatible with future output-layer refactors.
func newUpdateTestCmd(t *testing.T) (*cobra.Command, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	root := &cobra.Command{Use: "tmeet"}
	root.PersistentFlags().String("format", "", "output format")
	child := &cobra.Command{Use: "run"}
	root.AddCommand(child)

	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	child.SetOut(&stdout)
	child.SetErr(&stderr)
	// Give the child a context so cmd.Context() returns non-nil for
	// the consumers that bind timeouts off of it.
	child.SetContext(context.Background())
	return child, &stdout, &stderr
}

// isolateConfigDir redirects config.GetConfigDir() to a per-test
// tempdir by setting TMEET_CLI_CONFIG_DIR.  Returns the directory so
// the test can inspect / seed it (e.g. drop a sentinel under a specific
// openId).
func isolateConfigDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("TMEET_CLI_CONFIG_DIR", dir)
	return dir
}

// -----------------------------------------------------------------------------
// respondAlreadyLatest
// -----------------------------------------------------------------------------

func Test_respondAlreadyLatest(t *testing.T) {
	cmd, stdout, _ := newUpdateTestCmd(t)
	if err := respondAlreadyLatest(cmd, "1.0.0", "1.0.0"); err != nil {
		t.Fatalf("respondAlreadyLatest: %v", err)
	}

	// stdout holds a single-line JSON envelope.  Decode and inspect.
	var env struct {
		Message string                 `json:"message"`
		Data    map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(stdout.Bytes()), &env); err != nil {
		t.Fatalf("stdout is not valid JSON: %v, raw=%q", err, stdout.String())
	}
	if env.Message != "success" {
		t.Fatalf("expected message=success, got %q", env.Message)
	}
	if env.Data["status"] != "already_latest" {
		t.Fatalf("expected status=already_latest, got %v", env.Data["status"])
	}
	if env.Data["current_version"] != "1.0.0" || env.Data["target_version"] != "1.0.0" {
		t.Fatalf("version fields wrong: %+v", env.Data)
	}
}

// -----------------------------------------------------------------------------
// Run end-to-end: early-exit paths that do not require real npm / network
// -----------------------------------------------------------------------------

// newRunOptionsForTest wires up a RunOptions against the isolated
// config dir. Unlike the previous iteration there is no local
// version-info cache to seed anymore — `update run` always fetches
// against the server. Tests that need the fetch to succeed must stand
// up their own HTTP mock; tests exercising lock-held / fetch-failure
// flows can leave the stub alone so the fetch is intentionally made
// to fail against an un-initialised REST client.
func newRunOptionsForTest(t *testing.T, cliVersion string) (*RunOptions, *cobra.Command, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	_ = isolateConfigDir(t)

	const openId = "test_user_123"
	cmd, stdout, stderr := newUpdateTestCmd(t)
	opts := &RunOptions{
		tmeet: &internal.Tmeet{
			UserConfig: &config.UserConfig{OpenId: openId},
			CLIVersion: cliVersion,
		},
	}
	return opts, cmd, stdout, stderr
}

func Test_Run_LockHeldFailsFast(t *testing.T) {
	opts, cmd, _, _ := newRunOptionsForTest(t, "1.0.0")

	// Grab the update lock from "another" process (same PID is fine;
	// flock returns ErrHeld to any additional TryLock on the same
	// inode).
	lockPath := filepath.Join(config.GetConfigDir(), updateLockFileName)
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o700); err != nil {
		t.Fatalf("prepare lock dir: %v", err)
	}
	other := filelock.NewProcessLock(lockPath)
	if err := other.TryLock(); err != nil {
		t.Fatalf("seed lock: %v", err)
	}
	defer func() { _ = other.Unlock() }()

	err := opts.Run(cmd, nil)
	if err == nil {
		t.Fatalf("expected lock-held error")
	}
	if !strings.Contains(err.Error(), "already in progress") {
		t.Fatalf("expected 'already in progress' error, got %v", err)
	}
}

// -----------------------------------------------------------------------------
// Ensure output package import is actually used (silences unused-import
// linters that misread conditional usage above).
// -----------------------------------------------------------------------------
var _ = output.WithoutUpdateNotice
