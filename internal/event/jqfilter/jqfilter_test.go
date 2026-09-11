// jqfilter_test.go — unit coverage for the gojq integration.
//
// These tests deliberately avoid touching cobra / consume / bus; the goal is
// to pin the contract between Compile/Apply and the consume runner so that
// the runner's tests can focus on lifecycle (--quiet, --max-events, etc.)
// rather than gojq semantics.

package jqfilter

import (
	"strings"
	"testing"
)

func TestCompile_EmptyExprReturnsNilFilter(t *testing.T) {
	// The runner depends on this sentinel so it can use one Compile-call
	// regardless of whether --jq was passed.
	f, err := Compile("")
	if err != nil {
		t.Fatalf("Compile(\"\"): %v", err)
	}
	if f != nil {
		t.Errorf("Compile(\"\") should return nil filter, got %+v", f)
	}
}

func TestCompile_SyntaxErrorPrefix(t *testing.T) {
	_, err := Compile(".foo |") // dangling pipe
	if err == nil {
		t.Fatal("expected compile error")
	}
	if !strings.HasPrefix(err.Error(), "jq compile:") {
		t.Errorf("error should start with 'jq compile:', got %q", err.Error())
	}
}

func TestCompile_ValidExprStoresExpression(t *testing.T) {
	f, err := Compile(".x")
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if f == nil {
		t.Fatal("Compile returned nil for valid expr")
	}
	if got := f.Expression(); got != ".x" {
		t.Errorf("Expression() = %q, want %q", got, ".x")
	}
}

func TestApply_Identity(t *testing.T) {
	f, _ := Compile(".")
	results, dropped, err := f.ApplyToDoc([]byte(`{"a":1,"b":"x"}`))
	if err != nil {
		t.Fatalf("ApplyToDoc: %v", err)
	}
	if dropped {
		t.Error("identity filter must not drop")
	}
	if len(results) != 1 {
		t.Fatalf("identity should yield 1 result, got %d", len(results))
	}
	// Use a tolerant match because gojq normalises map ordering.
	got := string(results[0])
	if !strings.Contains(got, `"a":1`) || !strings.Contains(got, `"b":"x"`) {
		t.Errorf("identity output missing fields: %q", got)
	}
}

