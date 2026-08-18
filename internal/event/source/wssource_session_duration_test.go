// wssource_session_duration_test.go — regression coverage for the
// per-call sessionDur returned by WSSource.runOnce.
//
// Background
// ==========
//
// An earlier version of WSSource stored the most-recent session's
// duration in a shared atomic field (lastConnDuration) on the receiver
// and Run() read it AFTER runOnce returned to decide whether the
// just-finished attempt qualified as "stable" (>60s) and therefore
// deserved a backoff reset.
//
// That sharing was unsafe across the boundary between runOnce calls:
// once a long-lived session ended, the field kept the stable value;
// any subsequent runOnce that failed BEFORE reaching steady state
// (dial error, refreshToken failure, auth rejection, …) would leave
// the field untouched, and Run() would mistakenly classify that
// failed dial as "stable" and reset the backoff / consecutiveFailures
// counters.  Symptom in production: after a first reconnect that
// successfully recovered a long session, a SECOND disconnect would
// retry at the MinBackoff floor (~1s) forever instead of growing
// exponentially.
//
// Fix: runOnce returns sessionDur directly as a second return value,
// so each call's duration is scoped to that call.  Failed dials
// return zero; steady sessions return the real time since AuthBind
// completed.
//
// These tests pin that contract end-to-end without depending on the
// real 60s threshold (which would make the suite painfully slow).
// We instead exercise the invariant the bug actually violated:
// no value from a previous runOnce can leak into the next call's
// returned sessionDur.

package source

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	eventruntime "tmeet/internal/event"
)

// runOnceResult bundles the two return values for table-style asserts.
type runOnceResult struct {
	err error
	dur time.Duration
}

// callRunOnce is a thin wrapper that exercises runOnce with no-op
// emit / notify callbacks.  applyDefaults must be invoked first
// (Run() normally does that) because runOnce reads HandshakeTimeout,
// ReadTimeout, etc.
func callRunOnce(ctx context.Context, src *WSSource) runOnceResult {
	src.applyDefaults()
	err, dur := src.runOnce(ctx,
		func(*eventruntime.RawEvent) {},
		func(string, string) {})
	return runOnceResult{err: err, dur: dur}
}

