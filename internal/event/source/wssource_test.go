// wssource_test.go — coverage for the WSS event source.
//
// We host a real net/http test server with gorilla's Upgrader to exercise
// the full network path (TLS-less but otherwise identical to production).
// Each test stands up a fresh server because the WSSource is stateful
// across reconnects and we want isolation.

package source

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	eventruntime "tmeet/internal/event"
	"tmeet/internal/event/protocol"
)

// scriptableServer is a tiny WS server that lets each test push frames at
// well-defined moments.
type scriptableServer struct {
	srv          *httptest.Server
	upgrader     websocket.Upgrader
	connect      chan *websocket.Conn
	statusCode   atomic.Int32 // 0 = upgrade normally; non-zero = reject before upgrade.
	connectCount atomic.Int32
}

func newScriptableServer(t *testing.T) *scriptableServer {
	t.Helper()
	s := &scriptableServer{
		connect: make(chan *websocket.Conn, 8),
	}
	s.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if code := s.statusCode.Load(); code != 0 {
			http.Error(w, "rejected", int(code))
			return
		}
		conn, err := s.upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Logf("upgrade failed: %v", err)
			return
		}
		s.connectCount.Add(1)
		select {
		case s.connect <- conn:
		default:
			_ = conn.Close()
		}
	}))
	t.Cleanup(s.srv.Close)
	return s
}

func (s *scriptableServer) wsURL() string {
	return "ws" + strings.TrimPrefix(s.srv.URL, "http")
}

// nextConn waits up to 2 s for a fresh connection to arrive.  Used by
// reconnect tests where we drop one conn and expect the source to dial again.
func (s *scriptableServer) nextConn(t *testing.T) *websocket.Conn {
	t.Helper()
	select {
	case c := <-s.connect:
		return c
	case <-time.After(2 * time.Second):
		t.Fatalf("scriptableServer: no connection within 2s")
		return nil
	}
}

// sendEvent pushes a RawEvent as one text frame.
func (s *scriptableServer) sendEvent(t *testing.T, conn *websocket.Conn, ev eventruntime.RawEvent) {
	t.Helper()
	buf, err := json.Marshal(ev)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := conn.WriteMessage(websocket.TextMessage, buf); err != nil {
		t.Fatalf("server write: %v", err)
	}
}

// runSource starts src.Run on a goroutine and returns helpers to drive it.
type runHandle struct {
	cancel   context.CancelFunc
	emitted  chan *eventruntime.RawEvent
	statusMu sync.Mutex
	statuses []string // appended as "<state>:<detail>"
	runErr   chan error
}

func runSource(t *testing.T, src *WSSource) *runHandle {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	h := &runHandle{
		cancel:  cancel,
		emitted: make(chan *eventruntime.RawEvent, 16),
		runErr:  make(chan error, 1),
	}
	go func() {
		err := src.Run(ctx,
			func(ev *eventruntime.RawEvent) { h.emitted <- ev },
			func(state, detail string) {
				h.statusMu.Lock()
				h.statuses = append(h.statuses, state+":"+detail)
				h.statusMu.Unlock()
			})
		h.runErr <- err
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-h.runErr:
		case <-time.After(3 * time.Second):
			t.Errorf("Run did not return within 3s after cancel")
		}
	})
	return h
}

func (h *runHandle) waitForState(t *testing.T, want string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		h.statusMu.Lock()
		for _, s := range h.statuses {
			if strings.HasPrefix(s, want+":") {
				h.statusMu.Unlock()
				return
			}
		}
		h.statusMu.Unlock()
		time.Sleep(20 * time.Millisecond)
	}
	h.statusMu.Lock()
	all := append([]string(nil), h.statuses...)
	h.statusMu.Unlock()
	t.Fatalf("never observed state %q (saw: %v)", want, all)
}

// ---------- tests ---------------------------------------------------------

