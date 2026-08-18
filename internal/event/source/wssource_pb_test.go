// wssource_pb_test.go — coverage for the production pb path.
//
// These tests stand up a WS server that speaks the same protobuf
// envelope (ConnMsg) as the Tencent Meeting gateway, drives an
// AuthBindReq → AuthBindRsp handshake, sends push frames, and asserts
// that:
//
//  1. The first upstream frame is AuthBindReq with the configured
//     Token + DeviceID, and that the source does NOT proceed to the
//     read loop until the matching rsp arrives.
//  2. Non-zero AuthBindRsp.status surfaces as a fatal authError (Run
//     returns; bus daemon would exit).
//  3. Subscribe(ctx, key) writes a SubscribeReq with the right
//     event_list and cmd type, both for single-key and Replay paths.
//  4. SetOnReconnected fires after each successful AuthBind, so the
//     bus has a place to plug Replay.
//  5. Inbound push frames (cmd_type=2) decode into RawEvents with
//     matching Event/TraceID and trigger an outbound ack (cmd_type=3)
//     within a small window.

package source

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"

	eventruntime "tmeet/internal/event"
	"tmeet/internal/event/protocol/wsspb"
)

// pbServer hosts a ConnMsg-speaking endpoint.  Each test installs an
// ack handler that receives every binary frame the source writes.
type pbServer struct {
	srv      *httptest.Server
	upgrader websocket.Upgrader
	conns    chan *websocket.Conn
}

func newPbServer(t *testing.T) *pbServer {
	t.Helper()
	s := &pbServer{conns: make(chan *websocket.Conn, 4)}
	s.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := s.upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Logf("upgrade: %v", err)
			return
		}
		select {
		case s.conns <- conn:
		default:
			_ = conn.Close()
		}
	}))
	t.Cleanup(s.srv.Close)
	return s
}

func (s *pbServer) wsURL() string {
	return "ws" + strings.TrimPrefix(s.srv.URL, "http")
}

func (s *pbServer) accept(t *testing.T) *websocket.Conn {
	t.Helper()
	select {
	case c := <-s.conns:
		return c
	case <-time.After(2 * time.Second):
		t.Fatal("no connection within 2s")
		return nil
	}
}

// readFrame reads one binary ConnMsg from conn.  Fails the test on
// timeout / non-binary frame.
func readFrame(t *testing.T, conn *websocket.Conn) *wsspb.ConnMsg {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	mtype, data, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if mtype != websocket.BinaryMessage {
		t.Fatalf("expected binary frame, got type=%d", mtype)
	}
	cm, err := wsspb.Decode(data)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	return cm
}

// writeAuthBindRsp replies to an AuthBindReq carrying the supplied
// ret_code (0 = success).  Used by tests that don't care about the
// success path details and just want to advance the source past the
// handshake.
//
// The proto contract puts the success/failure signal in
// AuthBindRsp.ret_code (the body), not Head.status — so we marshal a
// real AuthBindRsp into ConnMsg.Data instead of just stuffing the
// envelope.
func writeAuthBindRsp(t *testing.T, conn *websocket.Conn, reqMsgID string, retCode int32) {
	t.Helper()
	inner := &wsspb.AuthBindRsp{
		RetCode: retCode,
	}
	if retCode != 0 {
		inner.Msg = "unit-test rejection"
	}
	innerWire, err := proto.Marshal(inner)
	if err != nil {
		t.Fatalf("marshal AuthBindRsp: %v", err)
	}
	rsp := &wsspb.ConnMsg{
		Head: &wsspb.Head{
			FrameType: wsspb.FrameTypeDefault,
			CmdType:   wsspb.CmdTypeUpstreamRsp,
			Cmd:       wsspb.CmdAuthBind,
			MsgId:     reqMsgID,
		},
		Data: innerWire,
	}
	wire, err := proto.Marshal(rsp)
	if err != nil {
		t.Fatalf("marshal rsp: %v", err)
	}
	if err := conn.WriteMessage(websocket.BinaryMessage, wire); err != nil {
		t.Fatalf("write rsp: %v", err)
	}
}

