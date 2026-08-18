// consume_runner.go — IPC handshake + event loop for `event consume`.
//
// Split from consume.go so cobra wiring and validation stay in one file
// while the (longer) IO/lifecycle plumbing lives here.
//
// Contract recap (consume command):
//
//   stdout  — one NDJSON line per business event delivered (Event frames).
//   stderr  — control-plane diagnostics:
//             [event] ready event_key=<key>           (always; not --quiet-able)
//             [event] received trace_id=<id>          (informational)
//             [source] <source>: <state> ...          (informational)
//             [event] WARN dropped <count> ...        (warn; always on)
//             [event] exited — received N event(s) ...(always; not --quiet-able)
//
//   exit codes:
//             0 — graceful (limit / timeout / signal / shutdown).
//             1 — fatal (Hello rejected, IO error, etc.).
//
// The loop is a 4-way select:
//   - ctx.Done()           ⇒ external cancellation (signal, parent timeout).
//   - timeoutCh            ⇒ --timeout fired.
//   - msgCh (frame parsed) ⇒ event / control / bye / decode error.
//   - readErrCh            ⇒ socket EOF or read error from the reader goroutine.

package event

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	eventruntime "tmeet/internal/event"
	"tmeet/internal/event/protocol"
	"tmeet/internal/event/transport"
	"tmeet/internal/exception"
	"tmeet/internal/output"
)

