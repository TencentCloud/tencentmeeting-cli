// Package event — type definitions shared by the registry, the bus daemon and
// the consumer.  Kept dependency-free so that schemas/registry code does not pull
// in protocol/transport packages.
package event

import "encoding/json"

// RawEvent is the canonical wire shape used between source -> bus -> consumer.
// It mirrors the stdout NDJSON contract for `tmeet event consume`:
//
//	{"event": "<key>", "trace_id": "<id>", "payload": {...}}
//
// trace_id is mandatory: it drives both bus-side dedup and consumer-side
// --output-dir filename derivation.
type RawEvent struct {
	Event   string          `json:"event"`
	TraceID string          `json:"trace_id"`
	Payload json.RawMessage `json:"payload"`
}

// ParamDef describes a single user-supplied --param key for an EventKey.
//
// Validation happens entirely on the consume process (L1 of the consume contract):
//   - Unknown keys, missing required keys and type/enum violations all map to exit 1.
//   - Bus only stores params on the per-conn subscription record for L2 fan-out.
//
// For L2 (bus-side filtering) the bus reads the value at PayloadPath inside
// each event's payload and compares it to the user-supplied --param value.
// PayloadPath uses dot-notation; "" means "top-level key with the same name
// as the ParamDef map key".  Each segment is interpreted positionally based
// on the current node:
//   - object node ⇒ segment is a field name
//   - array node  ⇒ segment must be a non-negative decimal integer index
//
// Examples:
//
//	ParamsSchema["meeting_id"] = ParamDef{}                                // payload.meeting_id
//	ParamsSchema["userid"]     = ParamDef{PayloadPath:"user.userid"}       // payload.user.userid
//	ParamsSchema["meeting_id"] = ParamDef{PayloadPath:"0.meeting_info.meeting_id"} // payload[0].meeting_info.meeting_id
//
// The notation is intentionally limited (no escapes, no wildcards / slices):
// the payload schemas tmeet supports today are flat enough that a richer
// expression language would be over-engineering.  If we ever need it we can
// switch this field to a JSONPath / gojq expression without breaking
// existing zero-value declarations.
type ParamDef struct {
	Type        string   `json:"type"` // "string" | "integer" | "boolean"
	Required    bool     `json:"required"`
	Description string   `json:"description,omitempty"`
	Enum        []string `json:"enum,omitempty"`         // optional whitelist; empty = unrestricted
	PayloadPath string   `json:"payload_path,omitempty"` // dot-path inside RawEvent.Payload; "" = top-level same-named key
}

// KeyDef is the immutable, registry-owned descriptor of one EventKey.
//
// `JQRootPath` is either "." or ".payload" and is exposed via
// `event schema` so Agents can compose --jq expressions without trial-and-error.
//
// `BufferSize` is the per-conn bounded chan capacity used by the hub.
// Per-key tunability lets noisy keys (e.g. transcription chunks) declare a
// larger buffer without affecting low-volume keys.
type KeyDef struct {
	Key                  string              `json:"key"`
	Domain               string              `json:"domain"`
	Description          string              `json:"description"`
	JQRootPath           string              `json:"jq_root_path"`
	ParamsSchema         map[string]ParamDef `json:"params_schema,omitempty"`
	ResolvedOutputSchema json.RawMessage     `json:"resolved_output_schema,omitempty"`

	// SubscribeRole declares which account identity is allowed to
	// subscribe to this EventKey:
	//
	//   - SubscribeRoleMaster — only the main account (AppMeta.ActiveOpenId)
	//     may subscribe.  This is the default for the meeting.* events.
	//   - SubscribeRoleAgent  — only a configured agent (子账号) may
	//     subscribe.  `event consume` validates an agent exists BEFORE
	//     forking the bus, and carries agent_open_id all the way down to
	//     the upstream SubscribeReq.
	//   - SubscribeRoleNone   — no identity restriction.
	//
	// Empty defaults to SubscribeRoleNone at registration time.
	SubscribeRole string `json:"subscribe_role"`

	// BufferSize: per-conn bounded chan capacity, defaults to defaultBufferSize
	// if zero.  Capped by maxBufferSize on registration to keep memory bounded.
	BufferSize int `json:"-"`
}

// SubscribeRole* enumerate the account identities that may subscribe to an
// EventKey.  Stored as plain strings on KeyDef so `event list` / `event
// schema` can surface them verbatim without an enum-to-string mapping.
const (
	SubscribeRoleMaster = "master" // only the main account may subscribe
	SubscribeRoleAgent  = "agent"  // only a configured agent (子账号) may subscribe
	SubscribeRoleNone   = "none"   // no identity restriction
)

// Buffer-size policy: hub drops oldest on overflow.  Keeping these here
// (rather than in hub) lets the registry self-validate at init time.
const (
	defaultBufferSize = 100
	maxBufferSize     = 1000
)

// EventListItem is the trimmed-down shape returned by `event list`.
// Defined here (not in cmd/event) so future API consumers can import it without
// pulling in cobra.
type EventListItem struct {
	Key           string `json:"key"`
	Domain        string `json:"domain"`
	Description   string `json:"description"`
	SubscribeRole string `json:"subscribe_role"`
}
