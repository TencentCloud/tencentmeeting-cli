// status_stop_test.go — cmd-layer end-to-end for `event status` and
// `event stop`.  Spins up an in-process bus (no fork), drives the cobra
// commands, and validates JSON output + post-state on disk.
//
// Owner-binding cases require a logged-in user, simulated via
// keychain.MockKeychain + config.SaveUserConfig.  Without that we couldn't
// reach the running / stale_owner / orphan branching logic at all.

package event

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"tmeet/internal/config"
	"tmeet/internal/core/keychain"
	eventruntime "tmeet/internal/event"
	"tmeet/internal/event/bus"
	"tmeet/internal/event/busctl"
	"tmeet/internal/event/source"
	"tmeet/internal/event/transport"
)

// withCmdTestEnv mirrors internal/event/bus.withTempBusDir but adds a
// keychain mock so config.GetUserConfig succeeds.  Returns the OpenIDHash of
// the simulated user so tests can compare against bus.meta.
func withCmdTestEnv(t *testing.T, openID string) string {
	t.Helper()

	// macOS unix-socket paths cap at 104 bytes; t.TempDir() under
	// /var/folders/... routinely exceeds that.  Use os.MkdirTemp on /tmp.
	dir, err := os.MkdirTemp("", "tmeet-cmd-*")
	if err != nil {
		t.Fatalf("mkdtemp: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	t.Setenv("TMEET_CLI_CONFIG_DIR", dir)
	if err := os.MkdirAll(filepath.Join(dir, "event"), 0700); err != nil {
		t.Fatalf("mkdir event: %v", err)
	}

	if openID != "" {
		mock := keychain.NewMockKeychain()
		config.SetKeychain(mock)
		config.ResetCache()
		t.Cleanup(func() {
			config.SetKeychain(nil)
			config.ResetCache()
		})
		err := config.SaveUserConfig(&config.UserConfig{
			SdkId:        "sdk-test",
			OpenId:       openID,
			AccessToken:  "test-access",
			RefreshToken: "test-refresh",
			Expires:      time.Now().Add(time.Hour).Unix(),
		})
		if err != nil {
			t.Fatalf("SaveUserConfig: %v", err)
		}
	}

	return eventruntime.OpenIDHash(openID)
}

// startBusInProcess starts a bus daemon for the lifetime of the test.
// Returns a cancel func; callers should call it via t.Cleanup.
func startBusInProcess(t *testing.T, ownerHash string) context.CancelFunc {
	t.Helper()
	b := bus.New(bus.Config{
		OpenIDHash:  ownerHash,
		BusVersion:  "test",
		Source:      []source.Source{noopBlocker{}},
		IdleTimeout: 30 * time.Second,
	})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = b.Run(ctx)
	}()

	// Wait until accepting.
	deadline := time.Now().Add(2 * time.Second)
	for {
		if err := busctl.Ping(transport.New()); err == nil {
			break
		}
		if time.Now().After(deadline) {
			cancel()
			<-done
			t.Fatalf("bus did not start within 2s")
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Errorf("bus did not exit within 3s")
		}
	})
	return cancel
}

// noopBlocker is a Source.Run that just blocks until ctx cancels.  Inlined
// here (rather than reused from bus_e2e_test.go) because that test file is
// in package bus_test, not importable from cmd/event.
type noopBlocker struct{}

func (noopBlocker) Name() string { return "noop" }
func (noopBlocker) Run(ctx context.Context, _ func(*eventruntime.RawEvent), _ source.StatusNotifier) error {
	<-ctx.Done()
	return nil
}

// captureCmd runs a cobra RunE with stdout / stderr buffered.  Returns the
// stdout buffer (most tests parse it as JSON), stderr buffer, and the err.
//
// Uses *safeBuffer (not *bytes.Buffer) so concurrent reads in
// waitForReadyMarker do not race the cobra-driven writes — see
// safebuffer_test.go for the rationale.
func captureCmd(t *testing.T, fn func(cmd *cobra.Command) error) (*safeBuffer, *safeBuffer, error) {
	t.Helper()
	stdout := newSafeBuffer()
	stderr := newSafeBuffer()
	cmd := &cobra.Command{}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	err := fn(cmd)
	return stdout, stderr, err
}

