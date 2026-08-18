// bus_e2e_test.go — single-process end-to-end exercise for the bus daemon.
//
// We bring up a real Bus inside the test process (no fork), connect a
// MockSubscriber via the actual transport, run StatusQuery + Hello +
// Publish-then-receive, and verify graceful shutdown teardown.  Goal:
// catch interface mismatches and life-cycle bugs that unit tests over
// individual files would miss.

package bus_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"tmeet/internal/core/filelock"
	eventruntime "tmeet/internal/event"
	"tmeet/internal/event/bus"
	"tmeet/internal/event/busctl"
	"tmeet/internal/event/protocol"
	"tmeet/internal/event/source"
	"tmeet/internal/event/transport"
)

// withTempBusDir redirects BusDir() to a short-path temp dir and ensures
// event/ exists.
//
// macOS unix-socket paths are capped at 104 bytes (109 with NUL), and
// t.TempDir() resolves to /var/folders/<long-hash>/T/<TestName>/<seq> which
// blows past that limit when the test name is long.  We sidestep by mkdtemp
// under os.TempDir() (== /tmp on macOS) and registering Cleanup ourselves.
func withTempBusDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "tmeet-bus-*")
	if err != nil {
		t.Fatalf("mkdtemp: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	t.Setenv("TMEET_CLI_CONFIG_DIR", dir)
	if err := os.MkdirAll(dir+"/event", 0700); err != nil {
		t.Fatalf("mkdir event: %v", err)
	}
	return dir
}

// noopSource is a Source that just blocks until ctx cancels — used when we
// don't want synthetic event traffic interfering with the test.
type noopSource struct{}

func (noopSource) Name() string { return "noop" }
func (noopSource) Run(ctx context.Context, emit func(*eventruntime.RawEvent), notify source.StatusNotifier) error {
	if notify != nil {
		notify(protocol.SourceStateSteady, "noop")
	}
	<-ctx.Done()
	return nil
}

// scriptedSource emits exactly the events the test wants.  Calling emit from
// the test goroutine after Run starts isn't safe (we don't have its ctx
// yet), so we expose a `Trigger` channel.
type scriptedSource struct {
	trigger chan *eventruntime.RawEvent
}

func newScriptedSource() *scriptedSource {
	return &scriptedSource{trigger: make(chan *eventruntime.RawEvent, 16)}
}

func (s *scriptedSource) Name() string { return "scripted" }
func (s *scriptedSource) Run(ctx context.Context, emit func(*eventruntime.RawEvent), notify source.StatusNotifier) error {
	if notify != nil {
		notify(protocol.SourceStateSteady, "ready")
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

// startBus spins a Bus on a temp dir with the given source and a quick idle
// timeout.  Returns a cleanup func that signals shutdown and waits.
func startBus(t *testing.T, src source.Source, ownerHash string, idle time.Duration) (*bus.Bus, func()) {
	t.Helper()
	withTempBusDir(t)
	b := bus.New(bus.Config{
		OpenIDHash:  ownerHash,
		BusVersion:  "test",
		Source:      []source.Source{src},
		IdleTimeout: idle,
		Logger:      nil, // discard
	})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = b.Run(ctx)
	}()
	// Wait until the listener is up — try Dial up to 2 s.
	tr := transport.New()
	deadline := time.Now().Add(2 * time.Second)
	for {
		if err := busctl.Ping(tr); err == nil {
			break
		}
		if time.Now().After(deadline) {
			cancel()
			<-done
			t.Fatalf("bus did not start within 2s")
		}
		time.Sleep(20 * time.Millisecond)
	}
	cleanup := func() {
		cancel()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Errorf("bus did not exit within 3s of cancel")
		}
	}
	return b, cleanup
}

// helloAndExpectAck Dials the bus, sends Hello, reads HelloAck.
// Returns the conn so the caller can read more frames; conn must be closed.
func helloAndExpectAck(t *testing.T, ownerHash, eventKey string) (conn closeReader, ack *protocol.HelloAck) {
	t.Helper()
	tr := transport.New()
	netConn, err := tr.Dial(eventruntime.BusSockPath())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	hello := protocol.NewHello(os.Getpid(), []string{eventKey}, nil, ownerHash, "test", "trace-1", "")
	if err := protocol.EncodeWithDeadline(netConn, hello, protocol.WriteTimeout); err != nil {
		_ = netConn.Close()
		t.Fatalf("write hello: %v", err)
	}
	br := bufio.NewReader(netConn)
	if err := netConn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		_ = netConn.Close()
		t.Fatalf("set read deadline: %v", err)
	}
	line, err := protocol.ReadFrame(br)
	if err != nil {
		_ = netConn.Close()
		t.Fatalf("read ack: %v", err)
	}
	msg, err := protocol.Decode(bytes.TrimRight(line, "\n"))
	if err != nil {
		_ = netConn.Close()
		t.Fatalf("decode ack: %v", err)
	}
	a, ok := msg.(*protocol.HelloAck)
	if !ok {
		_ = netConn.Close()
		t.Fatalf("expected HelloAck got %T", msg)
	}
	_ = netConn.SetReadDeadline(time.Time{})
	return closeReader{netConn, br}, a
}