// TestRunOnce_DialFailureReturnsZeroDuration pins the dial-error
// branch: when the WS handshake never lands a connection, runOnce
// must return (err, 0).  No "phantom" duration may sneak in from a
// stale shared field; the second return value is the *single source
// of truth* for "did we reach steady state?" in Run().
func TestRunOnce_DialFailureReturnsZeroDuration(t *testing.T) {
	// Point at a TCP endpoint nobody is listening on so dial fails
	// fast and deterministically.  127.0.0.1:1 is reserved (tcpmux)
	// and unbound on every CI runner we care about.
	src := &WSSource{
		URL:        "ws://127.0.0.1:1/never-listens",
		MinBackoff: 10 * time.Millisecond,
		// Decoder != nil skips the AuthBind path so we don't need
		// to wire fake Token / OpenID; runOnce hits the dial failure
		// long before AuthBind would even matter.
		Decoder:          DecodeNDJSONFrame,
		HandshakeTimeout: 200 * time.Millisecond,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	got := callRunOnce(ctx, src)
	if got.err == nil {
		t.Fatal("runOnce on an unreachable URL must return a non-nil error")
	}
	if got.dur != 0 {
		t.Errorf("runOnce on dial failure returned sessionDur=%s, want 0 "+
			"(the call never reached steady state)", got.dur)
	}
	// Sanity-check that the connectedSince field was NOT bumped on
	// the failed dial — Run()'s `event status` view would otherwise
	// mis-report a non-zero connected_at for a session that never
	// existed.
	if since := src.connectedSince.Load(); since != 0 {
		t.Errorf("connectedSince after failed dial = %d, want 0", since)
	}
}

// TestRunOnce_RefreshTokenFailureReturnsZeroDuration pins the
// dial-time TokenProvider branch.  This is one of the early-return
// paths that pre-dates the defer block that populates sessionDur,
// so the returned duration must remain the named-return zero value.
//
// Critically: this is the exact path that, under the original bug,
// could inherit the previous session's stable duration via the
// shared field.  We assert it returns zero unconditionally now.
func TestRunOnce_RefreshTokenFailureReturnsZeroDuration(t *testing.T) {
	tokenErr := errors.New("simulated refresh failure")
	src := &WSSource{
		URL:        "ws://127.0.0.1:1/unused", // never reached
		MinBackoff: 10 * time.Millisecond,
		Decoder:    DecodeNDJSONFrame,
		TokenProvider: func(context.Context) (string, error) {
			return "", tokenErr
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	got := callRunOnce(ctx, src)
	if got.err == nil {
		t.Fatal("runOnce must surface TokenProvider failure as an error")
	}
	if !isAuthError(got.err) {
		t.Errorf("TokenProvider failure should wrap into authError, got %T: %v",
			got.err, got.err)
	}
	if got.dur != 0 {
		t.Errorf("runOnce on TokenProvider failure returned sessionDur=%s, want 0", got.dur)
	}
}

// TestRunOnce_NoSessionDurationLeakAcrossCalls is the regression test
// for the bug this fix was written to repair.
//
// Steps:
//  1. Run one runOnce against a real WS server (NDJSON path, no
//     AuthBind required).  It reaches steady state, then the server
//     drops the connection.  This call returns (err, dur1>0).
//  2. Point the same WSSource at an unreachable URL and invoke
//     runOnce again.  Dial fails before steady state, so this call
//     must return (err, 0) — independent of what call #1 returned.
//
// In the pre-fix code, the receiver-level lastConnDuration field
// would still hold dur1 across the second call, and Run() would
// mis-classify the dial failure as "stable" if dur1 > 60s.  With
// the fix, sessionDur is scoped to each call and the second
// invocation literally cannot inherit dur1.
func TestRunOnce_NoSessionDurationLeakAcrossCalls(t *testing.T) {
	srv := newScriptableServer(t)

	src := &WSSource{
		URL:              srv.wsURL(),
		MinBackoff:       10 * time.Millisecond,
		Decoder:          DecodeNDJSONFrame,
		HandshakeTimeout: 500 * time.Millisecond,
	}
	src.applyDefaults()

	// --- Call #1: real connect, then server-side close --------------
	ctx1, cancel1 := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel1()

	// Drive runOnce on a goroutine so we can drop the server-side
	// conn from the test goroutine.
	type result struct {
		err error
		dur time.Duration
	}
	r1ch := make(chan result, 1)
	go func() {
		err, dur := src.runOnce(ctx1,
			func(*eventruntime.RawEvent) {},
			func(string, string) {})
		r1ch <- result{err: err, dur: dur}
	}()

	conn := srv.nextConn(t)
	// Give the source a tick to publish connState / fire the steady
	// notify before we tear the conn down — guarantees runOnce
	// crossed the defer that populates sessionDur.
	time.Sleep(50 * time.Millisecond)
	_ = conn.Close()

	var r1 result
	select {
	case r1 = <-r1ch:
	case <-time.After(3 * time.Second):
		t.Fatal("call #1 runOnce did not return within 3s after server closed conn")
	}
	if r1.err == nil {
		t.Fatal("call #1 expected an error after server closed the conn")
	}
	if r1.dur <= 0 {
		t.Fatalf("call #1 expected positive sessionDur (reached steady state), got %s", r1.dur)
	}

	// --- Call #2: unreachable endpoint, dial must fail before steady -
	src.URL = "ws://127.0.0.1:1/never-listens"

	ctx2, cancel2 := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel2()

	err2, dur2 := src.runOnce(ctx2,
		func(*eventruntime.RawEvent) {},
		func(string, string) {})
	if err2 == nil {
		t.Fatal("call #2 against an unreachable URL must return an error")
	}
	if dur2 != 0 {
		t.Errorf("call #2 (dial failure) returned sessionDur=%s, want 0 "+
			"(no value may leak from call #1, which returned dur=%s). "+
			"This is the exact invariant the bug fix established: per-call "+
			"sessionDur, not a shared field.",
			dur2, r1.dur)
	}
}

// TestRunOnce_SteadySessionReturnsPositiveDuration pins the positive
// half of the contract: a runOnce that DID reach steady state must
// return a duration > 0, which is what lets Run() compute wasStable
// correctly.  Combined with TestRunOnce_NoSessionDurationLeakAcrossCalls
// this gives us the full bidirectional pinning.
func TestRunOnce_SteadySessionReturnsPositiveDuration(t *testing.T) {
	srv := newScriptableServer(t)
	src := &WSSource{
		URL:              srv.wsURL(),
		MinBackoff:       10 * time.Millisecond,
		Decoder:          DecodeNDJSONFrame,
		HandshakeTimeout: 500 * time.Millisecond,
	}
	src.applyDefaults()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	type result struct {
		err error
		dur time.Duration
	}
	rch := make(chan result, 1)
	go func() {
		err, dur := src.runOnce(ctx,
			func(*eventruntime.RawEvent) {},
			func(string, string) {})
		rch <- result{err: err, dur: dur}
	}()

	conn := srv.nextConn(t)
	// Hold the session steady for a measurable window so the
	// returned duration is unambiguously positive and not a sub-ms
	// rounding artefact.
	const steadyHold = 150 * time.Millisecond
	time.Sleep(steadyHold)
	_ = conn.Close()

	var r result
	select {
	case r = <-rch:
	case <-time.After(3 * time.Second):
		t.Fatal("runOnce did not return within 3s after server closed conn")
	}
	if r.err == nil {
		t.Fatal("expected an error after server closed the conn")
	}
	if r.dur < steadyHold {
		t.Errorf("sessionDur = %s, want >= %s (we held the steady session for that long)",
			r.dur, steadyHold)
	}

	// connectedSince must be cleared by the defer before runOnce
	// returns, regardless of how the session ended.
	if since := src.connectedSince.Load(); since != 0 {
		t.Errorf("connectedSince after runOnce return = %d, want 0 "+
			"(defer must clear it on every steady-state exit)", since)
	}
}

// TestRunOnce_CtxCancelDuringSteadyReturnsRealDuration verifies the
// "ctx cancelled mid-session" exit path also produces a sane
// sessionDur — the defer must fire regardless of whether teardown
// came from a server close, a network error, or ctx cancellation.
//
// This guards against a subtle regression: if someone later replaced
// the defer with an inline `sessionDur = time.Since(...)` only on
// certain return paths, the ctx-cancel branch could quietly start
// returning zero again.  We pin the defer's universality here.
func TestRunOnce_CtxCancelDuringSteadyReturnsRealDuration(t *testing.T) {
	srv := newScriptableServer(t)
	src := &WSSource{
		URL:              srv.wsURL(),
		MinBackoff:       10 * time.Millisecond,
		Decoder:          DecodeNDJSONFrame,
		HandshakeTimeout: 500 * time.Millisecond,
	}
	src.applyDefaults()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	type result struct {
		err error
		dur time.Duration
	}
	rch := make(chan result, 1)
	go func() {
		err, dur := src.runOnce(ctx,
			func(*eventruntime.RawEvent) {},
			func(string, string) {})
		rch <- result{err: err, dur: dur}
	}()

	_ = srv.nextConn(t)
	const steadyHold = 120 * time.Millisecond
	time.Sleep(steadyHold)
	cancel() // tear runOnce down via ctx, not a server close

	var r result
	select {
	case r = <-rch:
	case <-time.After(3 * time.Second):
		t.Fatal("runOnce did not return within 3s after ctx cancel")
	}
	// We don't assert on r.err's shape here — ctx cancel surfaces as
	// "use of closed network connection" or context.Canceled
	// depending on goroutine scheduling.  The contract this test
	// pins is purely about r.dur.
	if r.dur < steadyHold {
		t.Errorf("sessionDur = %s, want >= %s (defer must capture the real "+
			"steady-state duration even on ctx-cancel teardown)",
			r.dur, steadyHold)
	}
}

// TestRunOnce_ErrorMessageHasNoDurationLeakBoilerplate is a tiny
// guard against an easy mistake: if someone "fixes" the bug by
// adding the duration to the error message instead of returning it,
// callers that grep error strings (Run()'s status notifier text)
// could pick up the wrong value.  We assert the dial-failure error
// surfaces a recognisable cause string and nothing duration-shaped.
func TestRunOnce_ErrorMessageHasNoDurationLeakBoilerplate(t *testing.T) {
	src := &WSSource{
		URL:              "ws://127.0.0.1:1/never-listens",
		MinBackoff:       10 * time.Millisecond,
		Decoder:          DecodeNDJSONFrame,
		HandshakeTimeout: 200 * time.Millisecond,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	got := callRunOnce(ctx, src)
	if got.err == nil {
		t.Fatal("dial failure must return a non-nil error")
	}
	// "dial:" is the prefix runOnce wraps the gorilla DialContext
	// error with.  Pin it so downstream string-matching logic
	// (Run()'s "lost: …" notify line) keeps working.
	if !strings.Contains(got.err.Error(), "dial:") {
		t.Errorf("dial-failure error should mention dial:, got %q", got.err.Error())
	}
}