// captureExitCode runs fn, recovers an exitPanic emitted by the test
// exitFunc stub (installed via SetExitFuncForTest), and returns the
// requested exit code.  When fn returns normally without exit, code is
// reported as 0.
func captureExitCode(t *testing.T, fn func()) int {
	t.Helper()
	restore := SetExitFuncForTest()
	defer restore()

	code := 0
	func() {
		defer func() {
			if r := recover(); r != nil {
				ep, ok := r.(exitPanic)
				if !ok {
					panic(r) // unrelated panic
				}
				code = ep.Code
			}
		}()
		fn()
	}()
	return code
}

// parseStatusOutput is a small JSON helper.
func parseStatusOutput(t *testing.T, raw []byte) statusOutput {
	t.Helper()
	var out statusOutput
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal status output: %v\nraw=%q", err, raw)
	}
	return out
}

func parseStopOutput(t *testing.T, raw []byte) stopOutput {
	t.Helper()
	var out stopOutput
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal stop output: %v\nraw=%q", err, raw)
	}
	return out
}

// firstStopResult is a tiny convenience to assert "exactly one entry" and
// pull it out — every stop test checks the same shape.
func firstStopResult(t *testing.T, out stopOutput) stopResult {
	t.Helper()
	if len(out.Results) != 1 {
		t.Fatalf("want 1 result, got %d (%+v)", len(out.Results), out)
	}
	return out.Results[0]
}

// ------------------------- status tests -------------------------

func TestStatus_NoBus(t *testing.T) {
	withCmdTestEnv(t, "")

	stdout, _, err := captureCmd(t, func(cmd *cobra.Command) error {
		return runStatus(cmd, false)
	})
	if err != nil {
		t.Fatalf("runStatus: %v", err)
	}
	out := parseStatusOutput(t, stdout.Bytes())
	if len(out.Buses) != 0 {
		t.Errorf("expected empty buses, got %+v", out)
	}
}

func TestStatus_Running(t *testing.T) {
	hash := withCmdTestEnv(t, "user_aaa")
	startBusInProcess(t, hash)

	stdout, _, err := captureCmd(t, func(cmd *cobra.Command) error {
		return runStatus(cmd, false)
	})
	if err != nil {
		t.Fatalf("runStatus: %v", err)
	}
	out := parseStatusOutput(t, stdout.Bytes())
	if len(out.Buses) != 1 {
		t.Fatalf("want 1 bus, got %d (%+v)", len(out.Buses), out)
	}
	b := out.Buses[0]
	if b.State != stateRunning {
		t.Errorf("state = %q, want %q", b.State, stateRunning)
	}
	if b.OpenIDHash != hash {
		t.Errorf("OpenIDHash = %q, want %q", b.OpenIDHash, hash)
	}
	if !b.IsActiveLogin {
		t.Errorf("IsActiveLogin should be true")
	}
}

func TestStatus_StaleOwner_DifferentLogin_v2(t *testing.T) {
	// Set up env + user X, start bus bound to user X, then swap UserConfig
	// to user Y in-place to simulate a re-login.
	hashX := withCmdTestEnv(t, "user_X")
	startBusInProcess(t, hashX)

	// Swap to user Y without disturbing the config dir / running bus.
	hashY := eventruntime.OpenIDHash("user_Y")
	if hashX == hashY {
		t.Fatalf("hash collision between user_X and user_Y; pick different inputs")
	}
	if err := config.SaveUserConfig(&config.UserConfig{
		SdkId:        "sdk-test",
		OpenId:       "user_Y",
		AccessToken:  "y-access",
		RefreshToken: "y-refresh",
		Expires:      time.Now().Add(time.Hour).Unix(),
	}); err != nil {
		t.Fatalf("SaveUserConfig: %v", err)
	}
	config.ResetCache()

	stdout, _, err := captureCmd(t, func(cmd *cobra.Command) error {
		return runStatus(cmd, false)
	})
	if err != nil {
		t.Fatalf("runStatus: %v", err)
	}
	out := parseStatusOutput(t, stdout.Bytes())
	if len(out.Buses) != 1 {
		t.Fatalf("want 1 bus got %d", len(out.Buses))
	}
	b := out.Buses[0]
	if b.State != stateStaleOwner {
		t.Errorf("state = %q, want %q", b.State, stateStaleOwner)
	}
	if b.IsActiveLogin {
		t.Errorf("IsActiveLogin should be false for stale_owner")
	}
}