func TestApply_SelectTrueKeeps(t *testing.T) {
	f, _ := Compile(`select(.role == "host")`)
	results, dropped, err := f.ApplyToDoc([]byte(`{"role":"host","uid":1}`))
	if err != nil {
		t.Fatalf("ApplyToDoc: %v", err)
	}
	if dropped {
		t.Error("select-true should not drop")
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
}

func TestApply_SelectFalseDrops(t *testing.T) {
	f, _ := Compile(`select(.role == "host")`)
	results, dropped, err := f.ApplyToDoc([]byte(`{"role":"attendee"}`))
	if err != nil {
		t.Fatalf("ApplyToDoc: %v", err)
	}
	if !dropped {
		t.Error("select-false MUST drop")
	}
	if len(results) != 0 {
		t.Errorf("dropped event must yield zero results, got %d", len(results))
	}
}

func TestApply_NullSingleValueDrops(t *testing.T) {
	// `.missing` on an object without that key yields a single null,
	// which we treat as "drop me" per the consume contract.
	f, _ := Compile(".missing")
	_, dropped, err := f.ApplyToDoc([]byte(`{"present":1}`))
	if err != nil {
		t.Fatalf("ApplyToDoc: %v", err)
	}
	if !dropped {
		t.Error("single-null result must map to dropped=true")
	}
}

func TestApply_ProjectionReshape(t *testing.T) {
	f, _ := Compile(`{u: .user.userid, t: .timestamp}`)
	results, dropped, err := f.ApplyToDoc(
		[]byte(`{"meeting_id":"m1","timestamp":1717900000,"user":{"userid":"u_001","role":"host"}}`))
	if err != nil {
		t.Fatalf("ApplyToDoc: %v", err)
	}
	if dropped {
		t.Error("projection should yield a value")
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	got := string(results[0])
	// Field order is gojq-determined; assert presence + values rather
	// than the byte-for-byte string.
	for _, want := range []string{`"u":"u_001"`, `"t":1717900000`} {
		if !strings.Contains(got, want) {
			t.Errorf("result missing %q: %s", want, got)
		}
	}
	if strings.Contains(got, `"meeting_id"`) {
		t.Errorf("projection should not leak un-projected fields: %s", got)
	}
}

func TestApply_GeneratorYieldsMultiple(t *testing.T) {
	// `.users[]` enumerates an array; expect 3 results.
	f, _ := Compile(".users[]")
	results, dropped, err := f.ApplyToDoc([]byte(`{"users":[1,2,3]}`))
	if err != nil {
		t.Fatalf("ApplyToDoc: %v", err)
	}
	if dropped {
		t.Error("generator with values must not be dropped")
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	for i, want := range []string{"1", "2", "3"} {
		if string(results[i]) != want {
			t.Errorf("results[%d] = %q, want %q", i, results[i], want)
		}
	}
}

func TestApply_RuntimeError(t *testing.T) {
	// `.[0]` on an object is a type error in gojq.
	f, _ := Compile(".[0]")
	_, _, err := f.ApplyToDoc([]byte(`{"x":1}`))
	if err == nil {
		t.Fatal("expected runtime error")
	}
	if !strings.HasPrefix(err.Error(), "jq runtime:") {
		t.Errorf("runtime error should be prefixed: %v", err)
	}
}

func TestApply_PreservesNumericPrecisionViaUseNumber(t *testing.T) {
	// 9007199254740993 is 2^53 + 1 — outside float64's safe integer
	// range.  gojq with json.Number input must preserve it exactly.
	f, _ := Compile(".id")
	results, _, err := f.ApplyToDoc([]byte(`{"id":9007199254740993}`))
	if err != nil {
		t.Fatalf("ApplyToDoc: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if string(results[0]) != "9007199254740993" {
		t.Errorf("precision lost: %q", results[0])
	}
}

func TestApply_StringLeafIsQuotedJSON(t *testing.T) {
	// Standalone scalar strings must come out as JSON-quoted literals so
	// downstream NDJSON readers can parse them.
	f, _ := Compile(".name")
	results, _, err := f.ApplyToDoc([]byte(`{"name":"alice"}`))
	if err != nil {
		t.Fatalf("ApplyToDoc: %v", err)
	}
	if len(results) != 1 || string(results[0]) != `"alice"` {
		t.Errorf("expected quoted JSON string, got %v", results)
	}
}

func TestApply_HTMLCharsNotEscaped(t *testing.T) {
	// We deliberately disable HTML escaping so URLs and angle brackets
	// survive the round-trip.  Without SetEscapeHTML(false) "&" becomes
	// "\u0026" which surprises users piping into `jq -r`.
	f, _ := Compile(".url")
	results, _, _ := f.ApplyToDoc([]byte(`{"url":"https://example.com/?a=1&b=2"}`))
	if len(results) != 1 {
		t.Fatalf("expected 1 result")
	}
	got := string(results[0])
	if strings.Contains(got, `\u0026`) {
		t.Errorf("HTML escape leaked: %q", got)
	}
	if !strings.Contains(got, "&b=2") {
		t.Errorf("'&' should round-trip verbatim: %q", got)
	}
}

func TestApplyToDoc_MalformedJSON(t *testing.T) {
	f, _ := Compile(".")
	_, _, err := f.ApplyToDoc([]byte(`{not json`))
	if err == nil {
		t.Fatal("malformed JSON must produce an error")
	}
	if !strings.HasPrefix(err.Error(), "jq input decode:") {
		t.Errorf("decode error should be prefixed: %v", err)
	}
}

func TestApply_OnNilFilterReturnsError(t *testing.T) {
	var f *Filter // nil
	_, _, err := f.Apply(map[string]interface{}{"x": 1})
	if err == nil {
		t.Fatal("nil filter must return an error, not panic")
	}
}