// runLoop is the long-running body of `event consume` after ensureBus.
//
// Returns nil on graceful exit (the caller — Run — returns nil to
// cobra and the process exits 0).  Returns a non-nil error on fatal
// failures; cobra's exit-code mapping in root.go translates that to
// exit 1.  We use the project-wide exception.* types so the original
// error class (EventBusError / EventInternalError / TokenExpiredError /
// InvalidArgsError) is preserved across the call boundary.
//
// All stderr output is funnelled through the `o.stderrf` (--quiet aware)
// or `output.EventStderr` (always on) helpers, so consumers grep-ing
// for the ready/exit marker have a stable contract regardless of internal
// refactors.
func (o *ConsumeOptions) runLoop(ctx context.Context, cmd *cobra.Command, tr transport.IPC, ownerHash string) error {
	// 1. Connect.
	netConn, err := tr.Dial(eventruntime.BusSockPath())
	if err != nil {
		return exception.EventBusError.With("dial bus: %v", err)
	}
	defer func() { _ = netConn.Close() }()

	// 2. Send Hello.
	//
	// Hello.EventKeys is a slice on the wire so future multi-key subscriptions
	// (--event-id "a|b" / --domain <d>) can land without a protocol churn.
	// This release is single-key only, so we always ship a 1-element slice
	// and the bus rejects any other length up-front.
	hello := protocol.NewHello(
		os.Getpid(), []string{o.EventKey}, o.Params,
		ownerHash, o.BusVersion, fmt.Sprintf("consume-%d-%d", os.Getpid(), time.Now().UnixNano()),
		o.AgentOpenID,
	)
	if err := protocol.EncodeWithDeadline(netConn, hello, protocol.WriteTimeout); err != nil {
		return exception.EventBusError.With("send hello: %v", err)
	}

	// 3. Read HelloAck (with deadline).
	br := bufio.NewReader(netConn)
	if err := netConn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return exception.EventBusError.With("set read deadline: %v", err)
	}
	line, err := protocol.ReadFrame(br)
	if err != nil {
		return exception.EventBusError.With("read hello_ack: %v", err)
	}
	_ = netConn.SetReadDeadline(time.Time{}) // clear for the steady-state loop

	ackMsg, err := protocol.Decode(bytes.TrimRight(line, "\n"))
	if err != nil {
		return exception.EventInternalError.With("decode hello_ack: %v", err)
	}
	ack, ok := ackMsg.(*protocol.HelloAck)
	if !ok {
		return exception.EventInternalError.With("expected HelloAck, got %T", ackMsg)
	}

	// 4. Map HelloAck.Error to user-facing messages.  Each branch returns a
	// fatal error so the process exits 1 — recovery here is the user's job.
	switch ack.Error {
	case "":
		// success; fall through.
	case protocol.HelloErrWrongOwner:
		output.EventStderr(cmd, "[event] bus owner mismatch (bus=%s consume=%s); run 'tmeet event stop --force' to clean up",
			ack.ExpectedOwnerHash, ownerHash)
		return exception.EventBusError.With("bus owner mismatch")
	case protocol.HelloErrUnknownKey:
		output.EventStderr(cmd, "[event] bus rejected EventKey %q (detail: %s); run 'tmeet event list' to discover registered keys",
			o.EventKey, ack.Detail)
		return exception.InvalidArgsError.With("unknown EventKey %q", o.EventKey)
	default:
		output.EventStderr(cmd, "[event] handshake failed: %s (%s)", ack.Error, ack.Detail)
		return exception.EventBusError.With("hello rejected: %s", ack.Error)
	}

	o.stderrf(cmd, "[event] handshake ok bus_version=%s", ack.BusVersion)

	// 5. Ready marker.  ALWAYS emitted, even with --quiet, because Agents
	// rely on this exact pattern as a synchronisation barrier.
	output.EventStderr(cmd, "[event] ready event_key=%s", o.EventKey)

	startedAt := time.Now()

	// 6. Wire signal handling.  We translate SIGINT/SIGTERM into ctx
	// cancellation so the main select treats them like any other shutdown
	// trigger.  No SIGHUP — terminal closure is handled by the OS killing
	// our process directly, and trapping SIGHUP would prevent that.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	loopCtx, loopCancel := context.WithCancel(ctx)
	defer loopCancel()

	go func() {
		select {
		case <-sigCh:
			loopCancel()
		case <-loopCtx.Done():
		}
	}()

	// 7. --timeout — implemented as a single AfterFunc rather than a Timer
	// in the select because we want exactly one trigger; reset/race
	// concerns disappear when the goroutine just calls cancel() once.
	var timeoutFired atomic.Bool
	if o.Timeout > 0 {
		t := time.AfterFunc(o.Timeout, func() {
			timeoutFired.Store(true)
			loopCancel()
		})
		defer t.Stop()
	}

	// 8. Reader goroutine — pumps frames into a channel.  Must be a
	// goroutine (not inline) because ReadFrame blocks indefinitely; we want
	// the main loop to be able to exit on signal without waiting for the
	// next frame.
	type framePkt struct {
		msg interface{}
		err error
	}
	frameCh := make(chan framePkt, 8)
	go func() {
		defer close(frameCh)
		for {
			line, err := protocol.ReadFrame(br)
			if err != nil {
				frameCh <- framePkt{err: err}
				return
			}
			line = bytes.TrimRight(line, "\n")
			if len(line) == 0 {
				continue
			}
			msg, derr := protocol.Decode(line)
			if derr != nil {
				// Malformed frame is a hard error: we can't recover the
				// stream sync after a bad line.  Surface and exit.
				frameCh <- framePkt{err: exception.EventInternalError.With("decode frame: %v", derr)}
				return
			}
			frameCh <- framePkt{msg: msg}
		}
	}()

	// 9. Main loop.
	var (
		delivered int64
		reason    = "signal" // default if ctx fires without another reason
	)

loop:
	for {
		select {
		case <-loopCtx.Done():
			if timeoutFired.Load() {
				reason = "timeout"
			}
			// else: signal / parent ctx; "signal" already set.
			break loop

		case pkt, open := <-frameCh:
			if !open {
				// Reader exited without sending a final pkt — shouldn't
				// happen because we always send before returning, but be
				// defensive.
				reason = "shutdown"
				break loop
			}
			if pkt.err != nil {
				if isExpectedReadErr(pkt.err) {
					reason = "shutdown"
					break loop
				}
				output.EventStderr(cmd, "[event] read error: %v", pkt.err)
				_ = sendBye(netConn, "read_error")
				return exception.EventBusError.With("read frame: %v", pkt.err)
			}

			done, exitReason, err := o.handleFrame(cmd, pkt.msg, &delivered)
			if err != nil {
				_ = sendBye(netConn, "handle_error")
				return err
			}
			if done {
				reason = exitReason
				break loop
			}
		}
	}

	// 10. Best-effort goodbye on the way out.  We don't error if it fails
	// (the bus may already be tearing down our conn).
	_ = sendBye(netConn, reason)

	// 10b. Ensure the reader goroutine terminates BEFORE we return, so the
	// goroutine never outlives runLoop.  Two failure modes are possible
	// after `break loop`:
	//
	//   (a) reader is blocked in protocol.ReadFrame — closing the socket
	//       unblocks it with net.ErrClosed.
	//   (b) reader is blocked writing to frameCh because the main loop
	//       drained no further packets after high-rate fan-out filled the
	//       buffer (cap 8) — closing the socket alone won't help; we must
	//       drain frameCh until reader's `defer close(frameCh)` fires.
	//
	// Closing netConn here (instead of relying on the deferred Close at
	// the top of the function) handles (a); ranging over frameCh handles
	// (b).  Together they guarantee no goroutine leak even if a future
	// caller wraps runLoop in a longer-lived process.
	_ = netConn.Close()
	for range frameCh {
	}

	// 11. Exit line — ALWAYS emitted, even with --quiet.
	elapsed := time.Since(startedAt).Round(time.Millisecond)
	output.EventStderr(cmd, "[event] exited — received %d event(s) in %s (reason: %s)",
		delivered, elapsed, reason)

	return nil
}

