// codec_test.go — round-trip coverage for the wsspb high-level helpers.
//
// We pin five behaviours:
//
//  1. Each Encode* function produces a wire blob that Decode reads back
//     into the expected Head.cmd / Head.cmd_type / Head.msg_id.
//  2. Inner messages survive the trip with their fields intact.
//  3. SeqGen is monotonic and concurrent-safe (-race).
//  4. EncodeSubscribeReq rejects an empty list (would silently misbehave
//     against the gateway).
//  5. EncodeAck rejects an empty msg_id (would silently misbehave: the
//     server has no way to correlate the ack).

package wsspb

import (
	"errors"
	"sync"
	"testing"

	"google.golang.org/protobuf/proto"
)

func TestEncodeAuthBindReq_RoundTrip(t *testing.T) {
	seq := &SeqGen{}
	wire, msgID, err := EncodeAuthBindReq(seq, "tok-123", "open-xyz", "open-xyz*device-abc")
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if msgID == "" {
		t.Fatal("msgID should be assigned")
	}

	cm, err := Decode(wire)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if cm.Head.Cmd != CmdAuthBind {
		t.Errorf("Head.Cmd = %q, want %q", cm.Head.Cmd, CmdAuthBind)
	}
	if cm.Head.CmdType != CmdTypeUpstreamReq {
		t.Errorf("Head.CmdType = %d, want %d", cm.Head.CmdType, CmdTypeUpstreamReq)
	}
	if cm.Head.MsgId != msgID {
		t.Errorf("Head.MsgId = %q, want %q", cm.Head.MsgId, msgID)
	}
	if cm.Head.SeqNo != 1 {
		t.Errorf("Head.SeqNo = %d, want 1", cm.Head.SeqNo)
	}

	inner := &AuthBindReq{}
	if err := proto.Unmarshal(cm.Data, inner); err != nil {
		t.Fatalf("unmarshal inner: %v", err)
	}
	if inner.TokenType != AuthTokenType {
		t.Errorf("TokenType = %q, want %q", inner.TokenType, AuthTokenType)
	}
	if inner.BizId != AuthBizID {
		t.Errorf("BizId = %q, want %q", inner.BizId, AuthBizID)
	}
	if inner.Token != "tok-123" {
		t.Errorf("Token = %q, want %q", inner.Token, "tok-123")
	}
	if inner.OpenId != "open-xyz" {
		t.Errorf("OpenId = %q, want %q", inner.OpenId, "open-xyz")
	}
	if inner.CliUniqId != "open-xyz*device-abc" {
		t.Errorf("CliUniqId = %q, want %q", inner.CliUniqId, "open-xyz*device-abc")
	}
}

func TestEncodeSubscribeReq_RoundTrip(t *testing.T) {
	seq := &SeqGen{}
	keys := []string{"meeting.started", "meeting.recording_completed"}
	const agent = "agent-open-id-123"
	wire, _, err := EncodeSubscribeReq(seq, keys, agent)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	cm, err := Decode(wire)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if cm.Head.Cmd != CmdEventSubscribe {
		t.Errorf("Head.Cmd = %q, want %q", cm.Head.Cmd, CmdEventSubscribe)
	}

	inner := &SubscribeReq{}
	if err := proto.Unmarshal(cm.Data, inner); err != nil {
		t.Fatalf("unmarshal inner: %v", err)
	}
	if len(inner.EventList) != len(keys) {
		t.Fatalf("EventList len = %d, want %d", len(inner.EventList), len(keys))
	}
	for i, k := range keys {
		if inner.EventList[i] != k {
			t.Errorf("EventList[%d] = %q, want %q", i, inner.EventList[i], k)
		}
	}
	if inner.AgentOpenId != agent {
		t.Errorf("AgentOpenId = %q, want %q", inner.AgentOpenId, agent)
	}
}

func TestEncodeSubscribeReq_RejectsEmpty(t *testing.T) {
	if _, _, err := EncodeSubscribeReq(&SeqGen{}, nil, ""); err == nil {
		t.Error("nil eventKeys: want error, got nil")
	}
	if _, _, err := EncodeSubscribeReq(&SeqGen{}, []string{}, ""); err == nil {
		t.Error("empty eventKeys: want error, got nil")
	}
}