type closeReader struct {
	c  closeableConn
	br *bufio.Reader
}

type closeableConn interface {
	Close() error
	SetReadDeadline(time.Time) error
}

func (cr closeReader) Close() error { return cr.c.Close() }

// ------------------------- Tests -------------------------

func TestBus_StatusQuery_Empty(t *testing.T) {
	owner := "owner-aaa"
	_, stop := startBus(t, noopSource{}, owner, 30*time.Second)
	defer stop()

	resp, err := busctl.QueryStatus(transport.New())
	if err != nil {
		t.Fatalf("QueryStatus: %v", err)
	}
	if resp.OwnerHash != owner {
		t.Errorf("OwnerHash mismatch: want %q got %q", owner, resp.OwnerHash)
	}
	if resp.ActiveConns != 0 {
		t.Errorf("ActiveConns: want 0 got %d", resp.ActiveConns)
	}
	if resp.PID != os.Getpid() {
		t.Errorf("PID: want %d got %d", os.Getpid(), resp.PID)
	}
}

func TestBus_Hello_WrongOwner(t *testing.T) {
	_, stop := startBus(t, noopSource{}, "owner-correct", 30*time.Second)
	defer stop()

	cr, ack := helloAndExpectAck(t, "owner-WRONG", "meeting.started")
	defer cr.Close()
	if ack.Error != protocol.HelloErrWrongOwner {
		t.Errorf("expected error %q got %q", protocol.HelloErrWrongOwner, ack.Error)
	}
	if ack.ExpectedOwnerHash != "owner-correct" {
		t.Errorf("ExpectedOwnerHash mismatch: want owner-correct got %q", ack.ExpectedOwnerHash)
	}
}

func TestBus_Hello_UnknownEventKey(t *testing.T) {
	owner := "owner-bbb"
	_, stop := startBus(t, noopSource{}, owner, 30*time.Second)
	defer stop()

	cr, ack := helloAndExpectAck(t, owner, "this.does.not.exist")
	defer cr.Close()
	if ack.Error != protocol.HelloErrUnknownKey {
		t.Errorf("expected error %q got %q", protocol.HelloErrUnknownKey, ack.Error)
	}
}

