// consume_test.go — end-to-end coverage for `event consume`.
//
// Strategy: spin an in-process Bus + scriptedSource (mirroring the helpers
// in internal/event/bus/bus_e2e_test.go but inlined here because that file
// is package bus_test, not importable).  Drive runConsumeLoop directly so
// we avoid having to spawn the real `_bus` binary — the ensureBus logic is
// already covered by busctl's Ping path in status_stop_test.go.

package event

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"tmeet/internal"
	eventruntime "tmeet/internal/event"
	"tmeet/internal/event/bus"
	"tmeet/internal/event/busctl"
	"tmeet/internal/event/jqfilter"
	"tmeet/internal/event/source"
	"tmeet/internal/event/transport"
	"tmeet/internal/exception"
)

// ---------- in-test source -------------------------------------------------

// scriptedConsumeSource is a Source whose Run blocks until ctx cancels and
// emits whatever is pushed on its trigger channel.  Inlined (not reused
// from bus_e2e_test.go) because that file lives in package bus_test.
type scriptedConsumeSource struct {
	trigger chan *eventruntime.RawEvent
}

func newScriptedConsumeSource() *scriptedConsumeSource {
	return &scriptedConsumeSource{trigger: make(chan *eventruntime.RawEvent, 16)}
}

func (s *scriptedConsumeSource) Name() string { return "scripted" }

func (s *scriptedConsumeSource) Run(ctx context.Context, emit func(*eventruntime.RawEvent), notify source.StatusNotifier) error {
	if notify != nil {
		notify("steady", "ready")
	}
	for {
		select {
		case <-ctx.Done():
			return nil
		case ev := <-s.trigger:
			if emit != nil {
				emit(ev)
			}
		}
	}
}

// ---------- helpers ---------------------------------------------------------

// startBusForConsume spins a bus bound to ownerHash and returns the source
// trigger channel + cleanup.  Mirrors startBusInProcess from status_stop_test
// but exposes the source so tests can push events.
func startBusForConsume(t *testing.T, ownerHash string) (chan<- *eventruntime.RawEvent, context.CancelFunc) {
	t.Helper()
	src := newScriptedConsumeSource()
	b := bus.New(bus.Config{
		OpenIDHash:  ownerHash,
		BusVersion:  "test",
		Source:      []source.Source{src},
		IdleTimeout: 30 * time.Second,
	})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = b.Run(ctx)
	}()

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
	return src.trigger, cancel
}

// runConsumeLoopAsync runs the loop in a goroutine and returns:
//   - stdout / stderr buffers for inspection,
//   - a wait func that blocks until the loop returns,
//   - a cancel func that signals graceful shutdown.
func runConsumeLoopAsync(t *testing.T, opts *consumeOpts, ownerHash string) (
	stdout, stderr *safeBuffer,
	wait func() error,
	cancel context.CancelFunc,
) {
	t.Helper()
	stdout = newSafeBuffer()
	stderr = newSafeBuffer()
	cmd := &cobra.Command{}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	ctx, cancelFn := context.WithCancel(context.Background())
	cmd.SetContext(ctx)

	errCh := make(chan error, 1)
	go func() {
		errCh <- runConsumeLoop(ctx, cmd, opts, transport.New(), ownerHash)
	}()
	wait = func() error {
		select {
		case err := <-errCh:
			return err
		case <-time.After(5 * time.Second):
			t.Errorf("runConsumeLoop did not return within 5s")
			return errors.New("timed out waiting for runConsumeLoop")
		}
	}
	return stdout, stderr, wait, cancelFn
}

// waitForReadyMarker blocks until the stderr buffer contains the ready
// marker, or fails the test on timeout.  Required because runConsumeLoop's
// startup is async — a test that pushes an event before ready can race.
func waitForReadyMarker(t *testing.T, stderr *safeBuffer) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(stderr.String(), "[event] ready event_key=") {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("ready marker not seen in stderr within 3s; stderr=%q", stderr.String())
}

// parseFirstNDJSON returns the first JSON object on stdout (one per line).
func parseFirstNDJSON(t *testing.T, stdout *safeBuffer) map[string]interface{} {
	t.Helper()
	sc := bufio.NewScanner(bytes.NewReader(stdout.Bytes()))
	if !sc.Scan() {
		t.Fatalf("no NDJSON line on stdout; stdout=%q", stdout.String())
	}
	var m map[string]interface{}
	if err := json.Unmarshal(sc.Bytes(), &m); err != nil {
		t.Fatalf("unmarshal stdout line: %v\nraw=%q", err, sc.Bytes())
	}
	return m
}

// ---------- tests -----------------------------------------------------------