// writeHeartRsp replies to a /conn/ping HeartReq with the supplied
// heart_interval (in seconds).  Used to advance the source past the
// post-AuthBind first heartbeat that would otherwise stall the read
// loop / Subscribe path tests.
func writeHeartRsp(t *testing.T, conn *websocket.Conn, reqMsgID string, heartInterval int32) {
	t.Helper()
	inner := &wsspb.HeartRsp{HeartInterval: heartInterval}
	innerWire, err := proto.Marshal(inner)
	if err != nil {
		t.Fatalf("marshal HeartRsp: %v", err)
	}
	rsp := &wsspb.ConnMsg{
		Head: &wsspb.Head{
			FrameType: wsspb.FrameTypeDefault,
			CmdType:   wsspb.CmdTypeUpstreamRsp,
			Cmd:       wsspb.CmdPing,
			MsgId:     reqMsgID,
		},
		Data: innerWire,
	}
	wire, err := proto.Marshal(rsp)
	if err != nil {
		t.Fatalf("marshal heart rsp: %v", err)
	}
	if err := conn.WriteMessage(websocket.BinaryMessage, wire); err != nil {
		t.Fatalf("write heart rsp: %v", err)
	}
}

// drainHeartReq reads the /conn/ping frame the source emits right after
// AuthBind succeeds and replies with a HeartRsp carrying a long
// interval (so a second heartbeat doesn't fire mid-test).
func drainHeartReq(t *testing.T, conn *websocket.Conn) {
	t.Helper()
	cm := readFrame(t, conn)
	if cm.Head.Cmd != wsspb.CmdPing || cm.Head.CmdType != wsspb.CmdTypeUpstreamReq {
		t.Fatalf("expected /conn/ping req, got cmd=%q cmd_type=%d",
			cm.Head.Cmd, cm.Head.CmdType)
	}
	writeHeartRsp(t, conn, cm.Head.MsgId, 3600)
}

// writePush sends a cmd_type=2 WsCLIPushEvent push frame.  Per the
// active wire contract, Head.Cmd is fixed to CmdWsCLIPushEvent and
// Data is the JSON encoding of a RawEvent ({"event", "trace_id",
// "payload"}); the inner "event" field carries the business EventKey
// the bus's hub routes by.  Returns the msg_id so the test can assert
// the subsequent ack matches.
func writePush(t *testing.T, conn *websocket.Conn, eventKey, traceID string, payload []byte) string {
	t.Helper()
	if len(payload) == 0 {
		payload = []byte("null")
	}
	rawEvent := struct {
		Event   string          `json:"event"`
		TraceID string          `json:"trace_id"`
		Payload json.RawMessage `json:"payload"`
	}{Event: eventKey, TraceID: traceID, Payload: payload}
	data, err := json.Marshal(rawEvent)
	if err != nil {
		t.Fatalf("marshal RawEvent: %v", err)
	}
	msgID := "push-" + traceID
	frame := &wsspb.ConnMsg{
		Head: &wsspb.Head{
			FrameType: wsspb.FrameTypeDefault,
			CmdType:   wsspb.CmdTypeDownstreamPush,
			Cmd:       wsspb.CmdWsCLIPushEvent,
			MsgId:     msgID,
		},
		Data: data,
	}
	wire, err := proto.Marshal(frame)
	if err != nil {
		t.Fatalf("marshal push: %v", err)
	}
	if err := conn.WriteMessage(websocket.BinaryMessage, wire); err != nil {
		t.Fatalf("write push: %v", err)
	}
	return msgID
}

// writeRawDownstream sends a cmd_type=2 frame with a caller-supplied
// Head.Cmd.  Used to exercise the source's filter for non-business
// downstream pushes (anything other than CmdWsCLIPushEvent).
func writeRawDownstream(t *testing.T, conn *websocket.Conn, cmd, msgID string, data []byte) {
	t.Helper()
	frame := &wsspb.ConnMsg{
		Head: &wsspb.Head{
			FrameType: wsspb.FrameTypeDefault,
			CmdType:   wsspb.CmdTypeDownstreamPush,
			Cmd:       cmd,
			MsgId:     msgID,
		},
		Data: data,
	}
	wire, err := proto.Marshal(frame)
	if err != nil {
		t.Fatalf("marshal raw downstream: %v", err)
	}
	if err := conn.WriteMessage(websocket.BinaryMessage, wire); err != nil {
		t.Fatalf("write raw downstream: %v", err)
	}
}