func TestStatus_StaleOwner_NotLoggedIn(t *testing.T) {
	hash := withCmdTestEnv(t, "user_X")
	startBusInProcess(t, hash)

	// Wipe the user config to simulate logout-while-bus-running.
	if err := config.ClearUserConfig(); err != nil {
		t.Fatalf("ClearUserConfig: %v", err)
	}
	config.ResetCache()

	stdout, _, err := captureCmd(t, func(cmd *cobra.Command) error {
		return runStatus(cmd, false)
	})
	if err != nil {
		t.Fatalf("runStatus: %v", err)
	}
	out := parseStatusOutput(t, stdout.Bytes())
	if len(out.Buses) != 1 || out.Buses[0].State != stateStaleOwner {
		t.Errorf("want 1 stale_owner, got %+v", out)
	}
}

func TestStatus_Orphan(t *testing.T) {
	hash := withCmdTestEnv(t, "user_orphan")

	// Simulate orphan state: bus.meta exists but no alive lock holder.
	meta := eventruntime.NewBusMeta(hash, "test", 99999)
	if err := eventruntime.WriteBusMeta(meta); err != nil {
		t.Fatalf("WriteBusMeta: %v", err)
	}

	stdout, _, err := captureCmd(t, func(cmd *cobra.Command) error {
		return runStatus(cmd, false)
	})
	if err != nil {
		t.Fatalf("runStatus: %v", err)
	}
	out := parseStatusOutput(t, stdout.Bytes())
	if len(out.Buses) != 1 || out.Buses[0].State != stateOrphan {
		t.Errorf("want 1 orphan, got %+v", out)
	}
}

func TestStatus_FailOnOrphan_DoesNotExitOnRunning(t *testing.T) {
	hash := withCmdTestEnv(t, "user_aaa")
	startBusInProcess(t, hash)

	// --fail-on-orphan should NOT exit because state==running.
	stdout, _, err := captureCmd(t, func(cmd *cobra.Command) error {
		return runStatus(cmd, true)
	})
	if err != nil {
		t.Fatalf("runStatus: %v", err)
	}
	out := parseStatusOutput(t, stdout.Bytes())
	if out.Buses[0].State != stateRunning {
		t.Errorf("state = %q, want %q", out.Buses[0].State, stateRunning)
	}
}

// ------------------------- stop tests -------------------------

func TestStop_NoBus(t *testing.T) {
	withCmdTestEnv(t, "")

	stdout, _, err := captureCmd(t, func(cmd *cobra.Command) error {
		return runStop(cmd, false, defaultStopTimeout)
	})
	if err != nil {
		t.Fatalf("runStop: %v", err)
	}
	r := firstStopResult(t, parseStopOutput(t, stdout.Bytes()))
	if r.State != stopStateNoBus {
		t.Errorf("state = %q, want %q", r.State, stopStateNoBus)
	}
}

func TestStop_GracefulShutdown(t *testing.T) {
	hash := withCmdTestEnv(t, "user_stop")
	startBusInProcess(t, hash)

	// noopBlocker bus has no active consumer, so refused-protection
	// doesn't kick in even without --force.
	stdout, _, err := captureCmd(t, func(cmd *cobra.Command) error {
		return runStop(cmd, false, 3*time.Second)
	})
	if err != nil {
		t.Fatalf("runStop: %v", err)
	}
	r := firstStopResult(t, parseStopOutput(t, stdout.Bytes()))
	if r.State != stopStateStopped {
		t.Errorf("state = %q, want %q (out=%+v)", r.State, stopStateStopped, r)
	}
	if r.Forced {
		t.Errorf("Forced should be false on plain graceful stop, got true")
	}

	// Bus should be unreachable now.
	if err := busctl.Ping(transport.New()); err == nil {
		t.Errorf("bus still reachable after stop")
	} else if !errors.Is(err, busctl.ErrNotRunning) {
		t.Errorf("expected ErrNotRunning got %v", err)
	}

	// The on-disk artefacts WriteBusMeta wrote should be gone (Bus.Run
	// defers RemoveBusMeta + RemovePIDFile).
	if _, err := os.Stat(eventruntime.BusMetaFile()); !os.IsNotExist(err) {
		t.Errorf("bus.meta should be removed after graceful stop, got err=%v", err)
	}
	if _, err := os.Stat(eventruntime.BusPIDFile()); !os.IsNotExist(err) {
		t.Errorf("bus.pid should be removed after graceful stop, got err=%v", err)
	}
}

