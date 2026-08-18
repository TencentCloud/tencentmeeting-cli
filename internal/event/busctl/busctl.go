// Package busctl is the wire-level control client used by `event status`,
// `event stop`, and the consume process's pre-Hello reachability probe.
//
// It deliberately knows NOTHING about how the bus was started — its sole
// concern is "given the IPC address, talk the protocol".  Discovery (alive
// lock, pid file) lives in busdiscover; busctl trusts the address it's
// handed.
package busctl

import (
	"bufio"
	"bytes"
	"fmt"
	"time"

	eventruntime "tmeet/internal/event"
	"tmeet/internal/event/protocol"
	"tmeet/internal/event/transport"
	"tmeet/internal/exception"
)

// readTimeout caps the wait for a StatusResponse / Shutdown ack so a wedged
// bus can never hang the controlling command.  Aligned with
// protocol.WriteTimeout to keep both sides symmetric.
const readTimeout = 5 * time.Second

// ErrNotRunning is returned by the helpers below when Dial fails because no
// bus is listening on the configured IPC address.  Callers errors.Is to
// distinguish "no bus" from a real protocol failure (Dial succeeded but the
// bus answered with garbage).
var ErrNotRunning = exception.EventBusNotRunningError

// QueryStatus dials the bus, sends StatusQuery, and returns the parsed
// StatusResponse.  Closes the connection before returning.
//
// Possible returns:
//   - (resp, nil)              — bus answered cleanly.
//   - (nil, ErrNotRunning)     — Dial failed (ECONNREFUSED, ENOENT, pipe-not-found).
//   - (nil, other error)       — protocol error after a successful Dial; the
//     underlying error is wrapped for diagnostics.
func QueryStatus(tr transport.IPC) (*protocol.StatusResponse, error) {
	conn, err := tr.Dial(eventruntime.BusSockPath())
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNotRunning, err)
	}
	defer func() { _ = conn.Close() }()

	if err := protocol.EncodeWithDeadline(conn, protocol.NewStatusQuery(), protocol.WriteTimeout); err != nil {
		return nil, fmt.Errorf("busctl: write status_query: %w", err)
	}
	if err := conn.SetReadDeadline(time.Now().Add(readTimeout)); err != nil {
		return nil, fmt.Errorf("busctl: set read deadline: %w", err)
	}
	line, err := protocol.ReadFrame(bufio.NewReader(conn))
	if err != nil {
		return nil, fmt.Errorf("busctl: read status_response: %w", err)
	}
	msg, err := protocol.Decode(bytes.TrimRight(line, "\n"))
	if err != nil {
		return nil, fmt.Errorf("busctl: decode status_response: %w", err)
	}
	resp, ok := msg.(*protocol.StatusResponse)
	if !ok {
		return nil, fmt.Errorf("busctl: unexpected response type %T (wanted StatusResponse)", msg)
	}
	return resp, nil
}

// SendShutdown asks the bus to terminate gracefully.  Returns once the
// frame is on the wire — the bus closes its accept loop asynchronously, so
// callers should poll QueryStatus / Dial to confirm exit.
//
// force=true is a hint to the bus daemon (currently unused; reserved for
// batch 4's WSS source which may want to skip a graceful shutdown of the
// upstream connection).
func SendShutdown(tr transport.IPC, force bool) error {
	conn, err := tr.Dial(eventruntime.BusSockPath())
	if err != nil {
		return fmt.Errorf("%w: %v", ErrNotRunning, err)
	}
	defer func() { _ = conn.Close() }()
	return protocol.EncodeWithDeadline(conn, protocol.NewShutdown(force), protocol.WriteTimeout)
}

// Ping verifies that something is listening on the bus address by Dialling
// and immediately closing.  Used by stop's "did the daemon actually exit?"
// poll-loop and by status as a cheap pre-flight before a full StatusQuery.
func Ping(tr transport.IPC) error {
	conn, err := tr.Dial(eventruntime.BusSockPath())
	if err != nil {
		return fmt.Errorf("%w: %v", ErrNotRunning, err)
	}
	return conn.Close()
}