// runConsumeLoop is a thin shim retained so consume_test.go can keep its
// many `runConsumeLoop(ctx, cmd, opts, transport.New(), ownerHash)` calls
// without touching the test code.  Production code reaches the loop via
// (*ConsumeOptions).runLoop directly from Run.
func runConsumeLoop(ctx context.Context, cmd *cobra.Command, opts *ConsumeOptions, tr transport.IPC, ownerHash string) error {
	return opts.runLoop(ctx, cmd, tr, ownerHash)
}

// handleFrame processes one decoded frame.
//
// Return values:
//   - done bool   — whether the loop should exit after this frame.
//   - reason str  — exit reason (only meaningful when done==true).
//   - err error   — fatal error; the loop must exit and Run returns 1.
//
// We track `delivered` via a *int64 pointer so --max-events compares against
// the post-write count without a separate counter living in the runner.
func (o *ConsumeOptions) handleFrame(cmd *cobra.Command, msg interface{}, delivered *int64) (done bool, reason string, err error) {
	switch m := msg.(type) {
	case *protocol.Event:
		// --output-dir gets the FULL event regardless of --jq: the
		// directory is documented as an audit trail (one file per
		// trace_id), not a filtered view.  jq is a stdout-channel
		// projection.
		if o.OutputDir != "" {
			if werr := o.writeEventToOutputDir(m); werr != nil {
				// --output-dir failure is a WARN, not fatal: the stdout
				// path may still succeed below so the caller still has
				// the data.
				output.EventStderr(cmd, "[event] WARN output-dir write failed trace_id=%s: %v",
					m.TraceID, werr)
			}
		}

		// stdout path: either default NDJSON or jq-projected.
		if o.jqFilter == nil {
			if werr := o.writeEventToStdout(cmd, m); werr != nil {
				if errors.Is(werr, syscall.EPIPE) {
					return true, "signal", nil
				}
				return false, "", exception.EventInternalError.With("write event to stdout: %v", werr)
			}
			n := atomic.AddInt64(delivered, 1)
			o.stderrf(cmd, "[event] received trace_id=%s", m.TraceID)
			if o.MaxEvents > 0 && n >= int64(o.MaxEvents) {
				return true, "limit", nil
			}
			return false, "", nil
		}

		return o.emitEventViaJQ(cmd, m, delivered)

	case *protocol.Control:
		switch m.Kind {
		case protocol.ControlKindSourceStatus:
			o.stderrf(cmd, "[source] %s: %s%s",
				m.Source, m.State, suffixIfDetail(m.Detail))
			if m.State == protocol.SourceStateAuthExpired {
				return false, "", exception.TokenExpiredError
			}
		case protocol.ControlKindDropped:
			// Always on \u2014 drops indicate data loss the operator must see.
			output.EventStderr(cmd, "[event] WARN dropped %d event(s) for key=%s since unix=%d",
				m.Count, m.EventKey, m.SinceTS)
		case protocol.ControlKindSubscribeError:
			// Upstream WsCLISubscribeEvent rejected this key (or its
			// rsp timed out).  The gateway will never push events for
			// it, so a continued wait would be silent failure.  We
			// surface the diagnostic on stderr (always on — not
			// suppressible by --quiet, same policy as dropped) and
			// return a fatal error so the process exits 1; cobra's
			// exit-code mapping in root.go does the rest.  We
			// deliberately do NOT write to stdout / --output-dir /
			// --jq for this frame: the public stdout schema is
			// business events only.
			output.EventStderr(cmd, "[event] WARN subscribe failed key=%s code=%d%s",
				m.EventKey, m.Code, suffixIfDetail(m.Detail))
			return false, "", exception.EventBusError.With("subscribe_failed key=%s code=%d msg=%q",
				m.EventKey, m.Code, m.Detail)
		case protocol.ControlKindBye:
			return true, "shutdown", nil
		default:
			o.stderrf(cmd, "[event] unknown control kind=%s", m.Kind)
		}
		return false, "", nil

	case *protocol.Bye:
		return true, "shutdown", nil

	default:
		// HelloAck / StatusResponse here would be a protocol bug; log
		// rather than crash to ease forward-compat.
		o.stderrf(cmd, "[event] unexpected frame type %T (ignored)", m)
		return false, "", nil
	}
}

