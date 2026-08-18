// Package jqfilter — gojq integration for `tmeet event consume --jq <expr>`.
//
// the consume contract mandates that --jq runs entirely inside the consume process
// (L1) and that the bus is unaware of it.  This package therefore exposes a
// tiny surface: compile once at startup, run per-event.
//
// Semantics (see the consume contract for the full table):
//
//   - Input root is determined by KeyDef.JQRootPath: "." passes the entire
//     {event, trace_id, payload} envelope; ".payload" passes only the payload.
//     The rule lives in the registry so per-key conventions (e.g. control-frame
//     keys want the envelope, business keys want the payload) are settable
//     without touching consumer code.
//
//   - Compilation errors are fatal at command startup (exit 1).  We use
//     gojq.Parse + gojq.Compile early so a typo'd expression can't ever reach
//     runConsumeLoop.
//
//   - Per-event evaluation:
//
//   - iterator yields nothing (typically because of `select(...)`)
//     OR yields a single nil      → DROP the event entirely (no
//     stdout, NOT counted by --max-events).
//
//   - iterator yields N≥1 non-nil → emit each value as its own NDJSON
//     line; --max-events counts each
//     emitted line individually (the
//     common case is N=1).
//
//   - iterator yields a runtime
//     error                       → caller logs `[event] WARN jq error`
//     and skips the event; loop continues.
//
//   - We marshal results with encoding/json (compact, no HTML escaping) so
//     the NDJSON line shape matches the un-jq'd default verbatim: no extra
//     spaces, no `&` -> `\u0026` rewrites that would surprise downstream
//     `jq -r`-driven pipelines.
//
// The package deliberately knows NOTHING about cobra, *Cmd, --quiet etc.;
// callers shape the user-visible output.  This keeps the unit tests trivial
// and the runtime path easy to mock.
package jqfilter

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/itchyny/gojq"
)

// Filter is a compiled --jq expression ready for repeated invocation.
//
// Construction is one-shot via Compile; the zero value is NOT usable.  All
// methods are safe for serial use; gojq.Code itself is documented as
// concurrency-safe but tmeet drives a single consume goroutine so we don't
// rely on that property.
type Filter struct {
	expr string     // original source — kept for error messages only.
	code *gojq.Code // compiled gojq program.
}

// Compile parses + compiles expr and returns a ready-to-use *Filter.
//
// Returns nil, nil for an empty expr so callers can use the same code path
// regardless of whether the user passed --jq.  This sentinel-nil pattern is
// documented in ApplyToEvent; callers MUST guard with `if f != nil` before
// invoking it (or rely on EmitFn helpers that do the guard for them).
//
// On parse error we wrap the underlying gojq message with a "jq compile:"
// prefix so the user can grep their stderr for the source of truth.
func Compile(expr string) (*Filter, error) {
	if expr == "" {
		return nil, nil
	}
	q, err := gojq.Parse(expr)
	if err != nil {
		return nil, fmt.Errorf("jq compile: %w", err)
	}
	code, err := gojq.Compile(q)
	if err != nil {
		return nil, fmt.Errorf("jq compile: %w", err)
	}
	return &Filter{expr: expr, code: code}, nil
}

// Expression returns the user-supplied source string.  Used by callers that
// want to embed it in `[event] ready ... jq=...` style stderr lines.
func (f *Filter) Expression() string {
	if f == nil {
		return ""
	}
	return f.expr
}

// Apply runs the filter against the given JSON document (the root chosen by
// the caller per JQRootPath) and returns:
//
//   - results: zero, one or many marshalled NDJSON lines (one element per
//     non-nil iterator value).  Each element is JSON-encoded WITHOUT a
//     trailing newline; callers append the newline when writing.
//   - dropped: true iff the iterator produced no values OR a single nil
//     (the user's filter is a `select(...)` that didn't match).  When
//     dropped is true, results is always empty; the caller should NOT count
//     this event toward --max-events.
//   - err: a runtime gojq error (e.g. type mismatch like `.[0]` on an
//     object).  results may be partial in this case — the convention in
//     the consume contract is that a runtime error voids the whole event, so
//     callers should treat `err != nil` as a "skip + WARN" signal and
//     ignore any partial results.
//
// We deliberately marshal each result here (rather than handing back
// interface{} values) so callers don't accidentally use fmt.Sprintf or %v
// — those produce Go-syntax output that would corrupt the NDJSON contract.
func (f *Filter) Apply(root interface{}) (results [][]byte, dropped bool, err error) {
	if f == nil || f.code == nil {
		return nil, false, errors.New("jqfilter: nil Filter")
	}

	iter := f.code.Run(root)

	for {
		v, more := iter.Next()
		if !more {
			break
		}
		if runErr, isErr := v.(error); isErr {
			// gojq surfaces runtime errors by yielding error values.
			// Wrap with our prefix so logs unambiguously trace back here.
			return nil, false, fmt.Errorf("jq runtime: %w", runErr)
		}
		if v == nil {
			// `select(false)` and friends yield a single nil; we treat that
			// as "no output for this event".  If a user expression yields
			// `null` as a *meaningful* value they should wrap it (e.g.
			// `{x: null}`); raw null is reserved for "drop me".
			continue
		}
		buf, jerr := marshalCompact(v)
		if jerr != nil {
			return nil, false, fmt.Errorf("jq result marshal: %w", jerr)
		}
		results = append(results, buf)
	}

	if len(results) == 0 {
		return nil, true, nil
	}
	return results, false, nil
}

// ApplyToDoc is a convenience that takes a raw JSON document, decodes it
// into a generic interface{}, then calls Apply.  Used by the consume runner
// which has json.RawMessage in hand and would otherwise repeat the decode
// dance for every event.
//
// json.Decoder.UseNumber preserves integer precision for IDs that exceed
// float64's safe integer range — common for meeting_id-style numeric strings
// that legitimately need 15+ digits without lossy conversion.
func (f *Filter) ApplyToDoc(doc []byte) (results [][]byte, dropped bool, err error) {
	var root interface{}
	dec := json.NewDecoder(bytes.NewReader(doc))
	dec.UseNumber()
	if derr := dec.Decode(&root); derr != nil {
		return nil, false, fmt.Errorf("jq input decode: %w", derr)
	}
	return f.Apply(root)
}

// marshalCompact JSON-encodes v with no trailing newline and no HTML
// escaping (so `&`/`<`/`>` survive the round-trip — important when the
// user's filter forwards URLs or HTML snippets).
func marshalCompact(v interface{}) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	// json.Encoder always appends '\n'; strip it so the caller controls
	// line termination (matters because --output-dir wants pretty-printed
	// files without the trailing blank line, while stdout wants exactly
	// one '\n' per record — both code paths need the un-newlined buffer).
	out := buf.Bytes()
	if n := len(out); n > 0 && out[n-1] == '\n' {
		out = out[:n-1]
	}
	return out, nil
}
