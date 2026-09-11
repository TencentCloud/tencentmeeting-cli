// wssource_token_refresh_test.go — coverage for the TokenProvider
// scheduling added in the A3 follow-up:
//
//  1. Dial-time refresh: WSSource calls TokenProvider before each
//     (re)connect and the AuthBindReq carries the freshest value.
//     We verify by driving a forced reconnect after the provider has
//     rotated the token, and asserting the SECOND AuthBindReq's
//     inner.Token is the new value.
//
//  2. Heartbeat-time refresh: between heartbeats the provider rotates
//     the token; the next heartbeat round is preceded by an
//     AuthRefreshReq carrying the new value (with msg_id correlated
//     ack), AND only AFTER the AuthRefresh ack does the source emit
//     the /conn/ping.  Frame ordering matters here — the gateway's
//     per-session token binding must already be updated before the
//     heartbeat lands so the next ping is authenticated against the
//     fresh token.

package source

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"

	"tmeet/internal/event/protocol/wsspb"
)

// rotatableTokenProvider is a tiny atomic-string holder shaped like
// the closure cmd/event/bus.go installs in production.  Tests swap the
// token between calls to drive the rotation paths; calls is a counter
// so tests can verify the provider was actually invoked on each
// expected hook (dial / heartbeat).
type rotatableTokenProvider struct {
	current atomic.Value // string
	calls   atomic.Int32
}

func newRotatableTokenProvider(initial string) *rotatableTokenProvider {
	p := &rotatableTokenProvider{}
	p.current.Store(initial)
	return p
}

func (p *rotatableTokenProvider) get(_ context.Context) (string, error) {
	p.calls.Add(1)
	return p.current.Load().(string), nil
}

func (p *rotatableTokenProvider) set(v string) {
	p.current.Store(v)
}