// writeEventToStdout serialises ev to a single NDJSON line on cmd.Stdout.
//
// We re-marshal the wire-level *protocol.Event into the publicly-documented
// {event, trace_id, payload} shape rather than passing through the raw
// "type":"event" envelope, because the consume command's public stdout
// schema has NO type field and downstream Agents rely on that.
//
// Receiver kept on *ConsumeOptions for symmetry with the other emit methods
// even though the body doesn't read any o.* state today; future projections
// (e.g. --no-trace-id) will live here.
func (o *ConsumeOptions) writeEventToStdout(cmd *cobra.Command, ev *protocol.Event) error {
	out := struct {
		Event   string          `json:"event"`
		TraceID string          `json:"trace_id"`
		Payload json.RawMessage `json:"payload"`
	}{
		Event:   ev.Event,
		TraceID: ev.TraceID,
		Payload: ev.Payload,
	}
	buf, err := json.Marshal(out)
	if err != nil {
		return err
	}
	w := cmd.OutOrStdout()
	if _, err := w.Write(buf); err != nil {
		return err
	}
	if _, err := io.WriteString(w, "\n"); err != nil {
		return err
	}
	return nil
}

// writeEventToOutputDir writes <output-dir>/<trace_id>.json with the same
// shape stdout receives.  Filename uses TraceID after a tiny safety filter
// (trace IDs are server-generated but defensively rejecting path separators
// is cheap insurance).
func (o *ConsumeOptions) writeEventToOutputDir(ev *protocol.Event) error {
	if o.OutputDir == "" {
		return nil
	}
	fname := sanitizeTraceID(ev.TraceID)
	if fname == "" {
		return errors.New("empty trace_id")
	}
	out := struct {
		Event   string          `json:"event"`
		TraceID string          `json:"trace_id"`
		Payload json.RawMessage `json:"payload"`
	}{
		Event:   ev.Event,
		TraceID: ev.TraceID,
		Payload: ev.Payload,
	}
	buf, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(o.OutputDir, fname+".json")
	return os.WriteFile(path, append(buf, '\n'), 0o644)
}

// sanitizeTraceID strips path separators / NUL / ".." from trace IDs so a
// pathologically-named trace can't escape the output directory.
func sanitizeTraceID(s string) string {
	if s == "" {
		return ""
	}
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '/', '\\', 0, ':':
			out = append(out, '_')
		default:
			out = append(out, c)
		}
	}
	clean := string(out)
	if clean == "." || clean == ".." {
		return "_"
	}
	return clean
}

// suffixIfDetail composes " (<detail>)" or "" so the stderr line stays
// readable when a source omits Detail.
func suffixIfDetail(detail string) string {
	if detail == "" {
		return ""
	}
	return " (" + detail + ")"
}

// sendBye writes a Bye frame to conn.  Best-effort; caller should treat any
// error as already-disconnected.
func sendBye(conn net.Conn, reason string) error {
	return protocol.EncodeWithDeadline(conn, protocol.NewBye(reason), protocol.WriteTimeout)
}

