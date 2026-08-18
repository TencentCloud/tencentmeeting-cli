// bus_subscribable_test.go — bus-level wiring of Subscribable +
// ReconnectNotifiable sources.
//
// The hub-level subRegistry already has its own unit test
// (subreg_test.go); these tests verify the *integration* in
// internal/event/bus/bus.go's startSources:
//
//   - The bus picks the first source that implements Subscribable
//     and routes hub.SetOnFirstSubscribe through it.
//   - The bus also installs an OnReconnected closure on every source
//     that implements ReconnectNotifiable, so a reconnect Replay
//     re-subscribes the current snapshot.
//
// We intentionally exercise this end-to-end via bus.New + the in-process
// transport rather than poking at hub internals: the contract we care
// about is "a consumer that registers an EventKey causes the source's
// Subscribe to fire", not the locking dance behind it.

package bus_test

import (
	"bytes"
	"context"
	"os"
	"sync"
	"testing"
	"time"

	eventruntime "tmeet/internal/event"
	"tmeet/internal/event/bus"
	"tmeet/internal/event/protocol"
	"tmeet/internal/event/source"
	"tmeet/internal/event/transport"
)

// fakeSubscribable is a Source that records every Subscribe call and
// holds the OnReconnected closure for tests to fire on demand.
type fakeSubscribable struct {
	mu             sync.Mutex
	subscribeCalls []string

	onReconnected     func()
	onSubscribeResult func(eventKeys []string, code uint32, msg string)

	// runDone closes when Run unblocks; used by tests that want to
	// assert the source goroutine exits cleanly.
	runDone chan struct{}
}

func newFakeSubscribable() *fakeSubscribable {
	return &fakeSubscribable{runDone: make(chan struct{})}
}

func (f *fakeSubscribable) Name() string { return "fake-sub" }

func (f *fakeSubscribable) Run(ctx context.Context, _ func(*eventruntime.RawEvent),
	_ source.StatusNotifier) error {

	defer close(f.runDone)
	<-ctx.Done()
	return nil
}

// Subscribe implements source.Subscribable.
func (f *fakeSubscribable) Subscribe(ctx context.Context, eventKey, agentOpenID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.subscribeCalls = append(f.subscribeCalls, eventKey)
	return nil
}

// SetOnReconnected implements source.ReconnectNotifiable.
func (f *fakeSubscribable) SetOnReconnected(fn func()) {
	f.onReconnected = fn
}

// SetOnSubscribeResult implements source.SubscribeResultNotifiable so
// the bus's startSources installs the subscribe-rsp watcher callback
// here.  Tests fire it manually to simulate a gateway response.
func (f *fakeSubscribable) SetOnSubscribeResult(fn func(eventKeys []string, code uint32, msg string)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.onSubscribeResult = fn
}

func (f *fakeSubscribable) onSubResult() func(eventKeys []string, code uint32, msg string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.onSubscribeResult
}

func (f *fakeSubscribable) calls() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, len(f.subscribeCalls))
	copy(out, f.subscribeCalls)
	return out
}

// fakeNonSubscribable is the negative case: a Source that does NOT
// implement Subscribable, ensuring the bus's type-assert tolerates the
// missing interface (i.e. nothing crashes when only MockSource-like
// sources are configured).
type fakeNonSubscribable struct {
	runDone chan struct{}
}

func newFakeNonSubscribable() *fakeNonSubscribable {
	return &fakeNonSubscribable{runDone: make(chan struct{})}
}

func (f *fakeNonSubscribable) Name() string { return "fake-nonsub" }
func (f *fakeNonSubscribable) Run(ctx context.Context, _ func(*eventruntime.RawEvent),
	_ source.StatusNotifier) error {

	defer close(f.runDone)
	<-ctx.Done()
	return nil
}

// fakeSubscriber implements bus.Subscriber for tests; we register it
// directly on the hub via b.HubForTest (or, since hub is unexported,
// reuse the existing in-process IPC path through Hello).  The simpler
// option here is to bypass hub.Register with a synthetic subscriber via
// the public-but-test-only HubForTest accessor — but the bus's hub
// field isn't exported.  We therefore drive the wiring through the IPC
// Hello path which is what the production consumer uses anyway.
//
// To avoid wiring up a full handler harness for one test, we use a
// hub-only test that exercises SetOnFirstSubscribe by faking a
// subscriber.  See subreg_test.go for the unit-level hub coverage; this
// integration test relies on the hub callback we know fires from the
// unit tests, plus the bus's own startSources wire-up.

