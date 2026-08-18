// Package protocol defines the newline-delimited JSON wire format used between
// the bus daemon and the consumer process (north-bound IPC).
//
// Frames are length-unprefixed: one JSON object per line, terminated by '\n'.
// All frames carry a "type" envelope so the reader can dispatch without a
// schema-per-payload decoder.
//
// Frame catalogue:
//
//	hello / hello_ack              — handshake; bus verifies openid_hash against bus.meta
//	event                          — business event delivery (RawEvent shape)
//	control + kind=dropped         — slow-consumer notification (aggregated by hub)
//	control + kind=source_status   — WSS state-machine broadcast
//	control + kind=subscribe_error — upstream WsCLISubscribeEvent rejected this
//	                                 EventKey; routed only to consumers of that key
//	control + kind=bye             — bus tells this conn to terminate
//	status_query / status_response — used by `event status` (control plane)
//	shutdown                       — used by `event stop` to ask bus to terminate
//
// Crucially, *event* frames carry no `seq` field: drops are surfaced
// out-of-band via control frames, never inferred from gaps.  This is the
// main reason this file does not mirror lark-cli's protocol verbatim.
package protocol

import "encoding/json"

// Message-type discriminators carried in the "type" field of every frame.
const (
	MsgTypeHello          = "hello"
	MsgTypeHelloAck       = "hello_ack"
	MsgTypeEvent          = "event"
	MsgTypeControl        = "control"
	MsgTypeBye            = "bye" // legacy alias; control+kind=bye is preferred but Bye is kept for symmetry with stop path
	MsgTypeStatusQuery    = "status_query"
	MsgTypeStatusResponse = "status_response"
	MsgTypeShutdown       = "shutdown"
)

// Control-frame kinds (only valid when Type==MsgTypeControl).
const (
	ControlKindDropped        = "dropped"
	ControlKindSourceStatus   = "source_status"
	ControlKindSubscribeError = "subscribe_error"
	ControlKindBye            = "bye"
)

// Source-state values broadcast via control+kind=source_status.
const (
	SourceStateConnecting   = "connecting"
	SourceStateReconnecting = "reconnecting"
	SourceStateSteady       = "steady"
	SourceStateAuthFailed   = "auth_failed"
	SourceStateAuthExpired  = "auth_expired"
	SourceStateDisconnected = "disconnected"
)

// Hello-error reason codes carried in HelloAck.Error.
const (
	HelloErrWrongOwner    = "WrongOwner"
	HelloErrUnknownKey    = "UnknownEventKey"
	HelloErrInvalidParams = "InvalidParams"
)

// Hello — first frame the consumer writes after Dial.
//
// The bus uses OpenIDHash to verify ownership against bus.meta;
// mismatch => HelloAck{Error:WrongOwner} and the bus closes the connection.
//
// EventKeys is a list to leave room for future multi-key subscriptions
// (`--event-id "a|b"` or `--domain <d>`).  This release only supports a
// single-key subscription per Hello — the bus rejects Hellos whose
// EventKeys length is not exactly 1 with HelloAck{Error:InvalidParams}.
// Keeping the wire shape a slice avoids a protocol churn when multi-key
// support lands.
type Hello struct {
	Type       string            `json:"type"`
	PID        int               `json:"pid"`
	EventKeys  []string          `json:"event_keys"`
	Params     map[string]string `json:"params,omitempty"`
	OpenIDHash string            `json:"openid_hash"`
	Version    string            `json:"version"`
	TraceID    string            `json:"trace_id,omitempty"` // for end-to-end log correlation

	// AgentOpenID carries the agent (子账号) open_id for EventKeys whose
	// SubscribeRole is "agent".  The bus still connects with the MASTER
	// account; this value is forwarded per-subscription down to the
	// upstream SubscribeReq so the gateway knows which agent the master's
	// connection is subscribing on behalf of.  Empty for master/none
	// events.
	AgentOpenID string `json:"agent_open_id,omitempty"`
}

// HelloAck — bus's reply to Hello.
//
// On success: Error=="" and BusVersion is populated.
// On failure: Error∈{WrongOwner, UnknownEventKey, InvalidParams, ...} and the
// bus closes the conn right after.  ExpectedOwnerHash helps consumer print a
// useful diagnostic when WrongOwner triggers.
type HelloAck struct {
	Type              string `json:"type"`
	BusVersion        string `json:"bus_version,omitempty"`
	Error             string `json:"error,omitempty"`
	ExpectedOwnerHash string `json:"expected_owner_hash,omitempty"`
	Detail            string `json:"detail,omitempty"`
}

// Event — business event delivery.  Mirrors the on-wire NDJSON shape seen by
// downstream tooling (jq, --output-dir).  No seq field on purpose — drops are
// signalled out-of-band via control frames, never inferred from gaps.
type Event struct {
	Type    string          `json:"type"`
	Event   string          `json:"event"`
	TraceID string          `json:"trace_id"`
	Payload json.RawMessage `json:"payload"`
}