// isExpectedReadErr returns true for errors that signal "the bus closed our
// conn cleanly" — EOF, "use of closed network connection", and similar.
//
// We treat these as a graceful "shutdown" reason rather than a fatal error
// so a `tmeet event stop` that races with consume's read loop doesn't make
// the consumer exit 1 spuriously.
func isExpectedReadErr(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}
	if errors.Is(err, net.ErrClosed) {
		return true
	}
	// Net package wraps "broken pipe" as *net.OpError; matching by string
	// is ugly but stable across platforms.
	msg := err.Error()
	if msg == "" {
		return false
	}
	return contains(msg, "use of closed network connection") ||
		contains(msg, "connection reset by peer") ||
		contains(msg, "broken pipe")
}

// emitEventViaJQ runs o.jqFilter against the event (root chosen by
// JQRoot), writes 0..N NDJSON lines to stdout and updates delivered counts.
//
// Contract recap (jq emission):
//   - dropped (no result / single null)         → silent, NOT counted.
//   - per-event runtime error                    → stderr WARN, NOT counted.
//   - 1+ non-null results                        → each becomes one NDJSON
//     line, each counted.
//
// We check --max-events after every emitted line so a generative filter
// (one that yields multiple values per event) can stop mid-event when the
// quota is hit; the remaining values are simply not emitted.
func (o *ConsumeOptions) emitEventViaJQ(cmd *cobra.Command, ev *protocol.Event, delivered *int64) (done bool, reason string, err error) {
	// Build the jq input root according to the EventKey's JQRootPath.
	// "."         ⇒ pass the full envelope {event, trace_id, payload}.
	// ".payload"  ⇒ pass only the payload object.
	// Any other value would be a registry-validation bug, not a runtime
	// concern (event.RegisterKey panics on registration); fall back to "."
	// defensively so a stale build can still run.
	var docBytes []byte
	switch o.JQRoot {
	case ".payload":
		docBytes = ev.Payload
	default:
		// Re-marshal the publicly-visible envelope shape so user filters
		// can reference `.event`, `.trace_id`, `.payload.*` consistently
		// with what stdout NDJSON normally looks like.
		envelope := struct {
			Event   string          `json:"event"`
			TraceID string          `json:"trace_id"`
			Payload json.RawMessage `json:"payload"`
		}{Event: ev.Event, TraceID: ev.TraceID, Payload: ev.Payload}
		var berr error
		docBytes, berr = json.Marshal(envelope)
		if berr != nil {
			// Marshalling our own envelope shouldn't ever fail; surface
			// as a fatal error rather than a WARN because it would
			// indicate memory corruption / programmer bug.
			return false, "", exception.EventInternalError.With("marshal jq envelope: %v", berr)
		}
	}

	results, dropped, jerr := o.jqFilter.ApplyToDoc(docBytes)
	if jerr != nil {
		// Runtime error — log and skip, do NOT count, do NOT terminate.
		// Always-on stderr (not o.stderrf) so --quiet can't mask
		// data-shape regressions that the user must know about.
		output.EventStderr(cmd, "[event] WARN jq error trace_id=%s: %v", ev.TraceID, jerr)
		return false, "", nil
	}
	if dropped {
		// Filter elected to drop this event (e.g. select(false)).
		// Silent: no stderr, no count change.
		return false, "", nil
	}

	w := cmd.OutOrStdout()
	for _, line := range results {
		if _, werr := w.Write(line); werr != nil {
			if errors.Is(werr, syscall.EPIPE) {
				return true, "signal", nil
			}
			return false, "", exception.EventInternalError.With("write jq result: %v", werr)
		}
		if _, werr := io.WriteString(w, "\n"); werr != nil {
			if errors.Is(werr, syscall.EPIPE) {
				return true, "signal", nil
			}
			return false, "", exception.EventInternalError.With("write jq newline: %v", werr)
		}
		n := atomic.AddInt64(delivered, 1)
		o.stderrf(cmd, "[event] received trace_id=%s", ev.TraceID)
		if o.MaxEvents > 0 && n >= int64(o.MaxEvents) {
			return true, "limit", nil
		}
	}
	return false, "", nil
}

// contains avoids pulling in strings just for one substring check that
// runs on the error path.
func contains(s, sub string) bool {
	return len(s) >= len(sub) && bytes.Contains([]byte(s), []byte(sub))
}
