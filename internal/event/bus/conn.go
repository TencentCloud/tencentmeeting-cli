// conn.go — per-connection IO loop on the bus side.
//
// One Conn is created per accepted IPC connection AFTER Hello has been
// validated by bus.go:handleConn.  The connection has three goroutines after
// Start():
//
//	SenderLoop  — drains sendCh, writes to the socket via writeFrame.
//	ReaderLoop  — reads control frames (Bye) until EOF.
//	(implicit)  — Hub.Publish writes into sendCh.
//
// All net.Conn writes go through writeFrame which holds writeMu for the
// duration of (SetWriteDeadline + protocol.Encode).  Without that, a Hello
// ack racing a Hub.Publish could interleave bytes on the wire.
//
// Drop-oldest back-pressure lives in PushDropOldest: on a full sendCh we
// evict ONE queued event under sendMu, then retry the push, atomically.
// Without sendMu a concurrent SenderLoop drain could turn the "drop one,
// push one" pair into "drop two, push one" or worse.

package bus

import (
	"bufio"
	"bytes"
	"context"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"tmeet/internal/event/protocol"
	tlog "tmeet/internal/log"
)

const (
	// sendChCap mirrors the per-key BufferSize default in event/types.go;
	// the dynamic per-key value lives in the registry's KeyDef.BufferSize
	// and is wired in batch 3 when consume reads it from the schema.
	sendChCap = 100

	// writeTimeout caps any single net.Conn.Write so a wedged consumer
	// can't stall the sender goroutine indefinitely.  Same value as
	// protocol.WriteTimeout to keep deadlines consistent across the
	// codebase.
	writeTimeout = 5 * time.Second
)

// Conn represents a single subscriber connection from the bus's perspective.
//
// Construction is split between bus.handleConn (which validates Hello and
// hands us a *bufio.Reader with already-buffered bytes) and NewConn.  Direct
// callers SHOULD NOT bypass bus.handleConn; the bufio handoff is essential
// for not losing the second-frame bytes that ReadFrame may have eagerly
// pulled off the wire.
type Conn struct {
	conn        net.Conn
	reader      *bufio.Reader
	eventKey    string
	params      map[string]string
	pid         int
	openIDH     string
	agentOpenID string
	logger      *tlog.Logger
	hub         *Hub
	bus         *Bus

	sendCh  chan interface{}
	sendMu  sync.Mutex // serialises drop+push in PushDropOldest
	writeMu sync.Mutex // serialises every net.Conn write

	closed    chan struct{}
	closeOnce sync.Once
	onClose   func(*Conn)

	received atomic.Int64
	dropped  atomic.Int64
}

// NewConn constructs a Conn.  reader may be a bufio.Reader created earlier by
// bus.handleConn (so any bytes pulled past the Hello frame survive); pass
// nil to start fresh.
//
// params is the validated --param map from Hello (already screened by the
// L1 consumer, but stored verbatim here so the hub's L2 MatchPayload can
// filter without re-validating).  An empty/nil map disables filtering for
// this conn (any matching-EventKey event is delivered).
func NewConn(conn net.Conn, reader *bufio.Reader, eventKey string, params map[string]string, pid int, openIDHash, agentOpenID string, hub *Hub, b *Bus) *Conn {
	if reader == nil {
		reader = bufio.NewReader(conn)
	}
	return &Conn{
		conn:        conn,
		reader:      reader,
		eventKey:    eventKey,
		params:      params,
		pid:         pid,
		openIDH:     openIDHash,
		agentOpenID: agentOpenID,
		hub:         hub,
		bus:         b,
		sendCh:      make(chan interface{}, sendChCap),
		closed:      make(chan struct{}),
	}
}

// SetLogger attaches a logger to this conn (nil tolerated).
func (c *Conn) SetLogger(l *tlog.Logger) { c.logger = l }

// SetOnClose installs a callback fired exactly once when the conn shuts down
// (graceful Bye, peer EOF, write error, or Bus.shutdownConns).  Used by
// bus.go to delete the conn from the active set and reset the idle timer.
func (c *Conn) SetOnClose(fn func(*Conn)) { c.onClose = fn }

// Subscriber-interface accessors -----------------------------------------------

func (c *Conn) EventKey() string          { return c.eventKey }
func (c *Conn) Params() map[string]string { return c.params }
func (c *Conn) AgentOpenID() string       { return c.agentOpenID }
func (c *Conn) PID() int                  { return c.pid }
func (c *Conn) Received() int64           { return c.received.Load() }
func (c *Conn) IncrementReceived()        { c.received.Add(1) }
func (c *Conn) DroppedCount() int64       { return c.dropped.Load() }
func (c *Conn) IncrementDropped()         { c.dropped.Add(1) }