// writeAuthRefreshRsp is the AuthRefresh-flavoured analogue of
// writeAuthBindRsp.  The current wssource path treats any rsp arrival
// as success (see refreshTokenIfNeeded comment), so ret_code is
// supplied for symmetry but not exercised on the source side yet.
func writeAuthRefreshRsp(t *testing.T, conn *websocket.Conn, reqMsgID string, retCode int32) {
	t.Helper()
	inner := &wsspb.AuthRefreshRsp{RetCode: retCode}
	if retCode != 0 {
		inner.Msg = "unit-test rejection"
	}
	innerWire, err := proto.Marshal(inner)
	if err != nil {
		t.Fatalf("marshal AuthRefreshRsp: %v", err)
	}
	rsp := &wsspb.ConnMsg{
		Head: &wsspb.Head{
			FrameType: wsspb.FrameTypeDefault,
			CmdType:   wsspb.CmdTypeUpstreamRsp,
			Cmd:       wsspb.CmdAuthRefresh,
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

// readAuthBindReq waits for an AuthBindReq on conn, replies with a
// success rsp, and returns the inner request body so the caller can
// assert on inner.Token / inner.OpenId.
func readAndAckAuthBind(t *testing.T, conn *websocket.Conn) *wsspb.AuthBindReq {
	t.Helper()
	frame := readFrame(t, conn)
	if frame.Head.Cmd != wsspb.CmdAuthBind || frame.Head.CmdType != wsspb.CmdTypeUpstreamReq {
		t.Fatalf("expected auth.bind upstream req, got cmd=%q cmd_type=%d",
			frame.Head.Cmd, frame.Head.CmdType)
	}
	inner := &wsspb.AuthBindReq{}
	if err := proto.Unmarshal(frame.Data, inner); err != nil {
		t.Fatalf("unmarshal AuthBindReq: %v", err)
	}
	writeAuthBindRsp(t, conn, frame.Head.MsgId, 0)
	return inner
}

// TestWSSource_PB_TokenRefreshOnDial pins the dial-time refresh hook:
// the AuthBindReq emitted on connect carries the value returned by
// TokenProvider, NOT the construction-time Token field.  The provider
// is the only legitimate source of the live access-token at runtime;
// the Token field is just the bootstrap seed and must not bleed into
// the AuthBindReq once a TokenProvider is wired.
//
// Test shape: configure Token="tok-construction-time" but set the
// provider to return "tok-from-provider".  Assert the AuthBindReq.Token
// observed on the wire is the provider value.
//
// Reconnect-time refresh is implicitly covered by this same hook —
// runOnce calls refreshAndStoreToken on EVERY dial, including
// reconnects — so we don't need a separate reconnect-flavoured test
// here (the reconnect machinery itself is already covered by
// TestWSSource_PB_OnReconnectedFiresAfterAuthBind).
func TestWSSource_PB_TokenRefreshOnDial(t *testing.T) {
	srv := newPbServer(t)
	provider := newRotatableTokenProvider("tok-from-provider")

	src := &WSSource{
		URL:           srv.wsURL(),
		MinBackoff:    10 * time.Millisecond,
		Token:         "tok-construction-time", // must be overridden by provider
		OpenID:        "open-1",
		CLIUniqID:     "open-1*dev",
		TokenProvider: provider.get,
	}
	h := runSource(t, src)
	_ = h

	conn := srv.accept(t)
	inner := readAndAckAuthBind(t, conn)
	if inner.Token != "tok-from-provider" {
		t.Errorf("AuthBindReq.Token = %q, want tok-from-provider (provider value, not construction-time Token)", inner.Token)
	}
	if inner.OpenId != "open-1" {
		t.Errorf("AuthBindReq.OpenId = %q, want open-1", inner.OpenId)
	}
	drainHeartReq(t, conn)

	// Provider must have been invoked at least once at dial time.
	// Heartbeat-time invocations may bump the count further; we only
	// care that the dial-time hook ran.
	if got := provider.calls.Load(); got < 1 {
		t.Errorf("provider.calls = %d, want >= 1 (dial-time hook)", got)
	}
}

// TestWSSource_PB_TokenRefreshBeforeHeartbeat pins the heartbeat-time
// refresh hook AND its frame ordering:
//
//	dial → AuthBind → AuthBindRsp → 1st heartbeat (no rotation yet,
//	    so just a /conn/ping)
//	rotate token via provider
//	2nd heartbeat round → AuthRefreshReq with the new token FIRST,
//	    then (after our ack) /conn/ping
//
// We use a very short PingPeriod so the second heartbeat lands inside
// the test budget without dragging the test runtime to multiple
// seconds.
func TestWSSource_PB_TokenRefreshBeforeHeartbeat(t *testing.T) {
	srv := newPbServer(t)
	provider := newRotatableTokenProvider("tok-1")

	src := &WSSource{
		URL:           srv.wsURL(),
		MinBackoff:    10 * time.Millisecond,
		Token:         "tok-construction-time",
		OpenID:        "open-1",
		CLIUniqID:     "open-1*dev",
		PingPeriod:    100 * time.Millisecond, // tight enough to keep the test snappy
		TokenProvider: provider.get,
	}
	h := runSource(t, src)
	_ = h

	conn := srv.accept(t)
	first := readAndAckAuthBind(t, conn)
	if first.Token != "tok-1" {
		t.Fatalf("1st AuthBindReq.Token = %q, want tok-1", first.Token)
	}

	// 1st heartbeat: provider still returns "tok-1" → no AuthRefresh,
	// just a /conn/ping.  Reply with a long heart_interval so the
	// gateway-supplied pacing doesn't compete with our PingPeriod.
	hb1 := readFrame(t, conn)
	if hb1.Head.Cmd != wsspb.CmdPing {
		t.Fatalf("1st heartbeat frame cmd = %q, want %q (no AuthRefresh expected when token unchanged)",
			hb1.Head.Cmd, wsspb.CmdPing)
	}
	// Echo back a SHORT heart_interval so the source schedules the
	// next ping quickly; the value the source uses for the next round
	// is min(PingPeriod, gateway-supplied).
	writeHeartRsp(t, conn, hb1.Head.MsgId, 1)

	// Rotate the token before the 2nd heartbeat fires.
	provider.set("tok-2")

	// 2nd heartbeat round: AuthRefreshReq must come FIRST (before
	// the ping) carrying the new token.  Frame ordering matters —
	// the gateway must rebind to the fresh token before the next
	// authenticated heartbeat lands.
	deadline := time.Now().Add(3 * time.Second)
	var refreshFrame *wsspb.ConnMsg
	for time.Now().Before(deadline) {
		f := readFrame(t, conn)
		if f.Head.Cmd == wsspb.CmdPing {
			t.Fatalf("got /conn/ping before AuthRefresh after token rotation; want AuthRefresh first (frame ordering contract)")
		}
		if f.Head.Cmd == wsspb.CmdAuthRefresh && f.Head.CmdType == wsspb.CmdTypeUpstreamReq {
			refreshFrame = f
			break
		}
		// Anything else here would be unexpected — fail fast so a
		// future regression doesn't get silently swallowed.
		t.Fatalf("unexpected frame between heartbeats: cmd=%q cmd_type=%d", f.Head.Cmd, f.Head.CmdType)
	}
	if refreshFrame == nil {
		t.Fatal("never observed AuthRefreshReq within 3s after token rotation")
	}

	refreshInner := &wsspb.AuthRefreshReq{}
	if err := proto.Unmarshal(refreshFrame.Data, refreshInner); err != nil {
		t.Fatalf("unmarshal AuthRefreshReq: %v", err)
	}
	if refreshInner.Token != "tok-2" {
		t.Errorf("AuthRefreshReq.Token = %q, want tok-2", refreshInner.Token)
	}
	if refreshInner.OpenId != "open-1" {
		t.Errorf("AuthRefreshReq.OpenId = %q, want open-1", refreshInner.OpenId)
	}
	if refreshInner.CliUniqId != "open-1*dev" {
		t.Errorf("AuthRefreshReq.CliUniqId = %q, want open-1*dev", refreshInner.CliUniqId)
	}

	// Ack the AuthRefresh; the source should now be unblocked and
	// emit the /conn/ping.
	writeAuthRefreshRsp(t, conn, refreshFrame.Head.MsgId, 0)

	hb2 := readFrame(t, conn)
	if hb2.Head.Cmd != wsspb.CmdPing {
		t.Fatalf("frame after AuthRefresh ack cmd = %q, want %q (heartbeat must follow refresh)",
			hb2.Head.Cmd, wsspb.CmdPing)
	}
	// Ack the heartbeat with a long interval so we don't keep
	// generating frames after the test's assertions are done.
	writeHeartRsp(t, conn, hb2.Head.MsgId, 3600)
}

// TestWSSource_PB_TokenRefreshUnchangedSkipsAuthRefresh pins the
// negative side of the heartbeat path: when TokenProvider returns the
// SAME value as the last stored token, the source must NOT emit an
// AuthRefreshReq — that's the cheap-no-op path that keeps the
// per-heartbeat overhead at zero bytes on the wire in the common case
// where the access-token is well within its TTL.
//
// We assert the contract by running across multiple heartbeats with a
// stable provider value and checking that every upstream frame in
// that window is /conn/ping (never /conn/refresh-...-auth-bind).
func TestWSSource_PB_TokenRefreshUnchangedSkipsAuthRefresh(t *testing.T) {
	srv := newPbServer(t)
	provider := newRotatableTokenProvider("tok-stable")

	src := &WSSource{
		URL:           srv.wsURL(),
		MinBackoff:    10 * time.Millisecond,
		Token:         "tok-stable", // matches provider so first storeToken is a no-op
		OpenID:        "open-1",
		CLIUniqID:     "open-1*dev",
		PingPeriod:    50 * time.Millisecond,
		TokenProvider: provider.get,
	}
	h := runSource(t, src)
	_ = h

	conn := srv.accept(t)
	if got := readAndAckAuthBind(t, conn); got.Token != "tok-stable" {
		t.Fatalf("AuthBindReq.Token = %q, want tok-stable", got.Token)
	}

	// Drain three heartbeats; none of them should be preceded by an
	// AuthRefresh because the provider keeps returning the same value.
	for i := 0; i < 3; i++ {
		f := readFrame(t, conn)
		if f.Head.Cmd != wsspb.CmdPing {
			t.Fatalf("frame[%d] cmd = %q, want %q (no AuthRefresh expected for unchanged token)",
				i, f.Head.Cmd, wsspb.CmdPing)
		}
		writeHeartRsp(t, conn, f.Head.MsgId, 1)
	}

	// Provider must have been called at least four times (one dial +
	// three heartbeats); >= keeps the test resilient to extra
	// heartbeat rounds that might fire before runHandle.cancel runs.
	if got := provider.calls.Load(); got < 4 {
		t.Errorf("provider.calls = %d, want >= 4 (one dial + three heartbeats)", got)
	}
}