// helloWithKeysAndExpectAck is the multi-key counterpart to
// helloAndExpectAck: it lets tests craft a Hello with an arbitrary
// EventKeys slice (including empty / >1 elements) so we can exercise
// the InvalidParams rejection path in handleHello.
func helloWithKeysAndExpectAck(t *testing.T, ownerHash string, keys []string) (conn closeReader, ack *protocol.HelloAck) {
	t.Helper()
	tr := transport.New()
	netConn, err := tr.Dial(eventruntime.BusSockPath())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	hello := protocol.NewHello(os.Getpid(), keys, nil, ownerHash, "test", "trace-multikey", "")
	if err := protocol.EncodeWithDeadline(netConn, hello, protocol.WriteTimeout); err != nil {
		_ = netConn.Close()
		t.Fatalf("write hello: %v", err)
	}
	br := bufio.NewReader(netConn)
	if err := netConn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		_ = netConn.Close()
		t.Fatalf("set read deadline: %v", err)
	}
	line, err := protocol.ReadFrame(br)
	if err != nil {
		_ = netConn.Close()
		t.Fatalf("read ack: %v", err)
	}
	msg, err := protocol.Decode(bytes.TrimRight(line, "\n"))
	if err != nil {
		_ = netConn.Close()
		t.Fatalf("decode ack: %v", err)
	}
	a, ok := msg.(*protocol.HelloAck)
	if !ok {
		_ = netConn.Close()
		t.Fatalf("expected HelloAck got %T", msg)
	}
	_ = netConn.SetReadDeadline(time.Time{})
	return closeReader{netConn, br}, a
}

// TestBus_Hello_ZeroEventKeysRejected exercises the "no keys" branch of
// the InvalidParams guard: the bus is single-key only in this release,
// so an empty EventKeys slice must be rejected before any downstream
// state (Conn, Hub) is touched.
func TestBus_Hello_ZeroEventKeysRejected(t *testing.T) {
	owner := "owner-invalid-zero"
	_, stop := startBus(t, noopSource{}, owner, 30*time.Second)
	defer stop()

	cr, ack := helloWithKeysAndExpectAck(t, owner, nil)
	defer cr.Close()
	if ack.Error != protocol.HelloErrInvalidParams {
		t.Errorf("expected error %q got %q", protocol.HelloErrInvalidParams, ack.Error)
	}
	if ack.Detail == "" {
		t.Errorf("expected non-empty detail on InvalidParams ack, got empty")
	}
}

// TestBus_Hello_MultipleEventKeysRejected exercises the ">1 keys" branch
// of the InvalidParams guard.  Kept separate from the zero-keys case so
// a future regression that only breaks one branch surfaces on the
// exact test that covers it.
func TestBus_Hello_MultipleEventKeysRejected(t *testing.T) {
	owner := "owner-invalid-multi"
	_, stop := startBus(t, noopSource{}, owner, 30*time.Second)
	defer stop()

	cr, ack := helloWithKeysAndExpectAck(t, owner, []string{"meeting.started", "meeting.end"})
	defer cr.Close()
	if ack.Error != protocol.HelloErrInvalidParams {
		t.Errorf("expected error %q got %q", protocol.HelloErrInvalidParams, ack.Error)
	}
	if ack.Detail == "" {
		t.Errorf("expected non-empty detail on InvalidParams ack, got empty")
	}
	// InvalidParams is a client-side / version mismatch; the bus must
	// not leak the expected owner hash on this branch (contrast with
	// WrongOwner where ExpectedOwnerHash is populated).
	if ack.ExpectedOwnerHash != "" {
		t.Errorf("ExpectedOwnerHash should be empty for InvalidParams, got %q", ack.ExpectedOwnerHash)
	}
}

func TestBus_PublishToSubscriber(t *testing.T) {
	owner := "owner-ccc"
	src := newScriptedSource()
	_, stop := startBus(t, src, owner, 30*time.Second)
	defer stop()

	cr, ack := helloAndExpectAck(t, owner, "meeting.started")
	defer cr.Close()
	if ack.Error != "" {
		t.Fatalf("hello failed: %+v", ack)
	}

	// Trigger one event.
	payload, _ := json.Marshal(map[string]string{"hello": "world"})
	src.trigger <- &eventruntime.RawEvent{
		Event:   "meeting.started",
		TraceID: "test-trace-1",
		Payload: payload,
	}

	// Read the next frame from the subscriber side.
	if err := cr.c.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("SetReadDeadline: %v", err)
	}
	line, err := protocol.ReadFrame(cr.br)
	if err != nil {
		t.Fatalf("read event: %v", err)
	}
	msg, err := protocol.Decode(bytes.TrimRight(line, "\n"))
	if err != nil {
		t.Fatalf("decode event: %v", err)
	}
	ev, ok := msg.(*protocol.Event)
	if !ok {
		t.Fatalf("expected Event got %T", msg)
	}
	if ev.Event != "meeting.started" || ev.TraceID != "test-trace-1" {
		t.Errorf("event mismatch: got %+v", ev)
	}

	// Status now reports 1 active conn.
	resp, err := busctl.QueryStatus(transport.New())
	if err != nil {
		t.Fatalf("QueryStatus: %v", err)
	}
	if resp.ActiveConns != 1 {
		t.Errorf("ActiveConns: want 1 got %d", resp.ActiveConns)
	}
}