func TestStop_OrphanWithoutForce_RefusesAndExitsTwo(t *testing.T) {
	hash := withCmdTestEnv(t, "user_orphan")
	meta := eventruntime.NewBusMeta(hash, "test", 99999)
	if err := eventruntime.WriteBusMeta(meta); err != nil {
		t.Fatalf("WriteBusMeta: %v", err)
	}

	// Pre-create the stdout buffer + cobra cmd outside captureExitCode so
	// the buffer survives even if exitFunc panics mid-write.  exitFunc
	// fires AFTER output.EventPrint has flushed JSON to cmd.OutOrStdout,
	// so stdoutBuf will already hold the result by the time we recover.
	stdoutBuf := newSafeBuffer()
	cmd := &cobra.Command{}
	cmd.SetOut(stdoutBuf)
	cmd.SetErr(newSafeBuffer())

	code := captureExitCode(t, func() {
		_ = runStop(cmd, false /*force*/, 1*time.Second)
	})
	if code != exitCodeOrphan {
		t.Errorf("exit code = %d, want %d", code, exitCodeOrphan)
	}
	r := firstStopResult(t, parseStopOutput(t, stdoutBuf.Bytes()))
	if r.State != stopStateRefused {
		t.Errorf("state = %q, want %q", r.State, stopStateRefused)
	}
	if _, err := os.Stat(eventruntime.BusMetaFile()); err != nil {
		t.Errorf("bus.meta should remain (no --force), got err=%v", err)
	}
}

func TestStop_OrphanWithForce_Scrubs(t *testing.T) {
	hash := withCmdTestEnv(t, "user_orphan")
	meta := eventruntime.NewBusMeta(hash, "test", 99999)
	if err := eventruntime.WriteBusMeta(meta); err != nil {
		t.Fatalf("WriteBusMeta: %v", err)
	}
	// Also drop a fake pid file so we test multi-file scrub.
	pidPath := eventruntime.BusPIDFile()
	if err := os.WriteFile(pidPath, []byte("99999\n2026-06-15T00:00:00Z\n"), 0600); err != nil {
		t.Fatalf("write fake bus.pid: %v", err)
	}

	stdout, _, err := captureCmd(t, func(cmd *cobra.Command) error {
		return runStop(cmd, true, 1*time.Second)
	})
	if err != nil {
		t.Fatalf("runStop: %v", err)
	}
	r := firstStopResult(t, parseStopOutput(t, stdout.Bytes()))
	if r.State != stopStateStopped {
		t.Errorf("state = %q, want %q", r.State, stopStateStopped)
	}
	if !r.Forced {
		t.Errorf("Forced should be true on --force scrub")
	}
	if _, err := os.Stat(eventruntime.BusMetaFile()); !os.IsNotExist(err) {
		t.Errorf("bus.meta should be removed, got err=%v", err)
	}
	if _, err := os.Stat(eventruntime.BusPIDFile()); !os.IsNotExist(err) {
		t.Errorf("bus.pid should be removed, got err=%v", err)
	}
}

func TestStop_StaleOwnerWithoutForce_RefusesAndExitsTwo(t *testing.T) {
	// Same dance as TestStatus_StaleOwner_DifferentLogin_v2: bind bus to X,
	// then swap UserConfig to Y.  runStop should refuse without --force.
	hashX := withCmdTestEnv(t, "user_X")
	startBusInProcess(t, hashX)

	if err := config.SaveUserConfig(&config.UserConfig{
		SdkId:        "sdk-test",
		OpenId:       "user_Y",
		AccessToken:  "y",
		RefreshToken: "y",
		Expires:      time.Now().Add(time.Hour).Unix(),
	}); err != nil {
		t.Fatalf("SaveUserConfig: %v", err)
	}
	config.ResetCache()

	stdoutBuf := newSafeBuffer()
	cmd := &cobra.Command{}
	cmd.SetOut(stdoutBuf)
	cmd.SetErr(newSafeBuffer())

	code := captureExitCode(t, func() {
		_ = runStop(cmd, false, 1*time.Second)
	})
	if code != exitCodeOrphan {
		t.Errorf("exit code = %d, want %d", code, exitCodeOrphan)
	}
	r := firstStopResult(t, parseStopOutput(t, stdoutBuf.Bytes()))
	if r.State != stopStateRefused {
		t.Errorf("state = %q, want %q (out=%+v)", r.State, stopStateRefused, r)
	}
	// Bus should still be running.
	if err := busctl.Ping(transport.New()); err != nil {
		t.Errorf("bus should still be running, got err=%v", err)
	}
}