func TestEncodeAck_RoundTrip(t *testing.T) {
	seq := &SeqGen{}
	wire, err := EncodeAck(seq, CmdWsCLIPushEvent, "msg-xyz")
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	cm, err := Decode(wire)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if cm.Head.Cmd != CmdWsCLIPushEvent {
		t.Errorf("Head.Cmd = %q, want %q", cm.Head.Cmd, CmdWsCLIPushEvent)
	}
	if cm.Head.CmdType != CmdTypeDownstreamAck {
		t.Errorf("Head.CmdType = %d, want %d", cm.Head.CmdType, CmdTypeDownstreamAck)
	}
	if cm.Head.MsgId != "msg-xyz" {
		t.Errorf("Head.MsgId = %q, want %q", cm.Head.MsgId, "msg-xyz")
	}
	if len(cm.Data) != 0 {
		t.Errorf("ack should carry empty Data, got %d bytes", len(cm.Data))
	}
}

func TestEncodeAck_RejectsEmptyMsgID(t *testing.T) {
	if _, err := EncodeAck(&SeqGen{}, CmdWsCLIPushEvent, ""); err == nil {
		t.Error("empty msgID: want error, got nil")
	}
}

func TestSeqGen_Monotonic(t *testing.T) {
	seq := &SeqGen{}
	for i := uint32(1); i <= 100; i++ {
		if got := seq.Next(); got != i {
			t.Errorf("Next() = %d, want %d", got, i)
		}
	}
}

