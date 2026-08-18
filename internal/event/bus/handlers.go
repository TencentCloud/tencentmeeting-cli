// handlers.go — frame dispatch on accepted connections.
//
// One handler per top-level frame type the bus accepts inbound:
//
//	hello          — register a subscriber (after owner_hash check).
//	status_query   — reply with StatusResponse and close.
//	shutdown       — signal the daemon to exit and close.
//
// Anything else (Event, HelloAck, Control, Bye-as-first-frame, ...) is a
// protocol violation; we log and close.  Keeping the dispatch small means
// any future frame type lands in exactly one place.

package bus

import (
	"bufio"
	"bytes"
	"context"
	"net"
	"os"
	"time"

	eventruntime "tmeet/internal/event"
	"tmeet/internal/event/protocol"
)

// helloReadTimeout — how long we wait for the consumer's first frame.
//
// Five seconds is generous for a same-host IPC handshake (typical: <1 ms)
// while preventing a malicious / wedged peer from holding a goroutine open
// indefinitely by Dialing and never writing.
const helloReadTimeout = 5 * time.Second

// handleConn is the entry point for every accepted connection.  It reads the
// first frame and dispatches; the bufio.Reader is then handed to the
// downstream handler so any bytes already buffered past the first frame
// aren't lost.
func (b *Bus) handleConn(conn net.Conn) {
	// handlers run in their own goroutines spawned from acceptLoop; they
	// don't carry the bus Run-ctx (passing it would tie individual handler
	// lifetimes to bus shutdown which we deliberately avoid — see
	// shutdownConns for the orderly teardown path).  context.Background()
	// is used purely as the carrier for trace metadata in tlog; cancellation
	// signals are not derived from it.
	ctx := context.Background()

	br := bufio.NewReader(conn)
	if err := conn.SetReadDeadline(time.Now().Add(helloReadTimeout)); err != nil {
		_ = conn.Close()
		return
	}
	line, err := protocol.ReadFrame(br)
	if err != nil {
		_ = conn.Close()
		return
	}
	// Reset the read deadline; downstream handlers manage their own.
	_ = conn.SetReadDeadline(time.Time{})

	msg, err := protocol.Decode(bytes.TrimRight(line, "\n"))
	if err != nil {
		b.logger.Warnf(ctx, "bus: malformed first frame from %s: %v", conn.RemoteAddr(), err)
		_ = conn.Close()
		return
	}

	switch m := msg.(type) {
	case *protocol.Hello:
		b.handleHello(ctx, conn, br, m)
	case *protocol.StatusQuery:
		b.handleStatusQuery(conn)
	case *protocol.Shutdown:
		b.handleShutdown(ctx, conn)
	default:
		b.logger.Warnf(ctx, "bus: unexpected first frame %T from %s", m, conn.RemoteAddr())
		_ = conn.Close()
	}
}