func TestConsume_RejectsUnknownEventKey(t *testing.T) {
	withCmdTestEnv(t, "user_aaa")

	cmd := &cobra.Command{}
	stdout := newSafeBuffer()
	stderr := newSafeBuffer()
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetContext(context.Background())

	err := runConsume(cmd, &consumeOpts{
		EventKey:   "this.does.not.exist",
		BusVersion: "test",
	})
	if err == nil {
		t.Fatal("runConsume should fail for unknown EventKey")
	}
	if !strings.Contains(err.Error(), "unknown EventKey") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestConsume_RejectsAbsoluteOutputDir(t *testing.T) {
	withCmdTestEnv(t, "user_aaa")

	cmd := &cobra.Command{}
	cmd.SetOut(newSafeBuffer())
	cmd.SetErr(newSafeBuffer())
	cmd.SetContext(context.Background())

	err := runConsume(cmd, &consumeOpts{
		EventKey:   "meeting.started",
		OutputDir:  "/tmp/abs",
		BusVersion: "test",
	})
	if err == nil || !strings.Contains(err.Error(), "must be relative") {
		t.Errorf("expected 'must be relative' error, got: %v", err)
	}
}

func TestConsume_RejectsParentInOutputDir(t *testing.T) {
	withCmdTestEnv(t, "user_aaa")

	cmd := &cobra.Command{}
	cmd.SetOut(newSafeBuffer())
	cmd.SetErr(newSafeBuffer())
	cmd.SetContext(context.Background())

	err := runConsume(cmd, &consumeOpts{
		EventKey:   "meeting.started",
		OutputDir:  "a/../../b",
		BusVersion: "test",
	})
	if err == nil || !strings.Contains(err.Error(), "must not contain '..'") {
		t.Errorf("expected '..' rejection, got: %v", err)
	}
}

func TestConsume_HappyPath_MaxEventsLimit(t *testing.T) {
	hash := withCmdTestEnv(t, "user_consume")
	trigger, _ := startBusForConsume(t, hash)

	stdout, stderr, wait, cancel := runConsumeLoopAsync(t, &consumeOpts{
		EventKey:   "meeting.started",
		MaxEvents:  1,
		BusVersion: "test",
	}, hash)
	defer cancel()

	waitForReadyMarker(t, stderr)

	payload, _ := json.Marshal(map[string]string{"hello": "world"})
	trigger <- &eventruntime.RawEvent{
		Event:   "meeting.started",
		TraceID: "trc_test_001",
		Payload: payload,
	}

	if err := wait(); err != nil {
		t.Fatalf("runConsumeLoop returned error: %v", err)
	}

	// stdout: exactly one NDJSON line, matching schema.
	got := parseFirstNDJSON(t, stdout)
	if got["event"] != "meeting.started" {
		t.Errorf("event = %v, want meeting.started", got["event"])
	}
	if got["trace_id"] != "trc_test_001" {
		t.Errorf("trace_id = %v, want trc_test_001", got["trace_id"])
	}
	if _, ok := got["payload"]; !ok {
		t.Errorf("payload missing from output: %+v", got)
	}
	// stdout must NOT carry the wire-level "type" field.
	if _, ok := got["type"]; ok {
		t.Errorf("stdout NDJSON should not include the wire 'type' field: %+v", got)
	}

	// stderr: ready marker + exit line with reason=limit.
	if !strings.Contains(stderr.String(), "[event] ready event_key=meeting.started") {
		t.Errorf("stderr missing ready marker: %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), "reason: limit") {
		t.Errorf("stderr missing 'reason: limit': %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), "received 1 event(s)") {
		t.Errorf("stderr missing 'received 1 event(s)': %q", stderr.String())
	}
}

func TestConsume_TimeoutTriggersGracefulExit(t *testing.T) {
	hash := withCmdTestEnv(t, "user_to")
	startBusForConsume(t, hash)

	stdout, stderr, wait, cancel := runConsumeLoopAsync(t, &consumeOpts{
		EventKey:   "meeting.started",
		Timeout:    300 * time.Millisecond,
		BusVersion: "test",
	}, hash)
	defer cancel()

	waitForReadyMarker(t, stderr)

	if err := wait(); err != nil {
		t.Fatalf("runConsumeLoop returned error: %v", err)
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout should be empty on timeout, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "reason: timeout") {
		t.Errorf("stderr missing 'reason: timeout': %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), "received 0 event(s)") {
		t.Errorf("stderr missing 'received 0 event(s)': %q", stderr.String())
	}
}

func TestConsume_QuietSuppressesInformationalButKeepsReadyAndExit(t *testing.T) {
	hash := withCmdTestEnv(t, "user_quiet")
	trigger, _ := startBusForConsume(t, hash)

	stdout, stderr, wait, cancel := runConsumeLoopAsync(t, &consumeOpts{
		EventKey:   "meeting.started",
		MaxEvents:  1,
		Quiet:      true,
		BusVersion: "test",
	}, hash)
	defer cancel()

	waitForReadyMarker(t, stderr)
	trigger <- &eventruntime.RawEvent{
		Event: "meeting.started", TraceID: "trc_q_001",
		Payload: json.RawMessage(`{}`),
	}
	if err := wait(); err != nil {
		t.Fatalf("runConsumeLoop returned error: %v", err)
	}

	stderrStr := stderr.String()
	if !strings.Contains(stderrStr, "[event] ready event_key=") {
		t.Errorf("--quiet should NOT suppress ready marker: %q", stderrStr)
	}
	if !strings.Contains(stderrStr, "[event] exited") {
		t.Errorf("--quiet should NOT suppress exit line: %q", stderrStr)
	}
	if strings.Contains(stderrStr, "[event] handshake ok") {
		t.Errorf("--quiet should suppress handshake informational: %q", stderrStr)
	}
	if strings.Contains(stderrStr, "[event] received trace_id=") {
		t.Errorf("--quiet should suppress per-event informational: %q", stderrStr)
	}
	// stdout must still carry the event (--quiet only affects stderr).
	if stdout.Len() == 0 {
		t.Errorf("stdout should still receive the event under --quiet")
	}
}

func TestConsume_OutputDirWritesFile(t *testing.T) {
	hash := withCmdTestEnv(t, "user_outdir")
	trigger, _ := startBusForConsume(t, hash)

	// runConsume normally creates the output dir; here we drive
	// runConsumeLoop directly, so it must already exist.  Use a relative
	// path under the per-test working dir so the rest of the contract holds.
	cwd, err := os.MkdirTemp("", "consume-cwd-*")
	if err != nil {
		t.Fatalf("mkdtemp cwd: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(cwd) })
	prevWd, _ := os.Getwd()
	if err := os.Chdir(cwd); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(prevWd) })

	outDir := "events.recv"
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("mkdir outdir: %v", err)
	}

	_, stderr, wait, cancel := runConsumeLoopAsync(t, &consumeOpts{
		EventKey:   "meeting.started",
		MaxEvents:  1,
		OutputDir:  outDir,
		BusVersion: "test",
	}, hash)
	defer cancel()

	waitForReadyMarker(t, stderr)
	trigger <- &eventruntime.RawEvent{
		Event:   "meeting.started",
		TraceID: "trc_outdir_42",
		Payload: json.RawMessage(`{"k":"v"}`),
	}
	if err := wait(); err != nil {
		t.Fatalf("runConsumeLoop returned error: %v", err)
	}

	path := filepath.Join(outDir, "trc_outdir_42.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var got map[string]interface{}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal %s: %v\nraw=%q", path, err, data)
	}
	if got["trace_id"] != "trc_outdir_42" {
		t.Errorf("trace_id in file = %v, want trc_outdir_42", got["trace_id"])
	}
}

func TestConsume_WrongOwnerExitsFatal(t *testing.T) {
	// Bus bound to user X; consumer connects with user Y's hash → WrongOwner.
	hashX := withCmdTestEnv(t, "user_X")
	startBusForConsume(t, hashX)

	hashY := eventruntime.OpenIDHash("user_Y")

	cmd := &cobra.Command{}
	stdout := newSafeBuffer()
	stderr := newSafeBuffer()
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetContext(context.Background())

	err := runConsumeLoop(context.Background(), cmd, &consumeOpts{
		EventKey:   "meeting.started",
		BusVersion: "test",
	}, transport.New(), hashY)
	if err == nil {
		t.Fatal("expected fatal error on WrongOwner")
	}
	if !exception.Is(err, exception.EventBusError) {
		t.Errorf("expected EventBusError, got: %v", err)
	}
	if !strings.Contains(stderr.String(), "bus owner mismatch") {
		t.Errorf("stderr missing owner-mismatch hint: %q", stderr.String())
	}
}

func TestConsume_BusBye_ReasonShutdown(t *testing.T) {
	hash := withCmdTestEnv(t, "user_bye")
	startBusForConsume(t, hash)

	_, stderr, wait, cancel := runConsumeLoopAsync(t, &consumeOpts{
		EventKey:   "meeting.started",
		BusVersion: "test",
	}, hash)
	defer cancel()
	waitForReadyMarker(t, stderr)

	// Asking the bus to shut down causes it to close all conns; consumer
	// should exit with reason=shutdown (not signal/timeout/limit).
	if err := busctl.SendShutdown(transport.New(), false); err != nil {
		t.Fatalf("SendShutdown: %v", err)
	}

	if err := wait(); err != nil {
		t.Fatalf("runConsumeLoop returned error: %v", err)
	}
	if !strings.Contains(stderr.String(), "reason: shutdown") {
		t.Errorf("stderr missing 'reason: shutdown': %q", stderr.String())
	}
}

func TestConsume_SignalCancellation_ReasonSignal(t *testing.T) {
	hash := withCmdTestEnv(t, "user_sig")
	startBusForConsume(t, hash)

	_, stderr, wait, cancel := runConsumeLoopAsync(t, &consumeOpts{
		EventKey:   "meeting.started",
		BusVersion: "test",
	}, hash)
	waitForReadyMarker(t, stderr)

	// Cancel the parent context — runConsumeLoop should exit with reason=signal.
	cancel()
	if err := wait(); err != nil {
		t.Fatalf("runConsumeLoop returned error: %v", err)
	}
	if !strings.Contains(stderr.String(), "reason: signal") {
		t.Errorf("stderr missing 'reason: signal': %q", stderr.String())
	}
}

// ---------- batch 3.1: --param L1 + L2 -------------------------------------

func TestConsume_RejectsParamWithoutEqual(t *testing.T) {
	withCmdTestEnv(t, "user_p1")

	cmd := &cobra.Command{}
	cmd.SetOut(newSafeBuffer())
	cmd.SetErr(newSafeBuffer())
	cmd.SetContext(context.Background())

	err := runConsume(cmd, &consumeOpts{
		EventKey:   "meeting.started",
		ParamsRaw:  []string{"meeting_id"},
		BusVersion: "test",
	})
	if err == nil || !strings.Contains(err.Error(), "key=value") {
		t.Errorf("expected key=value hint, got: %v", err)
	}
}

func TestConsume_RejectsUnknownParamWithSchemaHint(t *testing.T) {
	withCmdTestEnv(t, "user_p2")

	cmd := &cobra.Command{}
	cmd.SetOut(newSafeBuffer())
	cmd.SetErr(newSafeBuffer())
	cmd.SetContext(context.Background())

	err := runConsume(cmd, &consumeOpts{
		EventKey:   "meeting.started",
		ParamsRaw:  []string{"foo=bar"},
		BusVersion: "test",
	})
	if err == nil {
		t.Fatal("expected unknown-param error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "tmeet event schema") {
		t.Errorf("error should suggest 'tmeet event schema': %v", err)
	}
	if !strings.Contains(msg, "foo") {
		t.Errorf("error should name the offending key: %v", err)
	}
}

// TestConsume_ParamsFilterDeliversOnlyMatchingMeetingID validates the L2
// path: bus receives 3 events; only the one whose payload[0].meeting_info.meeting_id
// matches the consumer's --param is delivered.  The other two are silently
// dropped (NOT counted in dropped — they're filter misses, not back-pressure).
func TestConsume_ParamsFilterDeliversOnlyMatchingMeetingID(t *testing.T) {
	hash := withCmdTestEnv(t, "user_pflt")
	trigger, _ := startBusForConsume(t, hash)

	stdout, stderr, wait, cancel := runConsumeLoopAsync(t, &consumeOpts{
		EventKey:   "meeting.started",
		MaxEvents:  1,
		Params:     map[string]string{"meeting_id": "match_me"},
		BusVersion: "test",
	}, hash)
	defer cancel()
	waitForReadyMarker(t, stderr)

	// Push 2 non-matching first, then the match — the consumer should ignore
	// the first two and exit on the third (limit=1).  Payload mirrors the
	// real Tencent webhook shape (length-1 array of {operator,meeting_info}).
	for i, mid := range []string{"other_a", "other_b", "match_me"} {
		payload, _ := json.Marshal([]map[string]interface{}{{
			"operate_time": i,
			"operator":     map[string]interface{}{"userid": "tester"},
			"meeting_info": map[string]interface{}{"meeting_id": mid},
		}})
		trigger <- &eventruntime.RawEvent{
			Event:   "meeting.started",
			TraceID: fmt.Sprintf("trc_%d", i),
			Payload: payload,
		}
	}

	if err := wait(); err != nil {
		t.Fatalf("runConsumeLoop: %v", err)
	}

	// stdout: exactly one line, with payload[0].meeting_info.meeting_id="match_me".
	got := parseFirstNDJSON(t, stdout)
	if got["trace_id"] != "trc_2" {
		t.Errorf("trace_id = %v, want trc_2 (the matching event)", got["trace_id"])
	}
	payloadArr, _ := got["payload"].([]interface{})
	if len(payloadArr) != 1 {
		t.Fatalf("payload should be length-1 array, got %d: %+v", len(payloadArr), got["payload"])
	}
	first, _ := payloadArr[0].(map[string]interface{})
	mi, _ := first["meeting_info"].(map[string]interface{})
	if mi["meeting_id"] != "match_me" {
		t.Errorf("delivered payload[0].meeting_info.meeting_id = %v, want match_me", mi["meeting_id"])
	}
	// Only one NDJSON line on stdout.
	if extra := bytes.Count(stdout.Bytes(), []byte("\n")); extra != 1 {
		t.Errorf("stdout should contain exactly 1 line, got %d: %q", extra, stdout.String())
	}
}

// dotPathFixtureKey is registered once for TestConsume_ParamsFilterDotPath.
// We use a private fixture EventKey rather than meeting.started because the
// shipped meeting.started ParamsSchema deliberately omits a `userid` param
// (the spec doesn't expose per-operator filtering); the test still needs
// to cover "dot-path PayloadPath traversing array → object → leaf via L2
// filtering" though, so we synthesise a key with that exact shape here.
const dotPathFixtureKey = "test.consume_dotpath_fixture"

func init() {
	eventruntime.RegisterKey(eventruntime.KeyDef{
		Key:         dotPathFixtureKey,
		Domain:      "test",
		Description: "internal fixture for TestConsume_ParamsFilterDotPath (not user-visible)",
		JQRootPath:  ".payload",
		ParamsSchema: map[string]eventruntime.ParamDef{
			"userid": {
				Type:        "string",
				Required:    false,
				PayloadPath: "0.operator.userid",
			},
		},
	})
}

// TestConsume_ParamsFilterDotPath confirms PayloadPath="0.operator.userid"
// resolves correctly through nested arrays + objects.  Uses an ad-hoc
// fixture EventKey (see init above) so it doesn't depend on any shipped
// schema's choice of params.
func TestConsume_ParamsFilterDotPath(t *testing.T) {
	hash := withCmdTestEnv(t, "user_pdot")
	trigger, _ := startBusForConsume(t, hash)

	stdout, stderr, wait, cancel := runConsumeLoopAsync(t, &consumeOpts{
		EventKey:   dotPathFixtureKey,
		MaxEvents:  1,
		Params:     map[string]string{"userid": "u_001"},
		BusVersion: "test",
	}, hash)
	defer cancel()
	waitForReadyMarker(t, stderr)

	// Non-matching user, then matching.
	for _, uid := range []string{"u_999", "u_001"} {
		payload, _ := json.Marshal([]map[string]interface{}{{
			"operate_time": 1,
			"operator":     map[string]interface{}{"userid": uid, "instance_id": "2"},
			"meeting_info": map[string]interface{}{"meeting_id": "m1"},
		}})
		trigger <- &eventruntime.RawEvent{
			Event:   dotPathFixtureKey,
			TraceID: "trc_" + uid,
			Payload: payload,
		}
	}
	if err := wait(); err != nil {
		t.Fatalf("runConsumeLoop: %v", err)
	}
	got := parseFirstNDJSON(t, stdout)
	if got["trace_id"] != "trc_u_001" {
		t.Errorf("trace_id = %v, want trc_u_001 (the matching event)", got["trace_id"])
	}
}

// TestConsume_ParamsFilterMissesAllEvents — every emitted event misses the
// filter.  Combined with --timeout, the consumer exits with reason=timeout
// and received=0.  This is exactly the documented consume behaviour:
// filter mismatches do NOT increment delivered counts.
func TestConsume_ParamsFilterMissesAllEvents(t *testing.T) {
	hash := withCmdTestEnv(t, "user_pmiss")
	trigger, _ := startBusForConsume(t, hash)

	stdout, stderr, wait, cancel := runConsumeLoopAsync(t, &consumeOpts{
		EventKey:   "meeting.started",
		Timeout:    300 * time.Millisecond,
		Params:     map[string]string{"meeting_id": "never_matches"},
		BusVersion: "test",
	}, hash)
	defer cancel()
	waitForReadyMarker(t, stderr)

	for i := 0; i < 3; i++ {
		payload, _ := json.Marshal([]map[string]interface{}{{
			"operate_time": i,
			"operator":     map[string]interface{}{"userid": "tester"},
			"meeting_info": map[string]interface{}{"meeting_id": fmt.Sprintf("other_%d", i)},
		}})
		trigger <- &eventruntime.RawEvent{
			Event:   "meeting.started",
			TraceID: fmt.Sprintf("trc_%d", i),
			Payload: payload,
		}
	}

	if err := wait(); err != nil {
		t.Fatalf("runConsumeLoop: %v", err)
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout should be empty when filter misses everything, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "reason: timeout") {
		t.Errorf("stderr missing 'reason: timeout': %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), "received 0 event(s)") {
		t.Errorf("filter misses must NOT count toward received: %q", stderr.String())
	}
}

// ---------- batch 3.2: --jq -----------------------------------------------

// mustCompileJQ is a tiny helper so each test can express the filter in one
// line; failure here is a test-author bug, not a runtime concern.
func mustCompileJQ(t *testing.T, expr string) *jqfilter.Filter {
	t.Helper()
	f, err := jqfilter.Compile(expr)
	if err != nil {
		t.Fatalf("compile %q: %v", expr, err)
	}
	return f
}

func TestConsume_RejectsInvalidJQAtStartup(t *testing.T) {
	withCmdTestEnv(t, "user_jq_synerr")

	cmd := &cobra.Command{}
	stdout := newSafeBuffer()
	stderr := newSafeBuffer()
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetContext(context.Background())

	err := runConsume(cmd, &consumeOpts{
		EventKey:   "meeting.started",
		JQ:         ".foo |", // dangling pipe
		BusVersion: "test",
	})
	if err == nil {
		t.Fatal("expected fatal error for invalid jq")
	}
	if !strings.Contains(err.Error(), "jq compile") {
		t.Errorf("error should mention jq compile failure, got: %v", err)
	}
	// fail-fast contract: ready marker must NOT appear because we never
	// reached the loop.
	if strings.Contains(stderr.String(), "ready event_key=") {
		t.Errorf("ready marker should NOT appear after jq compile failure: %q", stderr.String())
	}
}

// TestConsume_JQIdentityIsTransparent confirms `--jq .` on the envelope
// produces stdout containing the same envelope shape (modulo gojq field
// reordering) as the no-jq path.
func TestConsume_JQIdentityIsTransparent(t *testing.T) {
	hash := withCmdTestEnv(t, "user_jq_id")
	trigger, _ := startBusForConsume(t, hash)

	opts := jqOpts(t, &consumeOpts{
		EventKey:   "meeting.started",
		MaxEvents:  1,
		BusVersion: "test",
	}, ".", ".") // identity; whole envelope as input

	stdout, stderr, wait, cancel := runConsumeLoopAsync(t, opts, hash)
	defer cancel()
	waitForReadyMarker(t, stderr)

	payload, _ := json.Marshal(map[string]string{"meeting_id": "abc"})
	trigger <- &eventruntime.RawEvent{
		Event: "meeting.started", TraceID: "trc_id_001", Payload: payload,
	}
	if err := wait(); err != nil {
		t.Fatalf("runConsumeLoop: %v", err)
	}
	got := parseFirstNDJSON(t, stdout)
	if got["trace_id"] != "trc_id_001" {
		t.Errorf("trace_id = %v, want trc_id_001", got["trace_id"])
	}
	if got["event"] != "meeting.started" {
		t.Errorf("event = %v, want meeting.started", got["event"])
	}
}

// jqOpts wires JQRoot+jqFilter for tests, since the unexported jqFilter
// field can only be populated from inside this package.
func jqOpts(t *testing.T, base *consumeOpts, expr, root string) *consumeOpts {
	t.Helper()
	base.JQ = expr
	base.JQRoot = root
	base.jqFilter = mustCompileJQ(t, expr)
	return base
}

func TestConsume_JQSelectDropsMismatchAndDoesNotCount(t *testing.T) {
	hash := withCmdTestEnv(t, "user_jq_select")
	trigger, _ := startBusForConsume(t, hash)

	opts := jqOpts(t, &consumeOpts{
		EventKey:   "meeting.started",
		MaxEvents:  1,
		BusVersion: "test",
	}, `select(.user.role == "host")`, ".payload")

	stdout, stderr, wait, cancel := runConsumeLoopAsync(t, opts, hash)
	defer cancel()
	waitForReadyMarker(t, stderr)

	// Push two attendee events (filtered out) then one host (matches).
	for _, role := range []string{"attendee", "attendee", "host"} {
		payload, _ := json.Marshal(map[string]interface{}{
			"meeting_id": "m1",
			"user":       map[string]string{"userid": "u", "role": role},
		})
		trigger <- &eventruntime.RawEvent{
			Event: "meeting.started", TraceID: "trc_" + role, Payload: payload,
		}
	}
	if err := wait(); err != nil {
		t.Fatalf("runConsumeLoop: %v", err)
	}

	// Exactly one NDJSON line on stdout — the host event's payload.
	if n := bytes.Count(stdout.Bytes(), []byte("\n")); n != 1 {
		t.Errorf("expected exactly 1 stdout line, got %d: %q", n, stdout.String())
	}
	if !strings.Contains(stdout.String(), `"role":"host"`) {
		t.Errorf("stdout should contain host event, got: %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "received 1 event(s)") {
		t.Errorf("jq drops must NOT count toward received: %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), "reason: limit") {
		t.Errorf("reason should be 'limit', got: %q", stderr.String())
	}
}

func TestConsume_JQProjectionReshapesStdoutLine(t *testing.T) {
	hash := withCmdTestEnv(t, "user_jq_proj")
	trigger, _ := startBusForConsume(t, hash)

	opts := jqOpts(t, &consumeOpts{
		EventKey:   "meeting.started",
		MaxEvents:  1,
		BusVersion: "test",
	}, `{uid: .user.userid, t: .timestamp}`, ".payload")

	stdout, stderr, wait, cancel := runConsumeLoopAsync(t, opts, hash)
	defer cancel()
	waitForReadyMarker(t, stderr)

	payload, _ := json.Marshal(map[string]interface{}{
		"meeting_id": "m1",
		"timestamp":  1717900000,
		"user":       map[string]string{"userid": "u_001", "role": "host"},
	})
	trigger <- &eventruntime.RawEvent{
		Event: "meeting.started", TraceID: "trc_proj", Payload: payload,
	}
	if err := wait(); err != nil {
		t.Fatalf("runConsumeLoop: %v", err)
	}

	line := strings.TrimRight(stdout.String(), "\n")
	var got map[string]interface{}
	if err := json.Unmarshal([]byte(line), &got); err != nil {
		t.Fatalf("unmarshal: %v\nline=%q", err, line)
	}
	if got["uid"] != "u_001" {
		t.Errorf("uid = %v, want u_001", got["uid"])
	}
	if _, ok := got["meeting_id"]; ok {
		t.Errorf("projection should not retain meeting_id, got %+v", got)
	}
	if _, ok := got["trace_id"]; ok {
		t.Errorf("with JQRoot=.payload the envelope's trace_id should not appear, got %+v", got)
	}
}

func TestConsume_JQRuntimeErrorWarnsButContinues(t *testing.T) {
	hash := withCmdTestEnv(t, "user_jq_rterr")
	trigger, _ := startBusForConsume(t, hash)

	// `.[0]` on an object payload is a type error — gojq surfaces it as a
	// runtime error.  Subsequent events shouldn't be affected.
	opts := jqOpts(t, &consumeOpts{
		EventKey:   "meeting.started",
		Timeout:    400 * time.Millisecond,
		BusVersion: "test",
	}, `.[0]`, ".payload")

	stdout, stderr, wait, cancel := runConsumeLoopAsync(t, opts, hash)
	defer cancel()
	waitForReadyMarker(t, stderr)

	payload, _ := json.Marshal(map[string]interface{}{"meeting_id": "m1"})
	for i := 0; i < 2; i++ {
		trigger <- &eventruntime.RawEvent{
			Event: "meeting.started", TraceID: fmt.Sprintf("trc_%d", i), Payload: payload,
		}
	}
	if err := wait(); err != nil {
		t.Fatalf("runConsumeLoop: %v", err)
	}

	if stdout.Len() != 0 {
		t.Errorf("runtime errors must NOT emit stdout, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "WARN jq error") {
		t.Errorf("stderr should contain WARN jq error: %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), "trc_0") {
		t.Errorf("WARN should reference offending trace_id: %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), "received 0 event(s)") {
		t.Errorf("jq runtime errors must NOT count: %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), "reason: timeout") {
		t.Errorf("loop should have continued and exited via timeout: %q", stderr.String())
	}
}

func TestConsume_JQQuietDoesNotSuppressWarn(t *testing.T) {
	hash := withCmdTestEnv(t, "user_jq_quiet")
	trigger, _ := startBusForConsume(t, hash)

	opts := jqOpts(t, &consumeOpts{
		EventKey:   "meeting.started",
		Timeout:    300 * time.Millisecond,
		Quiet:      true,
		BusVersion: "test",
	}, `.[0]`, ".payload")

	_, stderr, wait, cancel := runConsumeLoopAsync(t, opts, hash)
	defer cancel()
	waitForReadyMarker(t, stderr)

	payload, _ := json.Marshal(map[string]interface{}{"meeting_id": "m"})
	trigger <- &eventruntime.RawEvent{
		Event: "meeting.started", TraceID: "trc_q", Payload: payload,
	}
	if err := wait(); err != nil {
		t.Fatalf("runConsumeLoop: %v", err)
	}

	if !strings.Contains(stderr.String(), "WARN jq error") {
		t.Errorf("--quiet must NOT suppress jq runtime WARN: %q", stderr.String())
	}
	// But informational lines stay suppressed.
	if strings.Contains(stderr.String(), "[event] handshake ok") {
		t.Errorf("--quiet should still suppress informational handshake line: %q", stderr.String())
	}
}

func TestConsume_JQGeneratorCountsEachOutput(t *testing.T) {
	hash := withCmdTestEnv(t, "user_jq_gen")
	trigger, _ := startBusForConsume(t, hash)

	// `.users[]` on a single event with 3 users yields 3 outputs; with
	// --max-events=2 the second output should hit the limit and exit.
	opts := jqOpts(t, &consumeOpts{
		EventKey:   "meeting.started",
		MaxEvents:  2,
		BusVersion: "test",
	}, `.users[]`, ".payload")

	stdout, stderr, wait, cancel := runConsumeLoopAsync(t, opts, hash)
	defer cancel()
	waitForReadyMarker(t, stderr)

	payload, _ := json.Marshal(map[string]interface{}{
		"meeting_id": "m1",
		"users":      []map[string]string{{"userid": "u1"}, {"userid": "u2"}, {"userid": "u3"}},
	})
	trigger <- &eventruntime.RawEvent{
		Event: "meeting.started", TraceID: "trc_gen", Payload: payload,
	}
	if err := wait(); err != nil {
		t.Fatalf("runConsumeLoop: %v", err)
	}

	if n := bytes.Count(stdout.Bytes(), []byte("\n")); n != 2 {
		t.Errorf("expected 2 stdout lines (max-events=2), got %d: %q", n, stdout.String())
	}
	if !strings.Contains(stderr.String(), "received 2 event(s)") {
		t.Errorf("each emitted line counts; expected 2 received: %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), "reason: limit") {
		t.Errorf("reason should be 'limit': %q", stderr.String())
	}
}

// TestConsume_JQOutputDirIgnoresJQ verifies the audit-trail rule:
// --output-dir always writes the FULL event, even when --jq drops or
// reshapes the stdout line.
func TestConsume_JQOutputDirIgnoresJQ(t *testing.T) {
	hash := withCmdTestEnv(t, "user_jq_outdir")
	trigger, _ := startBusForConsume(t, hash)

	cwd, err := os.MkdirTemp("", "consume-jqdir-*")
	if err != nil {
		t.Fatalf("mkdtemp: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(cwd) })
	prevWd, _ := os.Getwd()
	if err := os.Chdir(cwd); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(prevWd) })

	outDir := "jq.recv"
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// jq drops every event (always-false select) but output-dir should
	// still get the original payload-bearing record.
	opts := jqOpts(t, &consumeOpts{
		EventKey:   "meeting.started",
		Timeout:    300 * time.Millisecond,
		OutputDir:  outDir,
		BusVersion: "test",
	}, `select(false)`, ".payload")

	_, stderr, wait, cancel := runConsumeLoopAsync(t, opts, hash)
	defer cancel()
	waitForReadyMarker(t, stderr)

	payload, _ := json.Marshal(map[string]interface{}{"meeting_id": "audit_me"})
	trigger <- &eventruntime.RawEvent{
		Event: "meeting.started", TraceID: "trc_audit", Payload: payload,
	}
	if err := wait(); err != nil {
		t.Fatalf("runConsumeLoop: %v", err)
	}

	if !strings.Contains(stderr.String(), "received 0 event(s)") {
		t.Errorf("jq drop must yield 0 received: %q", stderr.String())
	}
	// The on-disk file MUST still exist with the full event.
	data, err := os.ReadFile(filepath.Join(outDir, "trc_audit.json"))
	if err != nil {
		t.Fatalf("output-dir file should exist: %v", err)
	}
	if !strings.Contains(string(data), `"meeting_id": "audit_me"`) {
		t.Errorf("audit file missing original payload: %s", data)
	}
}

// ---------- Args validator -------------------------------------------------
//
// consume replaced cobra.ExactArgs(1) with a hand-rolled validator so
// that the "missing EventKey" error can point users at `tmeet event
// list` and `--help` (mirroring the tone of the "unknown EventKey"
// branch in Run).  The shared assertion helper lives in schema_test.go
// (expectFriendlyArgsError); kept package-level so both command files
// can reuse it without duplicating the contract.
//
// Tests flow through cmd.Execute() rather than calling Run() directly
// so the Args validator is actually exercised — Run() only runs after
// Args passes, so a unit test that bypassed cobra would skip the very
// layer under test.

// runConsumeArgs builds the real cobra command via newConsumeCmd and
// runs Execute() with the given argv.  internal.Tmeet is zero-valued
// because the Args validator runs strictly before RunE, so no field
// is touched.
func runConsumeArgs(t *testing.T, argv []string) error {
	t.Helper()
	c := newConsumeCmd(&internal.Tmeet{CLIVersion: "test"})
	c.SetOut(&bytes.Buffer{})
	c.SetErr(&bytes.Buffer{})
	c.SetContext(context.Background())
	c.SilenceUsage = true
	c.SilenceErrors = true
	c.SetArgs(argv)
	return c.Execute()
}

func TestConsume_Args_MissingEventIdGivesFriendlyHint(t *testing.T) {
	err := runConsumeArgs(t, []string{})
	expectFriendlyArgsError(t, err, "event consume (no --event-id)")
}

func TestConsume_Args_RejectsPositionalGivesFriendlyHint(t *testing.T) {
	err := runConsumeArgs(t, []string{"meeting.started", "extra.arg"})
	expectFriendlyArgsError(t, err, "event consume (positional args)")
}