func TestSeqGen_ConcurrentSafe(t *testing.T) {
	// -race validates the lack of races; we additionally verify total
	// ordering: 8 goroutines × 100 increments must yield distinct values
	// 1..800.
	seq := &SeqGen{}
	const workers = 8
	const perWorker = 100

	var (
		mu   sync.Mutex
		seen = make(map[uint32]struct{}, workers*perWorker)
	)

	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < perWorker; i++ {
				v := seq.Next()
				mu.Lock()
				seen[v] = struct{}{}
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	if len(seen) != workers*perWorker {
		t.Errorf("got %d distinct seqs, want %d", len(seen), workers*perWorker)
	}
}

func TestDecode_RejectsEmpty(t *testing.T) {
	if _, err := Decode(nil); err == nil {
		t.Error("nil wire: want error")
	}
	if _, err := Decode([]byte{}); err == nil {
		t.Error("empty wire: want error")
	}
}

func TestDecode_AuthBindRsp_Success(t *testing.T) {
	// Server returns an AuthBindRsp with ret_code=0 and a session_id —
	// the canonical success case.  Decode should surface all fields
	// verbatim so callers can persist session_id / connect_id for
	// observability.
	innerWire, err := proto.Marshal(&AuthBindRsp{
		RetCode:   0,
		Msg:       "",
		SessionId: "sess-1",
		ConnectId: "conn-1",
	})
	if err != nil {
		t.Fatalf("marshal AuthBindRsp: %v", err)
	}
	wire, err := proto.Marshal(&ConnMsg{
		Head: &Head{
			FrameType: FrameTypeDefault,
			CmdType:   CmdTypeUpstreamRsp,
			Cmd:       CmdAuthBind,
			SeqNo:     7,
			MsgId:     "msg-abc",
		},
		Data: innerWire,
	})
	if err != nil {
		t.Fatalf("marshal ConnMsg: %v", err)
	}
	cm, err := Decode(wire)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	rsp, err := DecodeAuthBindRsp(cm)
	if err != nil {
		t.Fatalf("DecodeAuthBindRsp: %v", err)
	}
	if rsp == nil {
		t.Fatal("rsp should be non-nil")
	}
	if rsp.RetCode != 0 {
		t.Errorf("RetCode = %d, want 0", rsp.RetCode)
	}
	if rsp.SessionId != "sess-1" {
		t.Errorf("SessionId = %q, want sess-1", rsp.SessionId)
	}
	if rsp.ConnectId != "conn-1" {
		t.Errorf("ConnectId = %q, want conn-1", rsp.ConnectId)
	}
}

func TestDecode_AuthBindRsp_Failure(t *testing.T) {
	// Non-zero ret_code surfaces as the gateway's auth rejection.  We
	// pin Msg propagation here because wssource.go embeds it in the
	// authError message for diagnostics.
	innerWire, err := proto.Marshal(&AuthBindRsp{
		RetCode: 4001,
		Msg:     "token expired",
	})
	if err != nil {
		t.Fatalf("marshal AuthBindRsp: %v", err)
	}
	wire, err := proto.Marshal(&ConnMsg{
		Head: &Head{
			FrameType: FrameTypeDefault,
			CmdType:   CmdTypeUpstreamRsp,
			Cmd:       CmdAuthBind,
			MsgId:     "m",
		},
		Data: innerWire,
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	cm, err := Decode(wire)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	rsp, err := DecodeAuthBindRsp(cm)
	if err != nil {
		t.Fatalf("DecodeAuthBindRsp: %v", err)
	}
	if rsp.RetCode != 4001 {
		t.Errorf("RetCode = %d, want 4001", rsp.RetCode)
	}
	if rsp.Msg != "token expired" {
		t.Errorf("Msg = %q, want token expired", rsp.Msg)
	}
}

func TestDecode_SubscribeRsp_Nested(t *testing.T) {
	// SubscribeRsp shape changed: it now carries a nested Data with
	// has_sub_event_list (the gateway echoes the full set of keys it
	// considers subscribed for this session).  We exercise the nested
	// codec path so a future regen that breaks the field number / name
	// is caught here rather than at runtime.
	innerWire, err := proto.Marshal(&SubscribeRsp{
		Code: 0,
		Msg:  "",
		Data: &SubscribeRsp_Data{
			HasSubEventList: []string{"meeting.started", "meeting.end"},
		},
	})
	if err != nil {
		t.Fatalf("marshal SubscribeRsp: %v", err)
	}
	wire, err := proto.Marshal(&ConnMsg{
		Head: &Head{
			FrameType: FrameTypeDefault,
			CmdType:   CmdTypeUpstreamRsp,
			Cmd:       CmdEventSubscribe,
			MsgId:     "m",
		},
		Data: innerWire,
	})
	if err != nil {
		t.Fatalf("marshal ConnMsg: %v", err)
	}
	cm, err := Decode(wire)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	rsp, err := DecodeSubscribeRsp(cm)
	if err != nil {
		t.Fatalf("DecodeSubscribeRsp: %v", err)
	}
	if rsp.Code != 0 {
		t.Errorf("Code = %d, want 0", rsp.Code)
	}
	if rsp.Data == nil {
		t.Fatal("Data should be non-nil")
	}
	got := rsp.Data.HasSubEventList
	if len(got) != 2 || got[0] != "meeting.started" || got[1] != "meeting.end" {
		t.Errorf("HasSubEventList = %v, want [meeting.started meeting.end]", got)
	}
}

// envelopeFrame is a tiny test helper that builds a marshalled ConnMsg
// with the given upstream-rsp envelope state and pre-serialised inner
// body.  Centralised so each Decode*Rsp envelope-status case below stays
// focused on the assertion rather than re-stating the framing boilerplate.
func envelopeFrame(t *testing.T, cmd string, status int32, innerWire []byte) *ConnMsg {
	t.Helper()
	wire, err := proto.Marshal(&ConnMsg{
		Head: &Head{
			FrameType: FrameTypeDefault,
			CmdType:   CmdTypeUpstreamRsp,
			Cmd:       cmd,
			MsgId:     "msg-status",
			Status:    status,
		},
		Data: innerWire,
	})
	if err != nil {
		t.Fatalf("marshal ConnMsg: %v", err)
	}
	cm, err := Decode(wire)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	return cm
}

// assertStatusErr verifies the decoder surfaced an *UpstreamRspStatusError
// with the expected status and cmd.  We pin both fields because the
// caller in wssource.go uses errors.As + statusErr.Status to populate
// the authError code; a regression that drops the typed error or
// blanks the fields would silently degrade auth diagnostics.
func assertStatusErr(t *testing.T, err error, wantCmd string, wantStatus int32) {
	t.Helper()
	if err == nil {
		t.Fatal("expected non-nil err for status != 0")
	}
	var se *UpstreamRspStatusError
	if !errors.As(err, &se) {
		t.Fatalf("err = %T %v; want *UpstreamRspStatusError", err, err)
	}
	if se.Cmd != wantCmd {
		t.Errorf("Cmd = %q, want %q", se.Cmd, wantCmd)
	}
	if se.Status != wantStatus {
		t.Errorf("Status = %d, want %d", se.Status, wantStatus)
	}
}

func TestDecodeAuthBindRsp_StatusNonZero_EmptyData(t *testing.T) {
	// Gateway short-circuits envelope-level (e.g. unknown cmd) and
	// returns no body at all.  Without the Head.status check the
	// decoder would happily return a zero-value AuthBindRsp and the
	// caller would treat it as ret_code=0 success.
	cm := envelopeFrame(t, CmdAuthBind, 4002, nil)
	_, err := DecodeAuthBindRsp(cm)
	assertStatusErr(t, err, CmdAuthBind, 4002)
}

func TestDecodeAuthBindRsp_StatusNonZero_WithBody(t *testing.T) {
	// Status is the envelope-level signal; even when the body decodes
	// cleanly with ret_code=0, a non-zero Head.status must take
	// precedence (envelope is authoritative per the .proto contract).
	innerWire, err := proto.Marshal(&AuthBindRsp{RetCode: 0, SessionId: "should-be-ignored"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	cm := envelopeFrame(t, CmdAuthBind, 5001, innerWire)
	_, derr := DecodeAuthBindRsp(cm)
	assertStatusErr(t, derr, CmdAuthBind, 5001)
}

func TestDecodeAuthRefreshRsp_StatusNonZero(t *testing.T) {
	cm := envelopeFrame(t, CmdAuthRefresh, 4003, nil)
	_, err := DecodeAuthRefreshRsp(cm)
	assertStatusErr(t, err, CmdAuthRefresh, 4003)
}

func TestDecodeSubscribeRsp_StatusNonZero(t *testing.T) {
	cm := envelopeFrame(t, CmdEventSubscribe, 4004, nil)
	_, err := DecodeSubscribeRsp(cm)
	assertStatusErr(t, err, CmdEventSubscribe, 4004)
}

func TestDecodeHeartRsp_StatusNonZero(t *testing.T) {
	cm := envelopeFrame(t, CmdPing, 4005, nil)
	_, err := DecodeHeartRsp(cm)
	assertStatusErr(t, err, CmdPing, 4005)
}

func TestDecodeAuthRefreshRsp_StatusZero_EmptyBody(t *testing.T) {
	// Healthy in-session refresh: envelope OK + body empty.  Decoder
	// must return a zero-value rsp and nil error so refreshTokenIfNeeded
	// can branch on rsp.GetRetCode() (also zero) and treat as success.
	cm := envelopeFrame(t, CmdAuthRefresh, 0, nil)
	rsp, err := DecodeAuthRefreshRsp(cm)
	if err != nil {
		t.Fatalf("DecodeAuthRefreshRsp: %v", err)
	}
	if rsp == nil {
		t.Fatal("rsp should be non-nil")
	}
	if rsp.GetRetCode() != 0 {
		t.Errorf("RetCode = %d, want 0", rsp.GetRetCode())
	}
}

func TestCheckUpstreamRspStatus_NonRspFramesIgnored(t *testing.T) {
	// status is "only meaningful on upstream rsp frames" per .proto.
	// A push/ack/req frame carrying a non-zero status field (which
	// should never happen in practice but could surface via a
	// proto-schema regression) must NOT be treated as a failure by the
	// decoder — otherwise legitimate downstream pushes would be dropped.
	for _, ct := range []uint32{CmdTypeUpstreamReq, CmdTypeDownstreamPush, CmdTypeDownstreamAck} {
		cm := &ConnMsg{Head: &Head{CmdType: ct, Cmd: "x", Status: 9999}}
		if err := checkUpstreamRspStatus(cm); err != nil {
			t.Errorf("cmd_type=%d: want nil err, got %v", ct, err)
		}
	}
}
