// params.go — `--param key=value` validation (L1) and payload matching (L2).
//
// The consume command splits responsibility between two layers:
//
//	L1 (consume process)  — Parse and validate user-supplied --param flags
//	                        against the EventKey's ParamsSchema.  Failures
//	                        are fatal: exit 1, point the user at
//	                        `tmeet event schema <key>`.
//	L2 (bus hub)          — For each delivered RawEvent, drop it for a
//	                        subscriber if any of that subscriber's params
//	                        does not match the value extracted from the
//	                        payload at PayloadPath.
//
// Keeping both halves in one file makes it easy to verify the contracts
// stay aligned (an L1 type-check that's permissive but an L2 path that's
// strict, or vice versa, would be a bug; co-location helps).
//
// Out of scope:
//   - JSON Pointer / JSONPath / gojq for path extraction.  PayloadPath is a
//     dot-separated chain of segments where each segment is either an
//     object key (when the current node is a JSON object) or a non-negative
//     decimal integer index (when the current node is a JSON array).  This
//     is just enough to address Tencent Meeting payloads, which wrap
//     single-event bodies inside a length-1 array (e.g. payload[0].meeting_info.meeting_id).
//   - Cross-param validation (e.g. "userid required when meeting_id absent").
//     Per-param Required+Type+Enum suffices for batch 3.1.

package event

import (
	"encoding/json"
	"sort"
	"strconv"
	"strings"
	"tmeet/internal/exception"
)

// ParseAndValidateParams converts the raw "k=v" slice given by cobra into a
// validated {k: v} map appropriate for the given EventKey.  All errors are
// user-actionable and include the EventKey so messages remain helpful even
// when piped into log aggregators.
//
// Returns nil with no error when raw is empty (the most common case for
// non-filtered consumes); callers can pass the result straight into
// protocol.NewHello which already treats nil Params as "no filter".
//
// Validation order (errors short-circuit on the first failure):
//
//  1. EventKey must exist in the registry (catches typos before we touch
//     the user's args).
//  2. Each entry must parse as "key=value" with non-empty key.
//  3. No duplicate keys (a duplicate is almost always a user copy-paste
//     mistake — silently letting the second value win is unfriendly).
//  4. Every key must be declared in ParamsSchema.
//  5. Per-key type/enum validation.
//  6. Every Required key in ParamsSchema must be present.
func ParseAndValidateParams(eventKey string, raw []string) (map[string]string, error) {
	def, ok := Lookup(eventKey)
	if !ok {
		return nil, exception.InvalidArgsError.With("unknown EventKey %q", eventKey)
	}

	if len(raw) == 0 {
		// Still need to enforce required params even when the user passes
		// none — surface the same error message shape.
		if err := requireAll(def, nil); err != nil {
			return nil, err
		}
		return nil, nil
	}

	out := make(map[string]string, len(raw))
	for _, kv := range raw {
		eq := strings.IndexByte(kv, '=')
		if eq <= 0 {
			// eq==-1 (no '=') OR eq==0 (empty key, e.g. "=foo") — both
			// invalid; combined check keeps the error message terse.
			return nil, exception.InvalidArgsError.With(
				"invalid --param %q: expected key=value with non-empty key", kv)
		}
		key := kv[:eq]
		val := kv[eq+1:]

		if _, dup := out[key]; dup {
			return nil, exception.InvalidArgsError.With("duplicate --param key %q", key)
		}

		spec, declared := def.ParamsSchema[key]
		if !declared {
			return nil, exception.InvalidArgsError.With(
				"unknown --param key %q for EventKey %q; run 'tmeet event schema %s' for valid keys",
				key, eventKey, eventKey)
		}

		if err := validateValue(key, val, spec); err != nil {
			return nil, exception.InvalidArgsError.With("--param %s: %v", key, err)
		}

		out[key] = val
	}

	if err := requireAll(def, out); err != nil {
		return nil, err
	}
	return out, nil
}