// Control — out-of-band frame.  Exactly one of the *kind*-specific embedded
// fields is populated based on Kind.
//
// Consumer treatment:
//   - dropped         → stderr WARN, NOT counted by --max-events, NOT piped through --jq
//   - source_status   → stderr informational (suppressible by --quiet)
//   - subscribe_error → stderr WARN (always on); consumer exits 1 with
//     reason="subscribe_failed" (never delivered to stdout / --jq /
//     --output-dir, never counted toward --max-events).
//   - bye             → consumer terminates with reason="shutdown", exit 0
type Control struct {
	Type string `json:"type"`
	Kind string `json:"kind"`

	// kind=dropped — also reused by kind=subscribe_error to identify the
	// EventKey the upstream rejected (so the consumer can match against
	// its own subscription).
	EventKey string `json:"event_key,omitempty"`
	Count    int64  `json:"count,omitempty"`
	SinceTS  int64  `json:"since_ts,omitempty"`

	// kind=source_status — also populated for kind=subscribe_error
	// (Detail carries the gateway's error msg).
	Source string `json:"source,omitempty"`
	State  string `json:"state,omitempty"`
	Detail string `json:"detail,omitempty"`

	// kind=subscribe_error — gateway-supplied SubscribeRsp.code (non-zero).
	// Kept distinct from any future numeric field on dropped/source_status
	// so adding more kinds doesn't force a JSON-shape migration.
	Code uint32 `json:"code,omitempty"`

	// kind=bye
	Reason string `json:"reason,omitempty"`
}

// Bye — symmetric counterpart to Control{Kind:Bye}.  Kept as a separate frame
// type because the *consumer-initiated* graceful close also uses it (consumer
// → bus on stdin EOF / SIGINT).  Bus → consumer prefers Control{Kind:Bye}.
type Bye struct {
	Type   string `json:"type"`
	Reason string `json:"reason,omitempty"`
}

// StatusQuery — `event status` reads a snapshot via this frame.
type StatusQuery struct {
	Type string `json:"type"`
}

// ConsumerInfo — one row of StatusResponse.Consumers.
type ConsumerInfo struct {
	PID      int    `json:"pid"`
	EventKey string `json:"event_key"`
	Received int64  `json:"received"`
	Dropped  int64  `json:"dropped"`
}

// StatusResponse — bus's reply to StatusQuery.
//
// OwnerHash is included so `event status` can render is_active_login by
// comparing against the currently-logged-in OpenId.
type StatusResponse struct {
	Type           string         `json:"type"`
	PID            int            `json:"pid"`
	OwnerHash      string         `json:"owner_hash"`
	StartedAt      string         `json:"started_at"` // RFC3339
	UptimeSec      int            `json:"uptime_sec"`
	ActiveConns    int            `json:"active_conns"`
	SubscribedKeys []string       `json:"subscribed_keys"`
	Consumers      []ConsumerInfo `json:"consumers"`
	WSSState       string         `json:"wss_state,omitempty"`
	WSSConnectedAt string         `json:"wss_connected_at,omitempty"`
	WSSReconnects  int64          `json:"wss_reconnects,omitempty"`
}

// Shutdown — `event stop` writes this frame to ask bus to terminate gracefully.
type Shutdown struct {
	Type  string `json:"type"`
	Force bool   `json:"force,omitempty"`
}

// ---- Constructors -----------------------------------------------------------

func NewHello(pid int, eventKeys []string, params map[string]string, openIDHash, version, traceID, agentOpenID string) *Hello {
	return &Hello{
		Type:       MsgTypeHello,
		PID:        pid,
		EventKeys:  eventKeys,
		Params:     params,
		OpenIDHash: openIDHash,
		Version:    version,
		TraceID:    traceID,
		AgentOpenID: agentOpenID,
	}
}

func NewHelloAckOK(busVersion string) *HelloAck {
	return &HelloAck{Type: MsgTypeHelloAck, BusVersion: busVersion}
}

func NewHelloAckError(code, expectedOwnerHash, detail string) *HelloAck {
	return &HelloAck{
		Type:              MsgTypeHelloAck,
		Error:             code,
		ExpectedOwnerHash: expectedOwnerHash,
		Detail:            detail,
	}
}

func NewEvent(key, traceID string, payload json.RawMessage) *Event {
	return &Event{Type: MsgTypeEvent, Event: key, TraceID: traceID, Payload: payload}
}

func NewControlDropped(eventKey string, count, sinceUnix int64) *Control {
	return &Control{
		Type:     MsgTypeControl,
		Kind:     ControlKindDropped,
		EventKey: eventKey,
		Count:    count,
		SinceTS:  sinceUnix,
	}
}

func NewControlSourceStatus(source, state, detail string) *Control {
	return &Control{
		Type:   MsgTypeControl,
		Kind:   ControlKindSourceStatus,
		Source: source,
		State:  state,
		Detail: detail,
	}
}

// NewControlSubscribeError builds a control frame for an upstream
// WsCLISubscribeEvent rejection.  eventKey identifies the per-consumer
// subscription that failed (so the bus can route this frame only to the
// affected consumers — see Hub.BroadcastSubscribeError); code mirrors
// SubscribeRsp.code from the gateway; msg is SubscribeRsp.msg verbatim,
// safe to surface on stderr (it never carries the access-token).
func NewControlSubscribeError(eventKey string, code uint32, msg string) *Control {
	return &Control{
		Type:     MsgTypeControl,
		Kind:     ControlKindSubscribeError,
		EventKey: eventKey,
		Code:     code,
		Detail:   msg,
	}
}

func NewControlBye(reason string) *Control {
	return &Control{Type: MsgTypeControl, Kind: ControlKindBye, Reason: reason}
}

func NewBye(reason string) *Bye { return &Bye{Type: MsgTypeBye, Reason: reason} }

func NewStatusQuery() *StatusQuery { return &StatusQuery{Type: MsgTypeStatusQuery} }

func NewShutdown(force bool) *Shutdown { return &Shutdown{Type: MsgTypeShutdown, Force: force} }