func TestWSSource_HappyPathEmitsEvents(t *testing.T) {
	srv := newScriptableServer(t)
	src := &WSSource{
		URL:        srv.wsURL(),
		MinBackoff: 10 * time.Millisecond,
		Decoder:    DecodeNDJSONFrame,
	}
	h := runSource(t, src)

	conn := srv.nextConn(t)
	srv.sendEvent(t, conn, eventruntime.RawEvent{
		Event:   "meeting.started",
		TraceID: "trc_1",
		Payload: json.RawMessage(`{"meeting_id":"m1"}`),
	})

	select {
	case ev := <-h.emitted:
		if ev.TraceID != "trc_1" || ev.Event != "meeting.started" {
			t.Errorf("unexpected event: %+v", ev)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("no event emitted within 2s")
	}

	h.waitForState(t, protocol.SourceStateSteady)
}

func TestWSSource_MultipleEventsInOrder(t *testing.T) {
	srv := newScriptableServer(t)
	src := &WSSource{URL: srv.wsURL(), MinBackoff: 10 * time.Millisecond, Decoder: DecodeNDJSONFrame}
	h := runSource(t, src)

	conn := srv.nextConn(t)
	for i := 0; i < 3; i++ {
		srv.sendEvent(t, conn, eventruntime.RawEvent{
			Event: "meeting.started", TraceID: "trc_" + string(rune('a'+i)),
			Payload: json.RawMessage(`{}`),
		})
	}

	got := []string{}
	for i := 0; i < 3; i++ {
		select {
		case ev := <-h.emitted:
			got = append(got, ev.TraceID)
		case <-time.After(2 * time.Second):
			t.Fatalf("only got %d events", i)
		}
	}
	want := []string{"trc_a", "trc_b", "trc_c"}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("event[%d] = %s, want %s", i, got[i], want[i])
		}
	}
}

func TestWSSource_ReconnectsAfterServerCloseConn(t *testing.T) {
	srv := newScriptableServer(t)
	src := &WSSource{
		URL:        srv.wsURL(),
		MinBackoff: 30 * time.Millisecond,
		MaxBackoff: 100 * time.Millisecond,
		Decoder:    DecodeNDJSONFrame,
	}
	h := runSource(t, src)

	// First session: receive then forcibly close from the server side.
	conn1 := srv.nextConn(t)
	srv.sendEvent(t, conn1, eventruntime.RawEvent{
		Event: "x.y", TraceID: "trc_1", Payload: json.RawMessage(`{}`),
	})
	<-h.emitted
	_ = conn1.Close()

	// Second session: new conn must arrive without manual intervention.
	conn2 := srv.nextConn(t)
	srv.sendEvent(t, conn2, eventruntime.RawEvent{
		Event: "x.y", TraceID: "trc_2", Payload: json.RawMessage(`{}`),
	})
	select {
	case ev := <-h.emitted:
		if ev.TraceID != "trc_2" {
			t.Errorf("post-reconnect event = %s, want trc_2", ev.TraceID)
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("no event after reconnect within 3s")
	}

	if srv.connectCount.Load() < 2 {
		t.Errorf("expected at least 2 connections, got %d", srv.connectCount.Load())
	}
	h.waitForState(t, protocol.SourceStateReconnecting)
}

func TestWSSource_AuthRejectIsFatal(t *testing.T) {
	srv := newScriptableServer(t)
	srv.statusCode.Store(http.StatusUnauthorized)

	src := &WSSource{
		URL:        srv.wsURL(),
		MinBackoff: 10 * time.Millisecond,
		Decoder:    DecodeNDJSONFrame,
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- src.Run(ctx,
			func(*eventruntime.RawEvent) {},
			func(state, detail string) {})
	}()

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("expected non-nil error on 401")
		}
		if !isAuthError(err) {
			t.Errorf("expected authError, got %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("Run did not return on 401 within 3s")
	}
}

func TestWSSource_CtxCancelReturnsCleanly(t *testing.T) {
	srv := newScriptableServer(t)
	src := &WSSource{URL: srv.wsURL(), MinBackoff: 10 * time.Millisecond, Decoder: DecodeNDJSONFrame}

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- src.Run(ctx,
			func(*eventruntime.RawEvent) {},
			func(state, detail string) {})
	}()

	srv.nextConn(t) // wait until connected
	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("ctx cancel must produce nil err, got %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Run did not return within 3s after cancel")
	}
}

