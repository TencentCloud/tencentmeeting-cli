// Package wsspb provides high-level codec helpers around the
// auto-generated tmeet_wss.pb.go bindings.
//
// All wire frames between the bus's WSS source and the Tencent Meeting
// upstream are ConnMsg envelopes:
//
//	ConnMsg { Head { frame_type, cmd_type, cmd, seq_no, msg_id, ... },
//	          data: <serialised inner message> }
//
// Cmd-type semantics:
//
//	0 — upstream request   (bus -> server, e.g. AuthBindReq, SubscribeReq)
//	1 — upstream response  (server -> bus, paired by msg_id with a prior cmd_type=0)
//	2 — downstream push    (server -> bus, business event delivery)
//	3 — downstream ack     (bus -> server, paired by msg_id with a prior cmd_type=2)
//
// Inner messages are protobuf-encoded payloads identified by Head.cmd:
//
//	"auth.bind"          AuthBindReq / AuthBindRsp
//	"auth.refresh"       AuthRefreshReq / AuthRefreshRsp     (reserved for A3)
//	"event.subscribe"    SubscribeReq / SubscribeRsp
//	"WsCLIPushEvent"     downstream business push — Head.cmd is the
//	                      sole event-notification cmd; Data is a JSON
//	                      RawEvent ({"event":..., "trace_id":..., "payload":...})
//	                      whose inner "event" field carries the
//	                      business EventKey used by the bus's hub.
//	                      Any other cmd_type=2 frame is non-business
//	                      and dropped by the source.
//
// This package owns:
//
//   - Frame builders (EncodeAuthBind, EncodeSubscribe, EncodeAck) so callers
//     don't have to wire Head fields manually each time.
//   - A single Decode entry point that returns *ConnMsg + the unmarshalled
//     inner message ready for type-switching by the WSSource.
//   - SeqGen, a thread-safe monotonic seq_no source.
//   - newMsgID, a UUID-ish identifier used in Head.msg_id.
//
// We deliberately do NOT touch the generated code in tmeet_wss.pb.go;
// regenerating from .proto would clobber any edits.
package wsspb

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sync/atomic"

	"google.golang.org/protobuf/proto"
)

// Frame-type / cmd-type constants mirroring the .proto comments.  Exposed
// as Go constants so callers don't sprinkle magic numbers.
const (
	FrameTypeDefault uint32 = 1

	CmdTypeUpstreamReq    uint32 = 0
	CmdTypeUpstreamRsp    uint32 = 1
	CmdTypeDownstreamPush uint32 = 2
	CmdTypeDownstreamAck  uint32 = 3
)

// Cmd identifiers.  Per the current Tencent Meeting wire contract,
// downstream business pushes ALWAYS carry Head.cmd == CmdWsCLIPushEvent;
// the per-event EventKey lives inside the JSON RawEvent in Data.event.
// Any other cmd value on a cmd_type=2 frame is treated as non-business
// noise by the source.
const (
	CmdPing           = "/conn/ping"
	CmdAuthBind       = "/conn/access-token-auth-bind"
	CmdAuthRefresh    = "/conn/refresh-access-token-auth-bind"
	CmdEventSubscribe = "WsCLISubscribeEvent"
	CmdWsCLIPushEvent = "WsCLIPushEvent"
)

// AuthTokenType is the only token_type value the gateway currently
// accepts.  Pinned here so AuthBindReq/AuthRefreshReq builders never
// have to repeat the literal.
const AuthTokenType = "access-token"

// AuthBizID is the only biz_id value the gateway accepts for the cli
// channel.  Pinned per the protocol spec ("恒为 web_hook_cli"); kept
// as a const so AuthBindReq builders can't accidentally vary it.
const AuthBizID = "web_hook_cli"

// Module values for Head.Module, pinned per upstream spec.
const (
	ModuleConnAccess    = "conn_access"
	ModuleWebhookNotify = "wemeet_webhook_notify_service"
)

// SeqGen produces monotonically increasing seq_no values.  The .proto
// guarantees seq_no is "ever-increasing per direction"; a single shared
// counter inside the source covers both auth + subscribe + ack writes
// with no risk of collision.  Zero-initialised value is fine since the
// first Next() returns 1.
type SeqGen struct {
	n atomic.Uint32
}