// handleHello validates owner identity, registers the subscriber, and starts
// the per-conn IO goroutines.  On any rejection we send HelloAck{Error:...}
// before closing so the consumer can produce a useful diagnostic.
func (b *Bus) handleHello(ctx context.Context, conn net.Conn, reader *bufio.Reader, hello *protocol.Hello) {
	// 1. Owner check — single most important security gate.  A bus bound to
	//    user A must never accept Hello from user B; once the consumer is
	//    Hello-acked, every subsequent event flows over the wire.
	if hello.OpenIDHash != b.cfg.OpenIDHash {
		b.logger.Warnf(ctx, "bus: rejecting Hello from pid=%d owner=%q (expected=%q)",
			hello.PID, hello.OpenIDHash, b.cfg.OpenIDHash)
		ack := protocol.NewHelloAckError(
			protocol.HelloErrWrongOwner,
			b.cfg.OpenIDHash,
			"bus is bound to a different user; logout the other tmeet account or run 'tmeet event stop --force'",
		)
		_ = protocol.EncodeWithDeadline(conn, ack, protocol.WriteTimeout)
		_ = conn.Close()
		return
	}

	// 2. EventKey validation.
	//
	// Hello.EventKeys is a slice on the wire so the shape can absorb
	// multi-key subscriptions without a protocol churn, but this release
	// only supports a single-key subscription per Hello.  Anything else is
	// rejected up-front with InvalidParams; the bus's downstream state
	// (Conn, Hub, subreg) is still built around "one Conn == one EventKey"
	// and would silently under-serve a multi-key request.
	if len(hello.EventKeys) != 1 {
		b.logger.Warnf(ctx, "bus: rejecting Hello from pid=%d with EventKeys=%v (expected exactly one key)",
			hello.PID, hello.EventKeys)
		ack := protocol.NewHelloAckError(
			protocol.HelloErrInvalidParams,
			"",
			"Hello.event_keys must contain exactly one EventKey; multi-key subscriptions are not supported",
		)
		_ = protocol.EncodeWithDeadline(conn, ack, protocol.WriteTimeout)
		_ = conn.Close()
		return
	}
	eventKey := hello.EventKeys[0]

	// 2a. EventKey existence — saves the consumer from waiting for events
	//     that will never come because they typo'd the key.
	if _, ok := eventruntime.Lookup(eventKey); !ok {
		b.logger.Warnf(ctx, "bus: rejecting Hello from pid=%d unknown EventKey=%q",
			hello.PID, eventKey)
		ack := protocol.NewHelloAckError(
			protocol.HelloErrUnknownKey,
			"",
			"unknown EventKey; run 'tmeet event list' for the registered set",
		)
		_ = protocol.EncodeWithDeadline(conn, ack, protocol.WriteTimeout)
		_ = conn.Close()
		return
	}

	// 3. Build the Conn now (still no goroutines running).
	bc := NewConn(conn, reader, eventKey, hello.Params, hello.PID, hello.OpenIDHash, hello.AgentOpenID, b.hub, b)
	bc.SetLogger(b.logger)

	// 4. Wire onClose — must be set BEFORE Register so we never miss a
	//    teardown notification.
	bc.SetOnClose(func(c *Conn) {
		b.hub.Unregister(c)
		b.mu.Lock()
		delete(b.conns, c)
		remaining := len(b.conns)
		b.mu.Unlock()
		b.logger.Infof(ctx, "bus: consumer disconnected: pid=%d key=%s (remaining=%d)",
			c.PID(), c.EventKey(), remaining)
		if remaining == 0 {
			b.resetIdleTimer()
		}
	})

	// 5. Register with hub & track in b.conns under one critical section so
	//    a concurrent idle-timer fire can't race a fresh registration.
	b.hub.Register(bc)
	b.mu.Lock()
	b.conns[bc] = struct{}{}
	b.stopIdleTimerLocked()
	b.mu.Unlock()

	// 6. Send the OK ack BEFORE starting goroutines.  If the ack fails we
	//    unwind everything; otherwise the consumer might think we accepted
	//    while we'll soon close on the failed write.
	ack := protocol.NewHelloAckOK(b.cfg.BusVersion)
	if err := bc.SendDirect(ack); err != nil {
		b.logger.Warnf(ctx, "bus: hello_ack write to pid=%d key=%s failed: %v (rejecting)",
			hello.PID, eventKey, err)
		bc.Close() // triggers onClose → unregister + b.conns cleanup
		return
	}

	b.logger.Infof(ctx, "bus: consumer connected: pid=%d key=%q owner=%q", hello.PID, eventKey, hello.OpenIDHash)
	bc.Start()
}

// handleStatusQuery replies with a StatusResponse snapshot and closes.
func (b *Bus) handleStatusQuery(conn net.Conn) {
	defer func() { _ = conn.Close() }()

	resp := &protocol.StatusResponse{
		Type:           protocol.MsgTypeStatusResponse,
		PID:            os.Getpid(),
		OwnerHash:      b.cfg.OpenIDHash,
		StartedAt:      b.startTime.UTC().Format(time.RFC3339),
		UptimeSec:      int(time.Since(b.startTime).Seconds()),
		ActiveConns:    b.hub.ConnCount(),
		SubscribedKeys: b.hub.SubscribedKeys(),
		Consumers:      b.hub.Consumers(),
	}
	_ = protocol.EncodeWithDeadline(conn, resp, protocol.WriteTimeout)
}

// handleShutdown signals the main loop to exit and closes the requesting conn.
func (b *Bus) handleShutdown(ctx context.Context, conn net.Conn) {
	defer func() { _ = conn.Close() }()
	b.logger.Infof(ctx, "bus: received shutdown command from %s", conn.RemoteAddr())
	b.signalShutdown()
}

// stopIdleTimerLocked stops & drains the idle timer.  Must be called with
// b.mu held.  Stop+drain in a single helper because Go's time.Timer needs
// both to avoid stale fires (cf. Timer.Reset docs).
func (b *Bus) stopIdleTimerLocked() {
	if b.idleTimer == nil {
		return
	}
	if !b.idleTimer.Stop() {
		select {
		case <-b.idleTimer.C:
		default:
		}
	}
}

// resetIdleTimer is the unlocked counterpart called from onClose paths.
// Acquires b.mu briefly so the stop+drain pair stays atomic relative to
// other timer ops.
func (b *Bus) resetIdleTimer() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.stopIdleTimerLocked()
	if b.idleTimer != nil {
		b.idleTimer.Reset(b.cfg.IdleTimeout)
	}
}