func TestBus_OnFirstSubscribe_RoutesToSourceSubscribe(t *testing.T) {
	// Strategy: stand up a bus with a fakeSubscribable source, then
	// connect a real IPC consumer that sends Hello with EventKey="k1".
	// The bus's hub.Register will fire onFirstSubscribe, which the
	// startSources wiring routes to our fake's Subscribe.
	dir, err := os.MkdirTemp("", "tmeet-bus-*")
	if err != nil {
		t.Fatalf("mkdtemp: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	t.Setenv("TMEET_CLI_CONFIG_DIR", dir)
	if err := os.MkdirAll(dir+"/event", 0700); err != nil {
		t.Fatal(err)
	}

	fake := newFakeSubscribable()
	b := bus.New(bus.Config{
		OpenIDHash:  eventruntime.OpenIDHash("user-sub-test"),
		BusVersion:  "test",
		Source:      []source.Source{fake},
		IdleTimeout: 30 * time.Second,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() { _ = b.Run(ctx); close(done) }()

	// Wait for the bus to start accepting.
	tr := transport.New()
	if !waitBusReady(tr) {
		t.Fatal("bus did not become ready")
	}

	// Connect + Hello with a real EventKey.
	hashOK := eventruntime.OpenIDHash("user-sub-test")
	openHelloOK(t, "meeting.started", hashOK)

	// Allow the OnFirstSubscribe callback to land.
	if !waitForCondition(2*time.Second, func() bool {
		return containsCall(fake.calls(), "meeting.started")
	}) {
		t.Errorf("fake.Subscribe never called for meeting.started; got %v", fake.calls())
	}

	cancel()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("bus did not exit within 3s")
	}
}

func TestBus_OnReconnected_TriggersReplayOfCurrentSnapshot(t *testing.T) {
	// Strategy: subscribe two consumers (k1, k2), then fire the
	// fake's OnReconnected to simulate a WSS reconnect.  We expect
	// the bus to call Subscribe (or SubscribeBatch — fake has no
	// batch method, so per-key) for both keys.
	dir, err := os.MkdirTemp("", "tmeet-bus-*")
	if err != nil {
		t.Fatalf("mkdtemp: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	t.Setenv("TMEET_CLI_CONFIG_DIR", dir)
	if err := os.MkdirAll(dir+"/event", 0700); err != nil {
		t.Fatal(err)
	}

	fake := newFakeSubscribable()
	b := bus.New(bus.Config{
		OpenIDHash:  eventruntime.OpenIDHash("user-replay-test"),
		BusVersion:  "test",
		Source:      []source.Source{fake},
		IdleTimeout: 30 * time.Second,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() { _ = b.Run(ctx); close(done) }()

	tr := transport.New()
	if !waitBusReady(tr) {
		t.Fatal("bus did not become ready")
	}
	hash := eventruntime.OpenIDHash("user-replay-test")

	openHelloOK(t, "meeting.started", hash)
	openHelloOK(t, "meeting.end", hash)

	// Wait until both initial Subscribe calls landed (these are the
	// 0→1 transitions for both keys).
	if !waitForCondition(2*time.Second, func() bool {
		c := fake.calls()
		return containsCall(c, "meeting.started") && containsCall(c, "meeting.end")
	}) {
		t.Fatalf("initial subscribes missing; got %v", fake.calls())
	}

	// Reset the call log and fire OnReconnected to simulate Replay.
	fake.mu.Lock()
	fake.subscribeCalls = nil
	cb := fake.onReconnected
	fake.mu.Unlock()
	if cb == nil {
		t.Fatal("bus did not install OnReconnected callback on the source")
	}
	cb()

	if !waitForCondition(2*time.Second, func() bool {
		c := fake.calls()
		return containsCall(c, "meeting.started") && containsCall(c, "meeting.end")
	}) {
		t.Errorf("Replay did not re-subscribe both keys; got %v", fake.calls())
	}

	cancel()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("bus did not exit within 3s")
	}
}

func TestBus_NonSubscribableSource_NoCrash(t *testing.T) {
	// A Source that doesn't implement Subscribable / ReconnectNotifiable
	// must not crash startSources.  Equivalent to the MockSource path in
	// production.
	dir, err := os.MkdirTemp("", "tmeet-bus-*")
	if err != nil {
		t.Fatalf("mkdtemp: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	t.Setenv("TMEET_CLI_CONFIG_DIR", dir)
	if err := os.MkdirAll(dir+"/event", 0700); err != nil {
		t.Fatal(err)
	}

	src := newFakeNonSubscribable()
	b := bus.New(bus.Config{
		OpenIDHash:  eventruntime.OpenIDHash("user-nonsub-test"),
		BusVersion:  "test",
		Source:      []source.Source{src},
		IdleTimeout: 30 * time.Second,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() { _ = b.Run(ctx); close(done) }()

	tr := transport.New()
	if !waitBusReady(tr) {
		t.Fatal("bus did not become ready")
	}

	hash := eventruntime.OpenIDHash("user-nonsub-test")
	openHelloOK(t, "meeting.started", hash)
	// No assertions on the source — the win condition is simply that
	// the bus didn't panic when fan-out reached a non-Subscribable.

	cancel()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("bus did not exit within 3s")
	}
}

// --- helpers shared with other bus_test files ---------------------------

// containsCall reports whether key appears in calls.
func containsCall(calls []string, key string) bool {
	for _, c := range calls {
		if c == key {
			return true
		}
	}
	return false
}

// waitForCondition polls fn at 20ms cadence until it returns true or
// the deadline lapses.  Used to synchronise on async hub callbacks
// without sprinkling time.Sleep into every test body.
func waitForCondition(timeout time.Duration, fn func() bool) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if fn() {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return false
}

// waitBusReady polls busctl.Ping until it succeeds or 2s elapses.
func waitBusReady(tr transport.IPC) bool {
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		c, err := tr.Dial(eventruntime.BusSockPath())
		if err == nil {
			_ = c.Close()
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return false
}

// openHelloOK reuses the existing helloAndExpectAck helper from
// bus_e2e_test.go.  Returns the read side so the caller can keep the
// conn alive (closing it would Unregister and the 0→1 we asserted
// would already have fired anyway, but matching real consumer
// behaviour keeps the test less surprising).
func openHelloOK(t *testing.T, eventKey, ownerHash string) {
	t.Helper()
	conn, ack := helloAndExpectAck(t, ownerHash, eventKey)
	if ack.Error != "" {
		_ = conn.c.Close()
		t.Fatalf("HelloAck.Error = %q (eventKey=%q hash=%q)", ack.Error, eventKey, ownerHash)
	}
	t.Cleanup(func() { _ = conn.c.Close() })
}

// (no longer needed; we use withTempBusDir from bus_e2e_test.go directly)
func mkSubdirOnce(cfgDir string) error { return nil }

// TestBus_OnSubscribeResult_Failure_RoutesToConsumer pins the end-to-end
// path that motivated the subscribe_error control frame: when the
// upstream WsCLISubscribeEvent rsp comes back with code != 0, the bus
// MUST fan out a control+kind=subscribe_error frame to every consumer
// subscribed to that EventKey.  Consumers subscribed to an unrelated
// key MUST NOT receive it.
func TestBus_OnSubscribeResult_Failure_RoutesToConsumer(t *testing.T) {
	dir, err := os.MkdirTemp("", "tmeet-bus-*")
	if err != nil {
		t.Fatalf("mkdtemp: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	t.Setenv("TMEET_CLI_CONFIG_DIR", dir)
	if err := os.MkdirAll(dir+"/event", 0700); err != nil {
		t.Fatal(err)
	}

	fake := newFakeSubscribable()
	hash := eventruntime.OpenIDHash("user-suberr-test")
	b := bus.New(bus.Config{
		OpenIDHash:  hash,
		BusVersion:  "test",
		Source:      []source.Source{fake},
		IdleTimeout: 30 * time.Second,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() { _ = b.Run(ctx); close(done) }()
	defer func() {
		cancel()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Errorf("bus did not exit within 3s")
		}
	}()

	tr := transport.New()
	if !waitBusReady(tr) {
		t.Fatal("bus did not become ready")
	}

	// Two consumers: one for the failing key, one for an unrelated key.
	crFail, ackFail := helloAndExpectAck(t, hash, "meeting.started")
	defer crFail.Close()
	if ackFail.Error != "" {
		t.Fatalf("hello (failing key) error: %q", ackFail.Error)
	}

	crOther, ackOther := helloAndExpectAck(t, hash, "meeting.end")
	defer crOther.Close()
	if ackOther.Error != "" {
		t.Fatalf("hello (other key) error: %q", ackOther.Error)
	}

	// Wait for the bus to install the OnSubscribeResult callback (it
	// happens once at startSources start; no race in practice but be
	// safe).
	var cb func(eventKeys []string, code uint32, msg string)
	if !waitForCondition(2*time.Second, func() bool {
		cb = fake.onSubResult()
		return cb != nil
	}) {
		t.Fatal("bus did not install OnSubscribeResult callback on the source")
	}

	// Simulate a gateway rejection of the SubscribeReq for meeting.started.
	cb([]string{"meeting.started"}, 42, "permission denied")

	// crFail (subscribed to meeting.started) MUST receive a
	// subscribe_error control frame within a reasonable window.
	_ = crFail.c.SetReadDeadline(time.Now().Add(2 * time.Second))
	line, rerr := protocol.ReadFrame(crFail.br)
	if rerr != nil {
		t.Fatalf("read subscribe_error frame: %v", rerr)
	}
	msg, derr := protocol.Decode(bytes.TrimRight(line, "\n"))
	if derr != nil {
		t.Fatalf("decode frame: %v", derr)
	}
	ctrl, ok := msg.(*protocol.Control)
	if !ok {
		t.Fatalf("expected *Control, got %T", msg)
	}
	if ctrl.Kind != protocol.ControlKindSubscribeError {
		t.Errorf("Kind = %q, want %q", ctrl.Kind, protocol.ControlKindSubscribeError)
	}
	if ctrl.EventKey != "meeting.started" {
		t.Errorf("EventKey = %q, want meeting.started", ctrl.EventKey)
	}
	if ctrl.Code != 42 {
		t.Errorf("Code = %d, want 42", ctrl.Code)
	}
	if ctrl.Detail != "permission denied" {
		t.Errorf("Detail = %q, want %q", ctrl.Detail, "permission denied")
	}

	// crOther (subscribed to meeting.end) MUST NOT receive the frame.
	// We probe with a short read deadline; a timeout is the success
	// condition.
	_ = crOther.c.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	if _, rerr := protocol.ReadFrame(crOther.br); rerr == nil {
		t.Errorf("unrelated consumer (meeting.end) unexpectedly received a frame")
	}
}