func TestWSSource_PB_AuthBindHappyPath(t *testing.T) {
	srv := newPbServer(t)
	src := &WSSource{
		URL:        srv.wsURL(),
		MinBackoff: 10 * time.Millisecond,
		Token:      "tok-prod",
		OpenID:     "open-prod",
		CLIUniqID:  "open-prod*dev-prod",
	}
	h := runSource(t, src)

	conn := srv.accept(t)
	// First upstream frame MUST be AuthBindReq.
	first := readFrame(t, conn)
	if first.Head.Cmd != wsspb.CmdAuthBind || first.Head.CmdType != wsspb.CmdTypeUpstreamReq {
		t.Fatalf("first frame cmd=%q cmd_type=%d, want auth.bind/0",
			first.Head.Cmd, first.Head.CmdType)
	}
	// Inner AuthBindReq carries the configured creds.
	inner := &wsspb.AuthBindReq{}
	if err := proto.Unmarshal(first.Data, inner); err != nil {
		t.Fatalf("unmarshal inner: %v", err)
	}
	if inner.Token != "tok-prod" || inner.CliUniqId != "open-prod*dev-prod" {
		t.Errorf("AuthBindReq = %+v, want token=tok-prod cli_uniq_id=open-prod*dev-prod", inner)
	}
	if inner.OpenId != "open-prod" {
		t.Errorf("OpenId = %q, want open-prod", inner.OpenId)
	}
	if inner.TokenType != wsspb.AuthTokenType {
		t.Errorf("TokenType = %q, want %q", inner.TokenType, wsspb.AuthTokenType)
	}
	if inner.BizId != wsspb.AuthBizID {
		t.Errorf("BizId = %q, want %q", inner.BizId, wsspb.AuthBizID)
	}

	// Reply success → source enters read loop.
	writeAuthBindRsp(t, conn, first.Head.MsgId, 0)

	// The source emits a /conn/ping right after AuthBind succeeds; ack it
	// with a long heart_interval so we can assert the push/ack flow next.
	drainHeartReq(t, conn)

	// Push an event and assert it arrives plus an ack is written back.
	pushMsgID := writePush(t, conn, "meeting.started", "trc1",
		[]byte(`{"meeting_id":"m1"}`))

	select {
	case ev := <-h.emitted:
		if ev.Event != "meeting.started" {
			t.Errorf("Event = %q", ev.Event)
		}
		if ev.TraceID != "trc1" {
			t.Errorf("TraceID = %q, want trc1", ev.TraceID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no event emitted within 2s")
	}

	// The next frame from the source should be the ack.
	ack := readFrame(t, conn)
	if ack.Head.CmdType != wsspb.CmdTypeDownstreamAck {
		t.Errorf("ack CmdType = %d, want %d", ack.Head.CmdType, wsspb.CmdTypeDownstreamAck)
	}
	if ack.Head.MsgId != pushMsgID {
		t.Errorf("ack MsgId = %q, want %q (push msg_id)", ack.Head.MsgId, pushMsgID)
	}
	if ack.Head.Cmd != wsspb.CmdWsCLIPushEvent {
		t.Errorf("ack Cmd = %q, want %q (mirrored from push)", ack.Head.Cmd, wsspb.CmdWsCLIPushEvent)
	}
}

func TestWSSource_PB_AuthBindRejectIsFatal(t *testing.T) {
	srv := newPbServer(t)
	src := &WSSource{
		URL:        srv.wsURL(),
		MinBackoff: 10 * time.Millisecond,
		Token:      "bad-tok",
		OpenID:     "open",
		CLIUniqID:  "open*dev",
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	go func() {
		errCh <- src.Run(ctx,
			func(*eventruntime.RawEvent) {},
			func(string, string) {})
	}()

	conn := srv.accept(t)
	first := readFrame(t, conn)
	// Reply with non-zero status.
	writeAuthBindRsp(t, conn, first.Head.MsgId, 401)

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("Run should return non-nil on AuthBindRsp.ret_code!=0")
		}
		var ae *authError
		if !errors.As(err, &ae) {
			t.Errorf("expected authError, got %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Run did not return on auth reject within 3s")
	}
}

// writeAuthBindRspWithStatus replies to an AuthBindReq carrying the
// supplied envelope-level Head.Status (the gateway short-circuit
// signal — e.g. 10006 "token invalid").  The body is intentionally
// left empty: per the .proto contract, Head.status != 0 means the
// gateway rejected the frame before the auth module ran, so Data
// would be empty on the wire too.  Mirrors the production path that
// surfaces as *wsspb.UpstreamRspStatusError inside DecodeAuthBindRsp.
func writeAuthBindRspWithStatus(t *testing.T, conn *websocket.Conn, reqMsgID string, status int32) {
	t.Helper()
	rsp := &wsspb.ConnMsg{
		Head: &wsspb.Head{
			FrameType: wsspb.FrameTypeDefault,
			CmdType:   wsspb.CmdTypeUpstreamRsp,
			Cmd:       wsspb.CmdAuthBind,
			MsgId:     reqMsgID,
			Status:    status,
		},
	}
	wire, err := proto.Marshal(rsp)
	if err != nil {
		t.Fatalf("marshal rsp: %v", err)
	}
	if err := conn.WriteMessage(websocket.BinaryMessage, wire); err != nil {
		t.Fatalf("write rsp: %v", err)
	}
}

// TestWSSource_PB_AuthBindEnvelopeStatusFiresOnAuthFailed asserts the
// narrow contract of WSSource.OnAuthFailed: when the gateway rejects
// AuthBind at the envelope layer with a token-expiry status code
// (Head.Status == ServerCodeWssHeadTokenExpired = 10006), Run() invokes
// OnAuthFailed exactly once with the status code, then returns the
// underlying *authError.  This is the hook the bus daemon uses to wipe
// the now-unusable credentials from the keychain (see cmd/event/bus.go).
func TestWSSource_PB_AuthBindEnvelopeStatusFiresOnAuthFailed(t *testing.T) {
	srv := newPbServer(t)

	var (
		hookMu    sync.Mutex
		hookCalls int
		gotCode   int
	)
	src := &WSSource{
		URL:        srv.wsURL(),
		MinBackoff: 10 * time.Millisecond,
		Token:      "dead-tok",
		OpenID:     "open",
		CLIUniqID:  "open*dev",
		OnAuthFailed: func(ctx context.Context, code int, err error) {
			hookMu.Lock()
			defer hookMu.Unlock()
			hookCalls++
			gotCode = code
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	go func() {
		errCh <- src.Run(ctx,
			func(*eventruntime.RawEvent) {},
			func(string, string) {})
	}()

	conn := srv.accept(t)
	first := readFrame(t, conn)
	// Envelope-level reject with token-expiry code (status=10006 =
	// ServerCodeWssHeadTokenExpired).  Body deliberately empty: the
	// gateway short-circuited before the auth module produced an
	// AuthBindRsp.
	writeAuthBindRspWithStatus(t, conn, first.Head.MsgId, 10006)

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("Run should return non-nil on envelope-level auth reject")
		}
		var ae *authError
		if !errors.As(err, &ae) {
			t.Fatalf("expected *authError, got %T %v", err, err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Run did not return on envelope-level auth reject within 3s")
	}

	hookMu.Lock()
	defer hookMu.Unlock()
	if hookCalls != 1 {
		t.Errorf("OnAuthFailed call count = %d, want 1", hookCalls)
	}
	if gotCode != 10006 {
		t.Errorf("OnAuthFailed code = %d, want 10006", gotCode)
	}
}

// TestWSSource_PB_AuthBindRetCodeDoesNotFireOnAuthFailed verifies the
// inverse contract: a business-layer rejection (AuthBindRsp.ret_code
// != 0 with envelope Head.Status == 0) is STILL fatal — Run returns
// *authError and the daemon exits — but OnAuthFailed is NOT invoked,
// because ret_code-only failures are not authoritative "this token
// is dead at the gateway" signals (the credential may still be valid
// for other endpoints; see WSSource.OnAuthFailed doc).
func TestWSSource_PB_AuthBindRetCodeDoesNotFireOnAuthFailed(t *testing.T) {
	srv := newPbServer(t)

	var (
		hookMu    sync.Mutex
		hookCalls int
	)
	src := &WSSource{
		URL:        srv.wsURL(),
		MinBackoff: 10 * time.Millisecond,
		Token:      "bad-tok",
		OpenID:     "open",
		CLIUniqID:  "open*dev",
		OnAuthFailed: func(ctx context.Context, code int, err error) {
			hookMu.Lock()
			defer hookMu.Unlock()
			hookCalls++
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	go func() {
		errCh <- src.Run(ctx,
			func(*eventruntime.RawEvent) {},
			func(string, string) {})
	}()

	conn := srv.accept(t)
	first := readFrame(t, conn)
	// Business-layer reject: Head.Status stays 0, but the body's
	// ret_code is non-zero.  Run must still treat this as fatal
	// authError, but OnAuthFailed is reserved for the envelope path.
	writeAuthBindRsp(t, conn, first.Head.MsgId, 401)

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("Run should return non-nil on AuthBindRsp.ret_code!=0")
		}
		var ae *authError
		if !errors.As(err, &ae) {
			t.Fatalf("expected *authError, got %T %v", err, err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Run did not return on auth reject within 3s")
	}

	hookMu.Lock()
	defer hookMu.Unlock()
	if hookCalls != 0 {
		t.Errorf("OnAuthFailed call count = %d, want 0 (ret_code path must not fire envelope hook)", hookCalls)
	}
}

func TestWSSource_PB_SubscribeWritesUpstream(t *testing.T) {
	srv := newPbServer(t)
	src := &WSSource{
		URL:        srv.wsURL(),
		MinBackoff: 10 * time.Millisecond,
		Token:      "t",
		OpenID:     "o",
		CLIUniqID:  "o*d",
	}
	h := runSource(t, src)
	_ = h

	conn := srv.accept(t)
	// Drain auth req.
	authReq := readFrame(t, conn)
	writeAuthBindRsp(t, conn, authReq.Head.MsgId, 0)
	drainHeartReq(t, conn)

	// Source is now in read loop; call Subscribe from the test goroutine.
	if err := src.Subscribe(context.Background(), "meeting.started", ""); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	// Expect a SubscribeReq frame on the wire.
	subReq := readFrame(t, conn)
	if subReq.Head.Cmd != wsspb.CmdEventSubscribe {
		t.Errorf("Cmd = %q, want %q", subReq.Head.Cmd, wsspb.CmdEventSubscribe)
	}
	if subReq.Head.CmdType != wsspb.CmdTypeUpstreamReq {
		t.Errorf("CmdType = %d, want upstream req", subReq.Head.CmdType)
	}
	inner := &wsspb.SubscribeReq{}
	if err := proto.Unmarshal(subReq.Data, inner); err != nil {
		t.Fatalf("unmarshal inner: %v", err)
	}
	if len(inner.EventList) != 1 || inner.EventList[0] != "meeting.started" {
		t.Errorf("EventList = %v, want [meeting.started]", inner.EventList)
	}
}

func TestWSSource_PB_SubscribeBatchSendsOneFrame(t *testing.T) {
	srv := newPbServer(t)
	src := &WSSource{
		URL:        srv.wsURL(),
		MinBackoff: 10 * time.Millisecond,
		Token:      "t",
		OpenID:     "o",
		CLIUniqID:  "o*d",
	}
	h := runSource(t, src)
	_ = h

	conn := srv.accept(t)
	authReq := readFrame(t, conn)
	writeAuthBindRsp(t, conn, authReq.Head.MsgId, 0)
	drainHeartReq(t, conn)

	keys := []string{"k1", "k2", "k3"}
	if err := src.SubscribeBatch(context.Background(), keys, ""); err != nil {
		t.Fatalf("SubscribeBatch: %v", err)
	}

	subReq := readFrame(t, conn)
	inner := &wsspb.SubscribeReq{}
	if err := proto.Unmarshal(subReq.Data, inner); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(inner.EventList) != 3 {
		t.Errorf("EventList len = %d, want 3", len(inner.EventList))
	}
	for i, k := range keys {
		if inner.EventList[i] != k {
			t.Errorf("EventList[%d] = %q, want %q", i, inner.EventList[i], k)
		}
	}
}

func TestWSSource_PB_OnReconnectedFiresAfterAuthBind(t *testing.T) {
	srv := newPbServer(t)
	src := &WSSource{
		URL:        srv.wsURL(),
		MinBackoff: 10 * time.Millisecond,
		Token:      "t",
		OpenID:     "o",
		CLIUniqID:  "o*d",
	}

	var (
		mu        sync.Mutex
		callbackN int
	)
	src.SetOnReconnected(func() {
		mu.Lock()
		callbackN++
		mu.Unlock()
	})

	h := runSource(t, src)
	_ = h

	// 1st connect.
	conn1 := srv.accept(t)
	authReq1 := readFrame(t, conn1)
	writeAuthBindRsp(t, conn1, authReq1.Head.MsgId, 0)
	drainHeartReq(t, conn1)
	// Wait briefly for callback to land.
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	if callbackN != 1 {
		mu.Unlock()
		t.Fatalf("after 1st connect: callbackN=%d, want 1", callbackN)
	}
	mu.Unlock()

	// Force reconnect.
	_ = conn1.Close()

	conn2 := srv.accept(t)
	authReq2 := readFrame(t, conn2)
	writeAuthBindRsp(t, conn2, authReq2.Head.MsgId, 0)
	drainHeartReq(t, conn2)
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if callbackN != 2 {
		t.Errorf("after reconnect: callbackN=%d, want 2", callbackN)
	}
}

// TestWSSource_PB_DownstreamUnknownCmdDropped pins the new wire-contract
// behaviour: only cmd_type=2 frames whose Head.Cmd == CmdWsCLIPushEvent
// carry a business event notification.  Anything else on cmd_type=2 is
// non-business (reserved control / unknown extension) and must be
// dropped silently — no emit, no ack — without disturbing the live
// session.  Acking would tell the gateway we delivered something we
// never surfaced, which would corrupt at-least-once semantics.
func TestWSSource_PB_DownstreamUnknownCmdDropped(t *testing.T) {
	srv := newPbServer(t)
	src := &WSSource{
		URL:        srv.wsURL(),
		MinBackoff: 10 * time.Millisecond,
		Token:      "t",
		OpenID:     "o",
		CLIUniqID:  "o*d",
	}
	h := runSource(t, src)

	conn := srv.accept(t)
	authReq := readFrame(t, conn)
	writeAuthBindRsp(t, conn, authReq.Head.MsgId, 0)
	drainHeartReq(t, conn)

	// Send a cmd_type=2 frame with a non-WsCLIPushEvent cmd; the
	// source should drop it without emitting and without acking.
	writeRawDownstream(t, conn, "some.unknown.notify", "downstream-1",
		[]byte(`{"event":"meeting.started","trace_id":"trc-x","payload":{}}`))

	// 1) No event surfaces.
	select {
	case ev := <-h.emitted:
		t.Fatalf("unexpected emit for non-WsCLIPushEvent push: %+v", ev)
	case <-time.After(200 * time.Millisecond):
		// good — silently dropped.
	}

	// 2) No ack is issued for the dropped frame, AND the connection
	// stays healthy.  We assert both in one shot: send a follow-up
	// real WsCLIPushEvent push and read exactly one frame from the
	// source — if the source had erroneously acked the dropped frame
	// first, we'd see msg_id="downstream-1" here instead of the
	// real-push msg_id.  This avoids a separate ReadMessage probe on
	// the test's deadline (which would put gorilla's read state into
	// an error condition and break subsequent reads).
	pushMsgID := writePush(t, conn, "meeting.started", "trc-real",
		[]byte(`{"meeting_id":"m1"}`))
	select {
	case ev := <-h.emitted:
		if ev.Event != "meeting.started" || ev.TraceID != "trc-real" {
			t.Errorf("follow-up event = %+v, want meeting.started/trc-real", ev)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("follow-up real push was not emitted within 2s")
	}
	ack := readFrame(t, conn)
	if ack.Head.CmdType != wsspb.CmdTypeDownstreamAck {
		t.Fatalf("expected ack frame, got cmd_type=%d cmd=%q msg_id=%q",
			ack.Head.CmdType, ack.Head.Cmd, ack.Head.MsgId)
	}
	if ack.Head.MsgId != pushMsgID {
		// If the source had acked the dropped frame, this would be
		// "downstream-1" instead of the real-push msg_id — that's
		// the contract this test is pinning.
		t.Errorf("ack MsgId = %q, want %q (real push); source may have acked the dropped frame",
			ack.Head.MsgId, pushMsgID)
	}
	if ack.Head.Cmd != wsspb.CmdWsCLIPushEvent {
		t.Errorf("ack Cmd = %q, want %q (mirrored from real push)",
			ack.Head.Cmd, wsspb.CmdWsCLIPushEvent)
	}
}

// TestWSSource_PB_SubscribeRsp_FailureFiresCallback pins the contract
// that a non-zero SubscribeRsp.code from the gateway routes back to
// OnSubscribeResult so the bus can broadcast a subscribe_error control
// frame.  Without this, a permanently-rejected subscribe would leave
// consumers blocked forever waiting for events the gateway will never
// push.
func TestWSSource_PB_SubscribeRsp_FailureFiresCallback(t *testing.T) {
	srv := newPbServer(t)
	src := &WSSource{
		URL:        srv.wsURL(),
		MinBackoff: 10 * time.Millisecond,
		Token:      "t",
		OpenID:     "o",
		CLIUniqID:  "o*d",
	}

	type result struct {
		keys []string
		code uint32
		msg  string
	}
	resCh := make(chan result, 1)
	src.SetOnSubscribeResult(func(keys []string, code uint32, msg string) {
		resCh <- result{keys: append([]string(nil), keys...), code: code, msg: msg}
	})

	h := runSource(t, src)
	_ = h

	conn := srv.accept(t)
	authReq := readFrame(t, conn)
	writeAuthBindRsp(t, conn, authReq.Head.MsgId, 0)
	drainHeartReq(t, conn)

	// Drive a subscribe from the test goroutine.
	if err := src.Subscribe(context.Background(), "meeting.started", ""); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	// Read the SubscribeReq the source emitted.
	subReq := readFrame(t, conn)
	if subReq.Head.Cmd != wsspb.CmdEventSubscribe {
		t.Fatalf("expected SubscribeReq, got cmd=%q", subReq.Head.Cmd)
	}

	// Server replies with code=42 / msg="permission denied".
	rspInner := &wsspb.SubscribeRsp{Code: 42, Msg: "permission denied"}
	innerWire, err := proto.Marshal(rspInner)
	if err != nil {
		t.Fatalf("marshal SubscribeRsp: %v", err)
	}
	rsp := &wsspb.ConnMsg{
		Head: &wsspb.Head{
			FrameType: wsspb.FrameTypeDefault,
			CmdType:   wsspb.CmdTypeUpstreamRsp,
			Cmd:       wsspb.CmdEventSubscribe,
			MsgId:     subReq.Head.MsgId,
		},
		Data: innerWire,
	}
	wire, err := proto.Marshal(rsp)
	if err != nil {
		t.Fatalf("marshal ConnMsg: %v", err)
	}
	if err := conn.WriteMessage(websocket.BinaryMessage, wire); err != nil {
		t.Fatalf("write SubscribeRsp: %v", err)
	}

	// The OnSubscribeResult callback should fire within a reasonable
	// window (well under defaultRspTimeout = 10s).
	select {
	case got := <-resCh:
		if got.code != 42 {
			t.Errorf("code = %d, want 42", got.code)
		}
		if got.msg != "permission denied" {
			t.Errorf("msg = %q, want %q", got.msg, "permission denied")
		}
		if len(got.keys) != 1 || got.keys[0] != "meeting.started" {
			t.Errorf("keys = %v, want [meeting.started]", got.keys)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("OnSubscribeResult was not invoked within 2s after gateway returned a failure SubscribeRsp")
	}
}