// Next returns the next seq_no.  Wraps cleanly on uint32 overflow,
// which would take ~70 days at 700 frames/sec — orders of magnitude
// beyond any realistic event stream.
func (g *SeqGen) Next() uint32 {
	return g.n.Add(1)
}

// newMsgID returns a 32-char hex string suitable for Head.msg_id.
//
// Spec says "based on uuid"; we use 16 random bytes hex-encoded which
// is RFC4122-compatible in entropy without pulling a uuid dependency
// in.  Crypto-rand is the right source: msg_id collisions across
// concurrent in-flight requests would silently misroute responses, so
// the birthday-bound on math/rand at ~2^32 ids is too tight.
func newMsgID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// rand.Read on POSIX/Windows reads /dev/urandom or
		// CryptGenRandom; failure here means the OS RNG is broken,
		// which is unrecoverable.  Falling back to a deterministic
		// id would re-introduce the collision risk we're trying to
		// avoid, so we propagate the panic.
		panic(fmt.Errorf("wsspb: rand.Read: %w", err))
	}
	return hex.EncodeToString(b[:])
}

// buildHead is a shared constructor.  Callers fill the cmd / cmd_type
// they need; everything else is centralised here so a future schema
// extension (Module, etc.) only changes one place.
func buildHead(seq *SeqGen, cmdType uint32, cmd, module, msgID string) *Head {
	if msgID == "" {
		msgID = newMsgID()
	}
	return &Head{
		FrameType: FrameTypeDefault,
		CmdType:   cmdType,
		Cmd:       cmd,
		SeqNo:     seq.Next(),
		MsgId:     msgID,
		Module:    module,
	}
}

// marshalConnMsg packs head+inner into a ConnMsg and serialises it.
// Inner may be nil (rsp/ack carrying empty payloads).
func marshalConnMsg(head *Head, inner proto.Message) ([]byte, *ConnMsg, error) {
	cm := &ConnMsg{Head: head}
	if inner != nil {
		raw, err := proto.Marshal(inner)
		if err != nil {
			return nil, nil, fmt.Errorf("wsspb: marshal inner %T: %w", inner, err)
		}
		cm.Data = raw
	}
	out, err := proto.Marshal(cm)
	if err != nil {
		return nil, nil, fmt.Errorf("wsspb: marshal ConnMsg: %w", err)
	}
	return out, cm, nil
}

// EncodeAuthBindReq builds the first frame the source sends after a
// successful WS handshake.  Returns the wire bytes plus the assigned
// msg_id so the caller can wait for the matching cmd_type=1 response.
//
// BizId / TokenType are fixed by the protocol spec and filled here so
// the WSSource layer doesn't have to know them.  cliUniqID must match
// the REST proxy's Tmeet-Unique-ID header value so the gateway can
// correlate REST and WSS sessions for the same CLI instance; callers
// should compose it via common.BuildUniqueID.
func EncodeAuthBindReq(seq *SeqGen, token, openID, cliUniqID string) (wire []byte, msgID string, err error) {
	head := buildHead(seq, CmdTypeUpstreamReq, CmdAuthBind, ModuleConnAccess, "")
	inner := &AuthBindReq{
		BizId:     AuthBizID,
		TokenType: AuthTokenType,
		Token:     token,
		OpenId:    openID,
		CliUniqId: cliUniqID,
	}
	wire, _, err = marshalConnMsg(head, inner)
	return wire, head.MsgId, err
}

// EncodeAuthRefreshReq is reserved for A3's token-refresh path; symmetric
// to EncodeAuthBindReq so when A3 lands the existing wait-for-rsp
// machinery in WSSource can be reused unchanged.
func EncodeAuthRefreshReq(seq *SeqGen, token, openID, cliUniqID string) (wire []byte, msgID string, err error) {
	head := buildHead(seq, CmdTypeUpstreamReq, CmdAuthRefresh, ModuleConnAccess, "")
	inner := &AuthRefreshReq{
		BizId:     AuthBizID,
		TokenType: AuthTokenType,
		Token:     token,
		OpenId:    openID,
		CliUniqId: cliUniqID,
	}
	wire, _, err = marshalConnMsg(head, inner)
	return wire, head.MsgId, err
}