func TestBus_ShutdownFrame_TerminatesDaemon(t *testing.T) {
	withTempBusDir(t)
	b := bus.New(bus.Config{
		OpenIDHash:  "owner-ddd",
		BusVersion:  "test",
		Source:      []source.Source{noopSource{}},
		IdleTimeout: 30 * time.Second,
		Logger:      nil,
	})
	ctx := context.Background()
	done := make(chan error, 1)
	go func() { done <- b.Run(ctx) }()

	// Wait until listening.
	tr := transport.New()
	for i := 0; i < 100; i++ {
		if busctl.Ping(tr) == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	if err := busctl.SendShutdown(tr, false); err != nil {
		t.Fatalf("SendShutdown: %v", err)
	}

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Run returned error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("bus did not exit after SendShutdown within 3s")
	}

	// Post-exit Ping should fail with ErrNotRunning (Dial fails).
	if err := busctl.Ping(tr); err == nil {
		t.Errorf("Ping should fail after shutdown, got nil")
	} else if !errors.Is(err, busctl.ErrNotRunning) {
		t.Errorf("expected ErrNotRunning got %v", err)
	}
}

func TestBus_ForkLoser_ExitsCleanly(t *testing.T) {
	withTempBusDir(t)
	first := bus.New(bus.Config{
		OpenIDHash:  "owner-eee",
		BusVersion:  "test",
		Source:      []source.Source{noopSource{}},
		IdleTimeout: 30 * time.Second,
		Logger:      nil,
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- first.Run(ctx) }()

	// Wait until first is up.
	tr := transport.New()
	for i := 0; i < 100; i++ {
		if busctl.Ping(tr) == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	// Second bus should immediately return nil because alive lock is held.
	second := bus.New(bus.Config{
		OpenIDHash:  "owner-eee",
		BusVersion:  "test",
		Source:      []source.Source{noopSource{}},
		IdleTimeout: 30 * time.Second,
		Logger:      nil,
	})
	secondErr := make(chan error, 1)
	go func() { secondErr <- second.Run(context.Background()) }()
	select {
	case err := <-secondErr:
		if err != nil {
			t.Errorf("loser bus returned error %v, want nil", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("loser bus did not exit within 2s")
	}

	// Cleanup first.
	cancel()
	<-done
}

func TestBus_AliveLockReleasedOnExit(t *testing.T) {
	withTempBusDir(t)
	b := bus.New(bus.Config{
		OpenIDHash:  "owner-fff",
		BusVersion:  "test",
		Source:      []source.Source{noopSource{}},
		IdleTimeout: 30 * time.Second,
		Logger:      nil,
	})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- b.Run(ctx) }()

	// Wait until ready.
	tr := transport.New()
	for i := 0; i < 100; i++ {
		if busctl.Ping(tr) == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	cancel()
	<-done

	// After exit, alive lock must be re-acquirable.
	probe := filelock.NewProcessLock(eventruntime.BusAliveLock())
	if err := probe.TryLock(); err != nil {
		t.Fatalf("alive lock not released after bus exit: %v", err)
	}
	_ = probe.Unlock()
}

// keep imports we may extend tests with in the future
var _ = sync.Mutex{}
var _ = fmt.Sprintf
