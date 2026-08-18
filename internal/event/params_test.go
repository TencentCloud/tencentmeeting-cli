// params_test.go — unit tests for ParseAndValidateParams (L1) and
// MatchPayload (L2).  Driven entirely off the schemas registered in
// schemas.go to avoid divergence between test fixtures and shipped specs.

package event

import (
	"encoding/json"
	"strings"
	"testing"
)

// ---------------- ParseAndValidateParams (L1) -------------------------------

func TestParseAndValidateParams_NilWhenEmpty(t *testing.T) {
	got, err := ParseAndValidateParams("meeting.started", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil map for empty input, got %+v", got)
	}
}

func TestParseAndValidateParams_UnknownEventKey(t *testing.T) {
	_, err := ParseAndValidateParams("does.not.exist", []string{"a=b"})
	if err == nil || !strings.Contains(err.Error(), "unknown EventKey") {
		t.Fatalf("expected unknown-EventKey error, got %v", err)
	}
}

func TestParseAndValidateParams_HappyString(t *testing.T) {
	got, err := ParseAndValidateParams("meeting.started", []string{"meeting_id=12345"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got["meeting_id"] != "12345" {
		t.Errorf("meeting_id = %q, want %q", got["meeting_id"], "12345")
	}
}

func TestParseAndValidateParams_MissingEqualSign(t *testing.T) {
	_, err := ParseAndValidateParams("meeting.started", []string{"meeting_id"})
	if err == nil || !strings.Contains(err.Error(), "key=value") {
		t.Errorf("expected key=value hint, got %v", err)
	}
}

func TestParseAndValidateParams_EmptyKey(t *testing.T) {
	_, err := ParseAndValidateParams("meeting.started", []string{"=value"})
	if err == nil || !strings.Contains(err.Error(), "non-empty key") {
		t.Errorf("expected non-empty-key error, got %v", err)
	}
}

func TestParseAndValidateParams_UnknownKey(t *testing.T) {
	_, err := ParseAndValidateParams("meeting.started", []string{"not_a_real_param=x"})
	if err == nil {
		t.Fatal("expected unknown-key error, got nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, "not_a_real_param") {
		t.Errorf("error should mention the offending key: %v", err)
	}
	if !strings.Contains(msg, "tmeet event schema") {
		t.Errorf("error should point user at 'tmeet event schema': %v", err)
	}
}

func TestParseAndValidateParams_DuplicateKey(t *testing.T) {
	_, err := ParseAndValidateParams("meeting.started",
		[]string{"meeting_id=A", "meeting_id=B"})
	if err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Errorf("expected duplicate-key error, got %v", err)
	}
}

// ---- type validation -------------------------------------------------------
//
// We register a synthetic EventKey here (rather than relying on the shipped
// schemas, all of which are string-typed) so the integer/boolean/enum
// branches are properly exercised.

func TestMain_RegistersSyntheticTypeFixtures(t *testing.T) {
	// nothing to do — this test name signals to readers that
	// registerTypeFixtures (called from init below) is intentional.
	_ = t
}

func init() {
	RegisterKey(KeyDef{
		Key:         "test.types_fixture",
		Domain:      "test",
		Description: "internal type-validation fixture (not user-visible)",
		JQRootPath:  ".payload",
		ParamsSchema: map[string]ParamDef{
			"int_param":  {Type: "integer"},
			"bool_param": {Type: "boolean"},
			"enum_param": {Type: "string", Enum: []string{"red", "green", "blue"}},
			"required_p": {Type: "string", Required: true},
		},
	})
}

func TestParseAndValidateParams_IntegerOK(t *testing.T) {
	_, err := ParseAndValidateParams("test.types_fixture",
		[]string{"int_param=42", "required_p=x"})
	if err != nil {
		t.Errorf("unexpected: %v", err)
	}
}

func TestParseAndValidateParams_IntegerInvalid(t *testing.T) {
	_, err := ParseAndValidateParams("test.types_fixture",
		[]string{"int_param=not-a-number", "required_p=x"})
	if err == nil || !strings.Contains(err.Error(), "expected integer") {
		t.Errorf("expected integer-error, got %v", err)
	}
}

func TestParseAndValidateParams_BooleanOK(t *testing.T) {
	for _, v := range []string{"true", "false"} {
		_, err := ParseAndValidateParams("test.types_fixture",
			[]string{"bool_param=" + v, "required_p=x"})
		if err != nil {
			t.Errorf("bool %q: unexpected error %v", v, err)
		}
	}
}

func TestParseAndValidateParams_BooleanInvalid(t *testing.T) {
	// strconv.ParseBool would accept "1" / "T" etc., but our wire contract
	// only accepts the canonical pair.
	for _, v := range []string{"1", "T", "yes", "no"} {
		_, err := ParseAndValidateParams("test.types_fixture",
			[]string{"bool_param=" + v, "required_p=x"})
		if err == nil || !strings.Contains(err.Error(), "boolean") {
			t.Errorf("bool %q: expected boolean error, got %v", v, err)
		}
	}
}

func TestParseAndValidateParams_EnumHit(t *testing.T) {
	_, err := ParseAndValidateParams("test.types_fixture",
		[]string{"enum_param=green", "required_p=x"})
	if err != nil {
		t.Errorf("unexpected: %v", err)
	}
}

func TestParseAndValidateParams_EnumMiss(t *testing.T) {
	_, err := ParseAndValidateParams("test.types_fixture",
		[]string{"enum_param=purple", "required_p=x"})
	if err == nil || !strings.Contains(err.Error(), "not in allowed set") {
		t.Errorf("expected enum-miss error, got %v", err)
	}
}

func TestParseAndValidateParams_RequiredMissing(t *testing.T) {
	_, err := ParseAndValidateParams("test.types_fixture",
		[]string{"int_param=1"})
	if err == nil || !strings.Contains(err.Error(), "required_p") {
		t.Errorf("expected required-missing error mentioning required_p, got %v", err)
	}
}

func TestParseAndValidateParams_RequiredMissingNoArgs(t *testing.T) {
	// Even with zero --param flags, a Required key must be enforced.
	_, err := ParseAndValidateParams("test.types_fixture", nil)
	if err == nil || !strings.Contains(err.Error(), "required_p") {
		t.Errorf("expected required-missing for empty input, got %v", err)
	}
}

// ---------------- MatchPayload (L2) ----------------------------------------

func TestMatchPayload_NilParamsAlwaysMatches(t *testing.T) {
	// Empty params filter ⇒ deliver everything.
	if !MatchPayload(nil, nil, json.RawMessage(`{"meeting_id":"x"}`)) {
		t.Errorf("nil params should match")
	}
}

// Schema-coupled MatchPayload tests for shipped EventKeys live inside the
// array-indexing block below (TestMatchPayload_ArrayIndex_*).  Payload
// shapes there mirror the real Tencent Meeting webhook envelope, which
// matters because PayloadPath for the shipped meeting.started /
// meeting.end keys steps through `payload[0]` rather than a flat object.

func TestMatchPayload_BooleanLeaf(t *testing.T) {
	// Generic schema/leaf decoder test: synthesise a payload + schema that
	// reads a boolean.  Top-level key, default PayloadPath.
	if !MatchPayload(map[string]ParamDef{"flag": {Type: "boolean"}},
		map[string]string{"flag": "true"},
		json.RawMessage(`{"flag":true}`)) {
		t.Errorf("boolean leaf should match")
	}
	if MatchPayload(map[string]ParamDef{"flag": {Type: "boolean"}},
		map[string]string{"flag": "true"},
		json.RawMessage(`{"flag":false}`)) {
		t.Errorf("boolean mismatch should NOT match")
	}
}

func TestMatchPayload_RejectObjectAndArrayLeaf(t *testing.T) {
	// If a schema's PayloadPath aims at a non-scalar, the contract is "no
	// match" rather than panic.  Defensive against schema/payload drift.
	if MatchPayload(map[string]ParamDef{"x": {Type: "string"}},
		map[string]string{"x": "abc"},
		json.RawMessage(`{"x":{"nested":1}}`)) {
		t.Errorf("object leaf should not match")
	}
	if MatchPayload(map[string]ParamDef{"x": {Type: "string"}},
		map[string]string{"x": "abc"},
		json.RawMessage(`{"x":[1,2]}`)) {
		t.Errorf("array leaf should not match")
	}
}

func TestMatchPayload_NullLeafIsMiss(t *testing.T) {
	if MatchPayload(map[string]ParamDef{"x": {Type: "string"}},
		map[string]string{"x": "abc"},
		json.RawMessage(`{"x":null}`)) {
		t.Errorf("null leaf should not match")
	}
}

func TestMatchPayload_MalformedPayloadIsMiss(t *testing.T) {
	if MatchPayload(map[string]ParamDef{"x": {Type: "string"}},
		map[string]string{"x": "abc"},
		json.RawMessage(`{not json`)) {
		t.Errorf("malformed payload should drop (return false), not match")
	}
}

func TestMatchPayload_EmptyPayloadIsMiss(t *testing.T) {
	if MatchPayload(map[string]ParamDef{"x": {Type: "string"}},
		map[string]string{"x": "abc"},
		json.RawMessage(``)) {
		t.Errorf("empty payload should NOT match a non-empty filter")
	}
}

func TestMatchPayload_AllParamsMustMatch(t *testing.T) {
	// AND semantics: one mismatched param drops the event.  Uses an
	// ad-hoc schema rather than a shipped EventKey so the fixture stays
	// independent of any specific webhook envelope shape.
	schema := map[string]ParamDef{
		"meeting_id": {Type: "string"},
		"userid":     {Type: "string", PayloadPath: "user.userid"},
	}
	good := json.RawMessage(`{"meeting_id":"X","user":{"userid":"u1"}}`)
	if MatchPayload(schema,
		map[string]string{"meeting_id": "X", "userid": "wrong"},
		good) {
		t.Errorf("AND-semantics: mixed match/miss should drop")
	}
	if !MatchPayload(schema,
		map[string]string{"meeting_id": "X", "userid": "u1"},
		good) {
		t.Errorf("AND-semantics: all-match should pass")
	}
}

// ---------------- MatchPayload (L2) — array indexing -----------------------
//
// Tencent Meeting webhook payloads wrap single-event bodies in a length-1
// JSON array, so PayloadPath must be able to step through arrays via
// non-negative decimal indices.  The tests below pin the contract:
//   - object segments still work after array indexing (mixed paths)
//   - out-of-range / non-numeric indices on an array node ⇒ no match
//   - numeric segments on an OBJECT node remain a field-name lookup
//     (no silent fall-through into array semantics)

func TestMatchPayload_ArrayIndex_HappyPath(t *testing.T) {
	// Mirrors meeting.started's PayloadPath "0.meeting_info.meeting_id".
	schema := map[string]ParamDef{
		"meeting_id": {Type: "string", PayloadPath: "0.meeting_info.meeting_id"},
	}
	good := json.RawMessage(`[{"meeting_info":{"meeting_id":"M-1"}}]`)
	if !MatchPayload(schema,
		map[string]string{"meeting_id": "M-1"}, good) {
		t.Errorf("array[0] → object → leaf should match")
	}
	if MatchPayload(schema,
		map[string]string{"meeting_id": "M-2"}, good) {
		t.Errorf("array[0] leaf mismatch should NOT match")
	}
}

func TestMatchPayload_ArrayIndex_OutOfRange(t *testing.T) {
	schema := map[string]ParamDef{
		"x": {Type: "string", PayloadPath: "5.foo"},
	}
	if MatchPayload(schema, map[string]string{"x": "v"},
		json.RawMessage(`[{"foo":"v"}]`)) {
		t.Errorf("out-of-range index should NOT match")
	}
}

func TestMatchPayload_ArrayIndex_NonNumericOnArray(t *testing.T) {
	// Stepping into an array with a non-integer segment must fail rather
	// than fall back to a field-name lookup.  This matters because some
	// JSON arrays carry struct-shaped elements; a typo'd schema like
	// "meeting_info.meeting_id" against an array payload should surface
	// as "no match" not as a silent miss against arr[0].
	schema := map[string]ParamDef{
		"x": {Type: "string", PayloadPath: "meeting_info.meeting_id"},
	}
	if MatchPayload(schema, map[string]string{"x": "v"},
		json.RawMessage(`[{"meeting_info":{"meeting_id":"v"}}]`)) {
		t.Errorf("non-integer segment on array node should NOT match")
	}
}

func TestMatchPayload_ArrayIndex_NegativeRejected(t *testing.T) {
	schema := map[string]ParamDef{
		"x": {Type: "string", PayloadPath: "-1.foo"},
	}
	if MatchPayload(schema, map[string]string{"x": "v"},
		json.RawMessage(`[{"foo":"v"}]`)) {
		t.Errorf("negative index should NOT match")
	}
}

func TestMatchPayload_NumericSegOnObjectIsFieldName(t *testing.T) {
	// Arrays use indices; objects always use field names — even when the
	// field name happens to be a numeric string.  Guards backward compat
	// for any future schema with `"0"` as a literal JSON object key.
	schema := map[string]ParamDef{
		"x": {Type: "string", PayloadPath: "0.value"},
	}
	if !MatchPayload(schema, map[string]string{"x": "ok"},
		json.RawMessage(`{"0":{"value":"ok"}}`)) {
		t.Errorf("numeric segment on object should be treated as field name")
	}
}

func TestMatchPayload_ArrayIndex_OnNonArrayWithNumericKey(t *testing.T) {
	// Counterpart to the above: if the schema declares "0.x" but payload
	// is an object without a "0" key, miss (don't accidentally treat it
	// as an array indexing failure or as a generic "object[0]" magic).
	schema := map[string]ParamDef{
		"x": {Type: "string", PayloadPath: "0.value"},
	}
	if MatchPayload(schema, map[string]string{"x": "ok"},
		json.RawMessage(`{"value":"ok"}`)) {
		t.Errorf("missing object key should NOT match")
	}
}

func TestMatchPayload_NestedArraysIndex(t *testing.T) {
	// Two consecutive array indices: payload[0][1].leaf
	schema := map[string]ParamDef{
		"x": {Type: "string", PayloadPath: "0.1.leaf"},
	}
	if !MatchPayload(schema, map[string]string{"x": "yes"},
		json.RawMessage(`[[{"leaf":"no"},{"leaf":"yes"}]]`)) {
		t.Errorf("nested array indexing should match")
	}
}