// requireAll asserts every Required key in def.ParamsSchema is present in got.
//
// Sorted iteration gives stable error ordering — important for tests and for
// users who run the same command twice and expect the same error.
func requireAll(def *KeyDef, got map[string]string) error {
	if def == nil {
		return nil
	}
	missing := make([]string, 0)
	keys := make([]string, 0, len(def.ParamsSchema))
	for k := range def.ParamsSchema {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		spec := def.ParamsSchema[k]
		if !spec.Required {
			continue
		}
		if _, ok := got[k]; !ok {
			missing = append(missing, k)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	return exception.InvalidArgsError.With("missing required --param: %s (run 'tmeet event schema %s' for details)",
		strings.Join(missing, ", "), def.Key)
}

// validateValue checks val against spec.Type and spec.Enum.
//
// Error messages name the offending value ('%q') so log scrapes can pinpoint
// which input failed without re-running with verbose logging.
func validateValue(key, val string, spec ParamDef) error {
	switch spec.Type {
	case "", "string":
		// Empty Type defaults to string (defensive — every shipped schema
		// declares Type explicitly; this branch keeps tooling forgiving).
	case "integer":
		if _, err := strconv.ParseInt(val, 10, 64); err != nil {
			return exception.InvalidArgsError.With("expected integer, got %q", val)
		}
	case "boolean":
		// strconv.ParseBool accepts {1, t, T, TRUE, true, True, 0, f, F,
		// FALSE, false, False}.  We narrow to the canonical pair to keep
		// the wire format predictable across the bus boundary.
		switch val {
		case "true", "false":
		default:
			return exception.InvalidArgsError.With("expected boolean ('true' or 'false'), got %q", val)
		}
	default:
		// Unknown Type means the schema declaration is wrong — surface as
		// a clear internal error rather than silently accepting anything.
		return exception.EventInternalError.With("internal: unsupported param type %q in schema for key %s",
			spec.Type, key)
	}

	if len(spec.Enum) > 0 {
		for _, allowed := range spec.Enum {
			if val == allowed {
				return nil
			}
		}
		return exception.InvalidArgsError.With("value %q is not in allowed set %v", val, spec.Enum)
	}
	return nil
}

// MatchPayload reports whether the event's payload satisfies every entry in
// params, given the EventKey's ParamsSchema (which dictates where in the
// payload each param's value lives).
//
// Semantics:
//   - Empty params ⇒ always matches (no filter).
//   - For each (k, v) in params:
//     — Look up spec=schema[k]; resolve PayloadPath (default: top-level k).
//     — Walk the payload object keys along the path; if the path is
//     missing or any intermediate node isn't an object, this param
//     does NOT match for this event ⇒ return false.
//     — Stringify the leaf JSON value and compare to v.  Only string,
//     number and boolean leaves are comparable; objects/arrays/null
//     can never match a --param value (since user-supplied --param is
//     always a flat string).
//   - All params must match (AND semantics).
//
// Return false on malformed payload — letting one rogue event through would
// violate the user's filter contract; dropping it is the safer default.
func MatchPayload(schema map[string]ParamDef, params map[string]string, payload json.RawMessage) bool {
	if len(params) == 0 {
		return true
	}
	if len(payload) == 0 {
		return false
	}
	for k, want := range params {
		spec := schema[k]
		path := spec.PayloadPath
		if path == "" {
			path = k
		}
		got, ok := extractScalarString(payload, path)
		if !ok {
			return false
		}
		if got != want {
			return false
		}
	}
	return true
}

// extractScalarString walks payload along a dot-separated path and returns
// the leaf as a string + true on success.  Returns ("", false) for any of:
//
//   - malformed JSON at any level
//   - missing key / out-of-range index at any segment
//   - intermediate node that's neither an object nor an array
//   - leaf node that's an object / array / null (not stringifiable)
//
// Path segments are interpreted positionally based on the current node:
//   - object node -> segment is the field name (any string)
//   - array node  -> segment must be a non-negative decimal integer index
//
// This means a path like "payload.0.meeting_info.meeting_id" is unambiguous:
// once we hit the array at "payload" the next segment "0" is forced to be
// a numeric index even though strings.Split returns it as a string.
//
// Numbers are formatted via strconv.Format* to avoid float-printing
// surprises for integer values that fit in int64.  Booleans become "true" /
// "false".  Strings drop the JSON quoting so the comparison with a
// user-supplied --param value is direct.
func extractScalarString(payload json.RawMessage, path string) (string, bool) {
	segments := strings.Split(path, ".")
	cur := payload
	for i, seg := range segments {
		next, ok := stepInto(cur, seg)
		if !ok {
			return "", false
		}
		if i == len(segments)-1 {
			return decodeLeaf(next)
		}
		cur = next
	}
	return "", false
}

// stepInto descends one level into a JSON node along seg.
//
// Behaviour by node kind (determined from the first non-whitespace byte):
//   - object '{': seg is a field name; missing key => (nil, false).
//   - array '[':  seg must parse as a non-negative int; out-of-range or
//     non-numeric seg => (nil, false).  We deliberately do NOT fall back to
//     a field-name lookup on arrays — that would silently mask schema bugs.
//   - anything else (scalar, malformed): (nil, false).
func stepInto(cur json.RawMessage, seg string) (json.RawMessage, bool) {
	trimmed := skipWS(cur)
	if len(trimmed) == 0 {
		return nil, false
	}
	switch trimmed[0] {
	case '{':
		var m map[string]json.RawMessage
		if err := json.Unmarshal(trimmed, &m); err != nil {
			return nil, false
		}
		next, ok := m[seg]
		if !ok {
			return nil, false
		}
		return next, true
	case '[':
		idx, err := strconv.Atoi(seg)
		if err != nil || idx < 0 {
			return nil, false
		}
		var arr []json.RawMessage
		if err := json.Unmarshal(trimmed, &arr); err != nil {
			return nil, false
		}
		if idx >= len(arr) {
			return nil, false
		}
		return arr[idx], true
	default:
		return nil, false
	}
}

// skipWS returns the slice with leading JSON whitespace trimmed.  json.Unmarshal
// itself tolerates leading whitespace, but we need to peek the first structural
// byte to decide between object/array dispatch in stepInto.
func skipWS(b json.RawMessage) json.RawMessage {
	for i, c := range b {
		switch c {
		case ' ', '\t', '\n', '\r':
			continue
		default:
			return b[i:]
		}
	}
	return b[:0]
}

// decodeLeaf converts a json.RawMessage scalar to its string form.
//
// We accept three JSON types — string, number, boolean — and reject the
// rest because --param values are always strings; comparing against an
// object/array/null would be meaningless.
func decodeLeaf(raw json.RawMessage) (string, bool) {
	if len(raw) == 0 {
		return "", false
	}
	switch raw[0] {
	case '"':
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return "", false
		}
		return s, true
	case 't', 'f':
		var b bool
		if err := json.Unmarshal(raw, &b); err != nil {
			return "", false
		}
		return strconv.FormatBool(b), true
	case 'n':
		// "null" — treat as missing rather than the string "null".
		return "", false
	case '{', '[':
		// Objects / arrays are not scalar; reject.
		return "", false
	default:
		// Number.  json.Number preserves precision for integer-typed IDs
		// (which is the common case for meeting_id).
		var n json.Number
		if err := json.Unmarshal(raw, &n); err != nil {
			return "", false
		}
		return n.String(), true
	}
}