// Start launches the sender and reader goroutines.  Call exactly once after
// the bus has registered the conn with the hub.
func (c *Conn) Start() {
	go c.senderLoop()
	go c.readerLoop()
}

// Close is idempotent.  Calling it from any goroutine triggers graceful
// shutdown: writers see closed signalled, reader sees the underlying conn
// closed and returns from ReadFrame, onClose fires once.
func (c *Conn) Close() { c.shutdown() }

// writeFrame is the sole write path.  Locking writeMu serialises the
// SetWriteDeadline + Encode pair against every other writer.
func (c *Conn) writeFrame(msg interface{}) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if err := c.conn.SetWriteDeadline(time.Now().Add(writeTimeout)); err != nil {
		return err
	}
	return protocol.Encode(c.conn, msg)
}

// senderLoop drains sendCh until c.closed is signalled.  We read closed from
// the *select* (not from a sendCh close) so the hub can keep pushing into
// sendCh without panicking on a closed channel.
func (c *Conn) senderLoop() {
	for {
		select {
		case <-c.closed:
			return
		case msg := <-c.sendCh:
			if err := c.writeFrame(msg); err != nil {
				// c.logger may be nil; *tlog.Logger.Warnf is nil-safe.
				c.logger.Warnf(context.Background(),
					"conn: write to pid=%d failed: %v", c.pid, err)
				c.shutdown()
				return
			}
		}
	}
}

// readerLoop reads control frames (Bye) until EOF or error.
//
// The bus deliberately accepts ONLY Bye on the conn after Hello; any other
// inbound message is logged and ignored — the consumer should never send
// Event/Hello/etc post-handshake, and silently dropping unexpected frames is
// safer than tearing the conn down on every malformed byte.
func (c *Conn) readerLoop() {
	for {
		line, err := protocol.ReadFrame(c.reader)
		if err != nil {
			break
		}
		line = bytes.TrimRight(line, "\n")
		if len(line) == 0 {
			continue
		}
		msg, err := protocol.Decode(line)
		if err != nil {
			c.logger.Warnf(context.Background(),
				"conn: pid=%d sent malformed frame: %v", c.pid, err)
			continue
		}
		switch m := msg.(type) {
		case *protocol.Bye:
			c.logger.Infof(context.Background(),
				"conn: consumer pid=%d sent bye (reason=%q)", c.pid, m.Reason)
			c.shutdown()
			return
		default:
			c.logger.Warnf(context.Background(),
				"conn: pid=%d sent unexpected post-handshake frame %T", c.pid, m)
		}
	}
	c.shutdown()
}

// shutdown is the once-only teardown.  Closing c.closed first makes
// senderLoop exit promptly; conn.Close then unblocks readerLoop's ReadFrame.
// onClose runs LAST so the callback observes a fully torn-down conn.
func (c *Conn) shutdown() {
	c.closeOnce.Do(func() {
		close(c.closed)
		_ = c.conn.Close()
		// sendCh is intentionally NOT closed: a concurrent Hub.Publish may
		// still hold a reference to it.  Letting GC reclaim it after the
		// last sender goroutine exits is simpler than coordinating a closer.
		if c.onClose != nil {
			c.onClose(c)
		}
	})
}

// PushDropOldest enqueues msg with drop-oldest semantics.  Returns
// (enqueued, dropped):
//
//	enqueued — true iff msg ended up in sendCh.
//	dropped  — true iff we evicted an oldest message to make room.
//
// In the rare race where SenderLoop drains the chan between our
// initial-failed push and the eviction attempt, we may end up needing no
// eviction at all and still succeed; that path returns (true, false).
func (c *Conn) PushDropOldest(msg interface{}) (enqueued, dropped bool) {
	c.sendMu.Lock()
	defer c.sendMu.Unlock()
	select {
	case c.sendCh <- msg:
		return true, false
	default:
	}
	select {
	case <-c.sendCh:
		dropped = true
	default:
	}
	select {
	case c.sendCh <- msg:
		return true, dropped
	default:
		return false, dropped
	}
}

// TrySend is non-evictive but shares sendMu with PushDropOldest so a control
// frame slipping in between an evict-and-push pair can't break the
// atomicity contract.  Returns false on a full channel.
func (c *Conn) TrySend(msg interface{}) bool {
	c.sendMu.Lock()
	defer c.sendMu.Unlock()
	select {
	case c.sendCh <- msg:
		return true
	default:
		return false
	}
}

// SendDirect bypasses sendCh and writes synchronously via writeFrame.  Used
// for HelloAck where we MUST hand the consumer a definitive yes/no before
// either side proceeds; using sendCh would race with the goroutine startup.
func (c *Conn) SendDirect(msg interface{}) error {
	return c.writeFrame(msg)
}