func TestWSSource_DecodeErrorSkipsFrameNotKill(t *testing.T) {
	srv := newScriptableServer(t)
	src := &WSSource{URL: srv.wsURL(), MinBackoff: 10 * time.Millisecond, Decoder: DecodeNDJSONFrame}
	h := runSource(t, src)

	conn := srv.nextConn(t)
	// First a malformed text frame, then a valid event.
	if err := conn.WriteMessage(websocket.TextMessage, []byte(`{not json`)); err != nil {
		t.Fatalf("write malformed: %v", err)
	}
	srv.sendEvent(t, conn, eventruntime.RawEvent{
		Event: "x.y", TraceID: "trc_after", Payload: json.RawMessage(`{}`),
	})

	select {
	case ev := <-h.emitted:
		if ev.TraceID != "trc_after" {
			t.Errorf("got %v, expected trc_after", ev.TraceID)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("decode error must not stop the read loop")
	}
}

func TestWSSource_AuthHookSetsHeader(t *testing.T) {
	gotHeader := make(chan string, 1)

	upgrader := websocket.Upgrader{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Capture before upgrade so a later panic doesn't lose the value.
		select {
		case gotHeader <- r.Header.Get("Authorization"):
		default:
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		_ = conn.Close()
	}))
	t.Cleanup(srv.Close)

	src := &WSSource{
		URL:        "ws" + strings.TrimPrefix(srv.URL, "http"),
		MinBackoff: 10 * time.Millisecond,
		Decoder:    DecodeNDJSONFrame,
		AuthHook: func(req *http.Request) error {
			req.Header.Set("Authorization", "Bearer test_token_xyz")
			return nil
		},
	}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		_ = src.Run(ctx,
			func(*eventruntime.RawEvent) {},
			func(string, string) {})
	}()
	defer cancel()

	select {
	case got := <-gotHeader:
		if got != "Bearer test_token_xyz" {
			t.Errorf("Authorization = %q, want Bearer test_token_xyz", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("server never received an upgrade request")
	}
}

func TestWSSource_StaticHeadersAreSent(t *testing.T) {
	gotHeader := make(chan string, 1)
	upgrader := websocket.Upgrader{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case gotHeader <- r.Header.Get("X-Tmeet-Test"):
		default:
		}
		c, _ := upgrader.Upgrade(w, r, nil)
		if c != nil {
			_ = c.Close()
		}
	}))
	t.Cleanup(srv.Close)

	src := &WSSource{
		URL:        "ws" + strings.TrimPrefix(srv.URL, "http"),
		MinBackoff: 10 * time.Millisecond,
		Decoder:    DecodeNDJSONFrame,
		Headers:    http.Header{"X-Tmeet-Test": []string{"hi"}},
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		_ = src.Run(ctx,
			func(*eventruntime.RawEvent) {},
			func(string, string) {})
	}()

	select {
	case got := <-gotHeader:
		if got != "hi" {
			t.Errorf("X-Tmeet-Test = %q, want hi", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("server didn't receive headers within 2s")
	}
}

func TestWSSource_EmptyURLIsImmediatelyFatal(t *testing.T) {
	src := &WSSource{URL: ""}
	err := src.Run(context.Background(), func(*eventruntime.RawEvent) {}, nil)
	if err == nil {
		t.Fatal("expected error on empty URL")
	}
	if !strings.Contains(err.Error(), "empty URL") {
		t.Errorf("error should mention empty URL, got %v", err)
	}
}

func TestWSSource_BackoffGrowsCappedAtMax(t *testing.T) {
	if got := nextBackoff(time.Second, 10*time.Second); got != 2*time.Second {
		t.Errorf("nextBackoff(1s, 10s) = %s, want 2s", got)
	}
	if got := nextBackoff(8*time.Second, 10*time.Second); got != 10*time.Second {
		t.Errorf("nextBackoff(8s, 10s) = %s, want 10s (capped)", got)
	}
	if got := nextBackoff(20*time.Second, 10*time.Second); got != 10*time.Second {
		t.Errorf("nextBackoff cap should never overshoot, got %s", got)
	}
}

func TestDecodeNDJSONFrame_BasicHappy(t *testing.T) {
	ev, err := DecodeNDJSONFrame(websocket.TextMessage,
		[]byte(`{"event":"k","trace_id":"t","payload":{}}`))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if ev == nil || ev.Event != "k" {
		t.Errorf("got %+v", ev)
	}
}

func TestDecodeNDJSONFrame_BinaryFrameSilentlySkipped(t *testing.T) {
	ev, err := DecodeNDJSONFrame(websocket.BinaryMessage, []byte(`{}`))
	if err != nil || ev != nil {
		t.Errorf("binary frame should yield (nil,nil), got (%+v, %v)", ev, err)
	}
}

func TestDecodeNDJSONFrame_MissingEventField(t *testing.T) {
	ev, err := DecodeNDJSONFrame(websocket.TextMessage, []byte(`{"trace_id":"t"}`))
	if ev != nil || err == nil {
		t.Errorf("missing event field should produce error, got (%+v, %v)", ev, err)
	}
}