// EncodeSubscribeReq packs eventKeys into a SubscribeReq.  Empty
// eventKeys is rejected as a programming error: the server's behaviour
// on an empty event_list isn't documented and we'd rather catch it on
// the bus side than send dubious frames upstream.
//
// agentOpenID identifies the agent (子账号) on whose behalf an
// agent-scoped event is being subscribed; it is empty for master-only
// (主账号) and unrestricted subscriptions.  One SubscribeReq carries a
// single agentOpenID, so callers MUST group eventKeys by agent before
// calling this.
func EncodeSubscribeReq(seq *SeqGen, eventKeys []string, agentOpenID string) (wire []byte, msgID string, err error) {
	if len(eventKeys) == 0 {
		return nil, "", errors.New("wsspb: SubscribeReq requires at least one event key")
	}
	head := buildHead(seq, CmdTypeUpstreamReq, CmdEventSubscribe, ModuleWebhookNotify, "")
	inner := &SubscribeReq{EventList: eventKeys, AgentOpenId: agentOpenID}
	wire, _, err = marshalConnMsg(head, inner)
	return wire, head.MsgId, err
}

// EncodeAck builds the cmd_type=3 ack frame for a previously-received
// push.  cmd is mirrored verbatim from the push's Head.Cmd; under the
// current protocol that is always CmdWsCLIPushEvent, but we keep it as
// an explicit parameter (rather than hard-coding the constant inside
// this builder) so the caller's intent — "echo back what we just
// received" — is visible at the call site and the wire value can never
// silently diverge from the source's dispatch logic.  No inner payload
// — Data is empty.
func EncodeAck(seq *SeqGen, cmd, msgID string) ([]byte, error) {
	if msgID == "" {
		return nil, errors.New("wsspb: ack requires non-empty msg_id (would silently fail correlation)")
	}
	head := buildHead(seq, CmdTypeDownstreamAck, cmd, ModuleConnAccess, msgID)
	wire, _, err := marshalConnMsg(head, nil)
	return wire, err
}

// Decode parses a raw WS binary frame into a *ConnMsg.  Inner data is
// left untouched — callers dispatch on Head.CmdType + Head.Cmd and call
// the appropriate DecodeXxx helper to unmarshal Data.
//
// We separate envelope parsing from inner parsing so a malformed
// AuthBindRsp doesn't kill the whole read loop: the caller can log,
// surface a notify, and skip the frame.
func Decode(wire []byte) (*ConnMsg, error) {
	if len(wire) == 0 {
		return nil, errors.New("wsspb: empty frame")
	}
	cm := &ConnMsg{}
	if err := proto.Unmarshal(wire, cm); err != nil {
		return nil, fmt.Errorf("wsspb: decode ConnMsg: %w", err)
	}
	if cm.Head == nil {
		return nil, errors.New("wsspb: frame missing head")
	}
	return cm, nil
}

// UpstreamRspStatusError is returned by the Decode*Rsp helpers when
// the envelope-level Head.status carries a non-zero gateway-side
// rejection.  It surfaces the originating cmd / msg_id alongside the
// status code so callers (and logs) can correlate the failure back to
// the request that triggered it without re-parsing Head.
//
// Why this exists: per .proto, Head.status is the authoritative
// envelope-level success signal for upstream rsps and is populated
// even in cases where the body is empty (gateway short-circuits before
// the business module produces a body).  Inspecting only the inner
// rsp's ret_code/code would miss those cases — Decode*Rsp would happily
// return a zero-value rsp and the caller would treat it as success.
type UpstreamRspStatusError struct {
	Cmd    string
	MsgID  string
	Status int32
}

func (e *UpstreamRspStatusError) Error() string {
	return fmt.Sprintf("wsspb: upstream rsp cmd=%q msg_id=%q non-zero status=%d",
		e.Cmd, e.MsgID, e.Status)
}

// checkUpstreamRspStatus enforces Head.status == 0 on upstream rsp
// frames (cmd_type=1).  It is a no-op for any other cmd_type because
// .proto explicitly defines status as "only meaningful on upstream rsp
// frames" — push / ack / req frames never carry a meaningful status
// and validating it would surface false positives.
//
// Returns a typed *UpstreamRspStatusError when status != 0 so callers
// that need to distinguish envelope-level rejection from body-decode
// failures can errors.As on it.
func checkUpstreamRspStatus(cm *ConnMsg) error {
	if cm == nil || cm.Head == nil {
		return nil
	}
	if cm.Head.CmdType != CmdTypeUpstreamRsp {
		return nil
	}
	if cm.Head.Status == 0 {
		return nil
	}
	return &UpstreamRspStatusError{
		Cmd:    cm.Head.Cmd,
		MsgID:  cm.Head.MsgId,
		Status: cm.Head.Status,
	}
}