// TestStop_RunningWithConsumer_RefusesWithoutForce — refused-protection
// regression for B2:
//
// When the bus has at least one active consumer, plain `event stop`
// (no --force) MUST refuse with state=refused, ConsumerCount populated,
// and exit code 2 — without sending Shutdown to the bus.  The test
// attaches a real consume loop, waits for the ready marker, then drives
// stop and asserts both the JSON shape and the bus is still running.
func TestStop_RunningWithConsumer_RefusesWithoutForce(t *testing.T) {
	hash := withCmdTestEnv(t, "user_consumer")
	trigger, _ := startBusForConsume(t, hash)
	_ = trigger // event triggering not needed; we just want a live subscriber

	// Attach a consumer and wait for ready.  runConsumeLoopAsync is
	// defined in consume_test.go in the same package.
	_, stderr, wait, cancelConsume := runConsumeLoopAsync(t, &consumeOpts{
		EventKey:   "meeting.started",
		Timeout:    5 * time.Second,
		BusVersion: "test",
	}, hash)
	t.Cleanup(cancelConsume)
	waitForReadyMarker(t, stderr)

	// Now drive stop without --force.  Pre-create the buffer so the JSON
	// survives the panic-based exit capture.
	stdoutBuf := newSafeBuffer()
	cmd := &cobra.Command{}
	cmd.SetOut(stdoutBuf)
	cmd.SetErr(newSafeBuffer())

	code := captureExitCode(t, func() {
		_ = runStop(cmd, false /*force*/, 1*time.Second)
	})
	if code != exitCodeOrphan {
		t.Errorf("exit code = %d, want %d", code, exitCodeOrphan)
	}

	r := firstStopResult(t, parseStopOutput(t, stdoutBuf.Bytes()))
	if r.State != stopStateRefused {
		t.Errorf("state = %q, want %q (r=%+v)", r.State, stopStateRefused, r)
	}
	if r.ConsumerCount < 1 {
		t.Errorf("ConsumerCount = %d, want >= 1", r.ConsumerCount)
	}
	if r.Forced {
		t.Errorf("Forced should be false on refused")
	}

	// Bus must still be reachable — refused does NOT send Shutdown.
	if err := busctl.Ping(transport.New()); err != nil {
		t.Errorf("bus should still be reachable after refused stop, got %v", err)
	}

	// Tear down the consumer so cleanup chain is clean.
	cancelConsume()
	if err := wait(); err != nil && !errors.Is(err, context.Canceled) {
		t.Logf("consumer wait returned: %v (acceptable on cancel)", err)
	}
}

// TestStop_RunningWithConsumer_ForceEvicts — companion to the refused
// test above: with --force, stop succeeds, ConsumersEvicted reflects the
// pre-stop consumer count, and the bus is gone.
func TestStop_RunningWithConsumer_ForceEvicts(t *testing.T) {
	hash := withCmdTestEnv(t, "user_consumer_force")
	trigger, _ := startBusForConsume(t, hash)
	_ = trigger

	_, stderr, wait, cancelConsume := runConsumeLoopAsync(t, &consumeOpts{
		EventKey:   "meeting.started",
		Timeout:    5 * time.Second,
		BusVersion: "test",
	}, hash)
	t.Cleanup(cancelConsume)
	waitForReadyMarker(t, stderr)

	stdout, _, err := captureCmd(t, func(cmd *cobra.Command) error {
		return runStop(cmd, true /*force*/, 3*time.Second)
	})
	if err != nil {
		t.Fatalf("runStop --force: %v", err)
	}

	r := firstStopResult(t, parseStopOutput(t, stdout.Bytes()))
	if r.State != stopStateStopped {
		t.Errorf("state = %q, want %q (r=%+v)", r.State, stopStateStopped, r)
	}
	if !r.Forced {
		t.Errorf("Forced should be true under --force")
	}
	if r.ConsumersEvicted < 1 {
		t.Errorf("ConsumersEvicted = %d, want >= 1", r.ConsumersEvicted)
	}

	// Bus must be gone.
	if err := busctl.Ping(transport.New()); err == nil {
		t.Errorf("bus still reachable after --force stop")
	} else if !errors.Is(err, busctl.ErrNotRunning) {
		t.Errorf("expected ErrNotRunning got %v", err)
	}

	// Wait for the consumer to wind down (the bus pushed Bye on its way out).
	cancelConsume()
	if err := wait(); err != nil && !errors.Is(err, context.Canceled) {
		t.Logf("consumer wait returned: %v (acceptable on cancel)", err)
	}
}