// DecodeAuthBindRsp unmarshals the AuthBindRsp body.
//
// Validation order:
//
//  1. Envelope-level Head.status must be 0; otherwise the gateway
//     rejected the frame before the business module ran (Data may be
//     empty in this case) and we surface an *UpstreamRspStatusError.
//  2. Body is unmarshalled when present; an empty Data field returns a
//     zero-value rsp (some gateway implementations leave the body blank
//     on success and rely solely on the envelope signal).
//
// Callers that need the gateway's textual error (AuthBindRsp.Msg) should
// still branch on rsp.RetCode after this returns; ret_code is the
// finer-grained business-layer signal layered on top of Head.status.
func DecodeAuthBindRsp(cm *ConnMsg) (*AuthBindRsp, error) {
	if cm == nil {
		return nil, errors.New("wsspb: nil ConnMsg")
	}
	if err := checkUpstreamRspStatus(cm); err != nil {
		return nil, err
	}
	rsp := &AuthBindRsp{}
	if len(cm.Data) == 0 {
		return rsp, nil
	}
	if err := proto.Unmarshal(cm.Data, rsp); err != nil {
		return nil, fmt.Errorf("wsspb: decode AuthBindRsp: %w", err)
	}
	return rsp, nil
}

// DecodeAuthRefreshRsp — symmetric to DecodeAuthBindRsp.  Used by the
// heartbeat-path in-session token refresh to verify the gateway accepted
// the new token rebinding before the next ping goes out.
func DecodeAuthRefreshRsp(cm *ConnMsg) (*AuthRefreshRsp, error) {
	if cm == nil {
		return nil, errors.New("wsspb: nil ConnMsg")
	}
	if err := checkUpstreamRspStatus(cm); err != nil {
		return nil, err
	}
	rsp := &AuthRefreshRsp{}
	if len(cm.Data) == 0 {
		return rsp, nil
	}
	if err := proto.Unmarshal(cm.Data, rsp); err != nil {
		return nil, fmt.Errorf("wsspb: decode AuthRefreshRsp: %w", err)
	}
	return rsp, nil
}

// DecodeSubscribeRsp — symmetric to DecodeAuthBindRsp.
func DecodeSubscribeRsp(cm *ConnMsg) (*SubscribeRsp, error) {
	if cm == nil {
		return nil, errors.New("wsspb: nil ConnMsg")
	}
	if err := checkUpstreamRspStatus(cm); err != nil {
		return nil, err
	}
	rsp := &SubscribeRsp{}
	if len(cm.Data) == 0 {
		return rsp, nil
	}
	if err := proto.Unmarshal(cm.Data, rsp); err != nil {
		return nil, fmt.Errorf("wsspb: decode SubscribeRsp: %w", err)
	}
	return rsp, nil
}

// EncodeHeartReq builds an upstream heartbeat (cmd_type=0, cmd="/conn/ping").
// The request carries no inner payload — the server replies with a
// HeartRsp whose heart_interval drives the client-side scheduler.
//
// We reuse the standard upstream req/rsp correlation: the assigned
// msg_id is returned so the caller can register a pending future and
// pick up the matching cmd_type=1 reply.
func EncodeHeartReq(seq *SeqGen) (wire []byte, msgID string, err error) {
	head := buildHead(seq, CmdTypeUpstreamReq, CmdPing, ModuleConnAccess, "")
	wire, _, err = marshalConnMsg(head, nil)
	return wire, head.MsgId, err
}

// DecodeHeartRsp — symmetric to DecodeAuthBindRsp.  Empty Data is
// tolerated: the gateway is expected to populate heart_interval but a
// zero value simply means the caller falls back to its configured
// default.
func DecodeHeartRsp(cm *ConnMsg) (*HeartRsp, error) {
	if cm == nil {
		return nil, errors.New("wsspb: nil ConnMsg")
	}
	if err := checkUpstreamRspStatus(cm); err != nil {
		return nil, err
	}
	rsp := &HeartRsp{}
	if len(cm.Data) == 0 {
		return rsp, nil
	}
	if err := proto.Unmarshal(cm.Data, rsp); err != nil {
		return nil, fmt.Errorf("wsspb: decode HeartRsp: %w", err)
	}
	return rsp, nil
}
