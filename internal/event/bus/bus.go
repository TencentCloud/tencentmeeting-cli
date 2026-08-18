// Package bus implements the per-host event-bus daemon.
//
// Lifecycle:
//
//  1. _bus is forked by `event consume` (or invoked directly for debugging).
//  2. Bus.Run takes BusAliveLock as a ProcessLock — losing the race means
//     another bus is already running; the loser exits cleanly with nil.
//  3. Bus.Run writes bus.pid + bus.meta atomically, then Listens on the
//     configured IPC transport (unix socket on POSIX, named pipe on Windows).
//  4. The configured Source(s) start in their own goroutines.  Any source
//     returning (nil or error) triggers full bus shutdown — there is no
//     auto-restart at this layer.  Reconnect lives inside the Source.
//  5. The accept loop dispatches each incoming connection by reading the
//     first frame: Hello → register subscriber; StatusQuery → reply & close;
//     Shutdown → trigger orderly exit.
//  6. An idle timer fires after IdleTimeout with zero subscribers and ends
//     the daemon — there is no point keeping the process around when no
//     consumer is listening.
//
// log.go owns the bus.log redirection; conn.go owns per-connection IO; hub.go
// owns subscriber routing and back-pressure.
package bus

import (
	"context"
	"errors"
	"net"
	"os"
	"sync"
	"time"

	"tmeet/internal/core/filelock"
	eventruntime "tmeet/internal/event"
	"tmeet/internal/event/busdiscover"
	"tmeet/internal/event/source"
	"tmeet/internal/event/transport"
	"tmeet/internal/exception"
	tlog "tmeet/internal/log"
)

// IdleTimeout — the bus exits after this long with no active consumer.
//
// Chosen to be > the typical fork-then-Hello latency (~50 ms) by orders of
// magnitude so a fresh consumer never finds the bus mid-shutdown, but short
// enough that a dropped consume doesn't keep the WSS connection alive
// indefinitely.  30 s matches lark-cli.
const IdleTimeout = 30 * time.Second

// Config bundles every knob the bus needs at construction time.
//
// All fields are mandatory except Source which falls back to source.All().
// OpenIDHash MUST be non-empty: a bus with no owner is a security hole
// (any consumer would pass the WrongOwner check trivially).
type Config struct {
	// OpenIDHash identifies the user the bus binds to.
	OpenIDHash string

	// BusVersion is the build version (Tmeet.CLIVersion); written into bus.meta.
	BusVersion string

	// Transport is the IPC implementation; nil means transport.New().
	Transport transport.IPC

	// Source overrides the auto-registered source list; nil means source.All().
	Source []source.Source

	// IdleTimeout overrides the default; zero means IdleTimeout.
	IdleTimeout time.Duration

	// Logger receives bus daemon log lines; nil means logs are dropped.
	// Production callers obtain the logger via SetupBusLogger; tests typically
	// pass nil and rely on *tlog.Logger's nil-receiver safety.
	Logger *tlog.Logger
}

// Bus is the running daemon.
type Bus struct {
	cfg       Config
	transport transport.IPC
	hub       *Hub
	dedup     *eventruntime.Dedup
	logger    *tlog.Logger
	startTime time.Time

	listener  net.Listener
	pidHandle *busdiscover.Handle

	mu         sync.Mutex
	conns      map[*Conn]struct{}
	idleTimer  *time.Timer
	shutdownCh chan struct{}
}

// New constructs a Bus.  It does NOT acquire any locks or open files; that
// happens in Run, so the caller can defer cleanup of paths created by New on
// the error path before Run is reached.
func New(cfg Config) *Bus {
	if cfg.IdleTimeout <= 0 {
		cfg.IdleTimeout = IdleTimeout
	}
	tr := cfg.Transport
	if tr == nil {
		tr = transport.New()
	}
	return &Bus{
		cfg:        cfg,
		transport:  tr,
		hub:        NewHub(),
		dedup:      eventruntime.NewDedup(0), // default capacity (see eventruntime.DefaultDedupCapacity)
		logger:     cfg.Logger,
		conns:      make(map[*Conn]struct{}),
		shutdownCh: make(chan struct{}, 1),
	}
}

// Run binds the IPC, writes pid/meta, starts sources, and blocks until shutdown.
//
// Returns nil when shutdown completes cleanly (idle timeout, ctx cancel, or
// Shutdown frame received).  Returns a non-nil error only on startup failure
// (lock contention is NOT an error; we return nil so the fork-loser exits 0).
func (b *Bus) Run(ctx context.Context) error {
	if b.cfg.OpenIDHash == "" {
		return exception.EventBusError.With("OpenIDHash is required")
	}

	// 1. Take the alive lock.  ErrHeld here is the fork-loser path — exit clean.
	pidHandle, err := busdiscover.WritePIDFile(os.Getpid())
	if err != nil {
		if errors.Is(err, filelock.ErrHeld) {
			b.logger.Infof(ctx, "bus: another bus already holds bus.alive.lock, exiting")
			return nil
		}
		return exception.EventBusError.With("write pid file: %v", err)
	}
	b.pidHandle = pidHandle
	defer func() {
		_ = b.pidHandle.Release()
		_ = busdiscover.RemovePIDFile()
	}()

	// 2. Write bus.meta — writing AFTER the lock means readers can trust meta
	//    while the lock is held: if the lock is dropped, meta becomes stale
	//    but the readers have already seen the alive-lock TryLock succeed (=>
	//    not running), so they ignore the leftover meta as part of cleanup.
	meta := eventruntime.NewBusMeta(b.cfg.OpenIDHash, b.cfg.BusVersion, os.Getpid())
	if err := eventruntime.WriteBusMeta(meta); err != nil {
		return exception.EventBusError.With("write meta: %v", err)
	}
	defer func() { _ = eventruntime.RemoveBusMeta() }()
	// Scrub ws.state on the way out so the next `event status` doesn't
	// surface a stale WSS snapshot that no live bus is updating.
	defer func() { _ = eventruntime.RemoveWSState() }()
	b.startTime = time.Now()

	// 3. Listen.  If a stale socket is left from a hard crash and Listen
	//    fails, probe via Dial — if the Dial succeeds another live bus is
	//    out there, and we exit; if Dial fails, unlink the socket and retry.
	addr := eventruntime.BusSockPath()
	ln, err := b.transport.Listen(addr)
	if err != nil {
		if probe, dialErr := b.transport.Dial(addr); dialErr == nil {
			_ = probe.Close()
			b.logger.Infof(ctx, "bus: another bus is already accepting on %s, exiting", addr)
			return nil
		}
		b.transport.Cleanup(addr)
		ln, err = b.transport.Listen(addr)
		if err != nil {
			return exception.EventBusError.With("listen %s: %v", addr, err)
		}
	}
	b.listener = ln
	defer func() { _ = b.listener.Close() }()
	b.logger.Infof(ctx, "bus: started owner=%s pid=%d addr=%s", b.cfg.OpenIDHash, os.Getpid(), addr)

	// 4. Idle timer.  Reset on each Hello; checked under b.mu against
	//    len(b.conns) to handle the stale-tick race documented in lark-cli.
	b.idleTimer = time.NewTimer(b.cfg.IdleTimeout)

	// Wire the hub's logger here (rather than inside startSources) so the
	// hub can emit diagnostics independent of whether any source is
	// configured.  Without this, a no-source bus path (startSources early
	// returns when len(srcs) == 0) would leave the hub with a nil logger
	// and any later recordDrop / Broadcast warnings would be silently
	// dropped — unlikely in production but easy to trip in tests.
	b.hub.SetLogger(b.logger)

	// 5. Start sources in their own context so the accept loop can terminate
	//    them cleanly if the bus shuts down before a source notices ctx.
	srcCtx, srcCancel := context.WithCancel(ctx)
	defer srcCancel()
	b.startSources(srcCtx)

	// 6. Accept loop runs until listener.Close().
	acceptDone := make(chan struct{})
	go func() {
		defer close(acceptDone)
		b.acceptLoop(ctx)
	}()

	// 7. Main select: wait for any of {ctx cancel, idle timeout, Shutdown}.
	for {
		select {
		case <-ctx.Done():
			b.logger.Infof(ctx, "bus: shutting down (context cancelled)")
		case <-b.idleTimer.C:
			b.mu.Lock()
			active := len(b.conns)
			if active > 0 {
				b.idleTimer.Reset(b.cfg.IdleTimeout)
				b.mu.Unlock()
				continue
			}
			b.mu.Unlock()
			b.logger.Infof(ctx, "bus: shutting down (idle %v, no active connections)", b.cfg.IdleTimeout)
		case <-b.shutdownCh:
			b.logger.Infof(ctx, "bus: shutting down (shutdown command received)")
		}
		break
	}

	// 8. Close listener (kicks Accept), drop conns, wait for accept loop drain.
	_ = b.listener.Close()
	b.shutdownConns()
	<-acceptDone
	b.logger.Infof(ctx, "bus: exited cleanly")
	return nil
}

// shutdownConns snapshots subscribers under lock then closes outside the lock
// (close → onClose → b.mu re-acquisition would deadlock if held here).
func (b *Bus) shutdownConns() {
	b.mu.Lock()
	conns := make([]*Conn, 0, len(b.conns))
	for c := range b.conns {
		conns = append(conns, c)
	}
	b.mu.Unlock()
	for _, c := range conns {
		c.Close()
	}
}

// startSources fires every configured Source in its own goroutine.  Any
// Source.Run return triggers full bus shutdown via shutdownCh — reconnect is
// strictly the Source's responsibility.
func (b *Bus) startSources(ctx context.Context) {
	srcs := b.cfg.Source
	if srcs == nil {
		srcs = source.All()
	}
	if len(srcs) == 0 {
		b.logger.Warnf(ctx, "bus: no event source configured; bus will idle out without emitting")
		return
	}

	// Wire up the upstream-subscribe path.  Wemeet runs at most one
	// subscribable source (the WSS source); if multiple sources are
	// registered we pick the first one that implements Subscribable.
	// MockSource doesn't implement it so this no-ops in dev/test setups.
	var upstream source.Subscribable
	for _, s := range srcs {
		if sb, ok := s.(source.Subscribable); ok {
			upstream = sb
			break
		}
	}
	if upstream != nil {
		// 0→1 transitions on a key fire a single Subscribe upstream.
		// Errors are best-effort: failure here just means the gateway
		// doesn't yet know to push the key; the next reconnect Replay
		// will retry.  agentOpenID is carried per-subscribe so an
		// agent-scoped (子账号) event reaches the gateway tagged with the
		// agent; it is empty for master-only / unrestricted events.
		b.hub.SetOnFirstSubscribe(func(eventKey, agentOpenID string) {
			if err := upstream.Subscribe(ctx, eventKey, agentOpenID); err != nil {
				b.logger.Warnf(ctx, "bus: upstream subscribe(%s) failed: %v", eventKey, err)
			}
		})

		// Wire the SubscribeRsp watcher: the source invokes this from
		// a background goroutine once the gateway's verdict is known.
		// We only act on FAILURES \u2014 a successful subscribe is the
		// happy path and producing a control frame for it would be
		// noise.  On failure we route subscribe_error to consumers of
		// the failed key(s) only; per the consume contract those
		// consumers will exit 1 with reason=subscribe_failed (a
		// permanently-rejected key would otherwise leave the consumer
		// hanging forever waiting for events the gateway will never
		// push).
		if srn, ok := upstream.(source.SubscribeResultNotifiable); ok {
			srn.SetOnSubscribeResult(func(eventKeys []string, code uint32, msg string) {
				if code == 0 && msg == "" {
					return // success; nothing to broadcast.
				}
				for _, k := range eventKeys {
					b.logger.Warnf(ctx, "bus: upstream subscribe(%s) rejected: code=%d msg=%q", k, code, msg)
					b.hub.BroadcastSubscribeError(k, code, msg)
				}
			})
		}

		// Replay path: every successful (re-)connect re-subscribes the
		// hub's current key snapshot.  Looking up the source again as
		// ReconnectNotifiable keeps the Subscribable / ReconnectNotifiable
		// pair decoupled — a future source could implement only one.
		for _, s := range srcs {
			rn, ok := s.(source.ReconnectNotifiable)
			if !ok {
				continue
			}
			rn.SetOnReconnected(func() {
				// One SubscribeReq carries a single agent_open_id, so we
				// replay one batch per agent: master-only / unrestricted
				// keys group under "" (empty agent), agent-scoped keys
				// group under their agent_open_id.
				byAgent := b.hub.SubscribedKeysByAgent()
				if len(byAgent) == 0 {
					return
				}
				// Prefer batch when the source supports it (saves N
				// roundtrips on a noisy reconnect after many consumers
				// have piled up).  WSSource is the only batch-capable
				// source today; the type-assert keeps room for sources
				// that only implement single-key Subscribe.
				batch, batchOK := upstream.(interface {
					SubscribeBatch(ctx context.Context, eventKeys []string, agentOpenID string) error
				})
				for agentOpenID, keys := range byAgent {
					if len(keys) == 0 {
						continue
					}
					if batchOK {
						if err := batch.SubscribeBatch(ctx, keys, agentOpenID); err != nil {
							b.logger.Warnf(ctx, "bus: upstream replay (batch, agent=%q) failed: %v", agentOpenID, err)
						}
						continue
					}
					for _, k := range keys {
						if err := upstream.Subscribe(ctx, k, agentOpenID); err != nil {
							b.logger.Warnf(ctx, "bus: upstream replay(%s, agent=%q) failed: %v", k, agentOpenID, err)
						}
					}
				}
			})
		}
	}

	for _, src := range srcs {
		go func(s source.Source) {
			b.logger.Infof(ctx, "bus: starting source: %s", s.Name())
			err := s.Run(ctx,
				func(raw *eventruntime.RawEvent) {
					if raw == nil {
						return
					}
					// Drop duplicates from a WSS replay.
					// dedup.Seen is O(1) under a single mutex; a duplicate
					// just returns without entering the hub fan-out.
					if b.dedup.Seen(raw.TraceID) {
						return
					}
					b.hub.Publish(raw)
				},
				func(state, detail string) {
					b.hub.BroadcastSourceStatus(s.Name(), state, detail)
				},
			)
			if ctx.Err() != nil {
				return
			}
			if err != nil {
				b.logger.Warnf(ctx, "bus: source %s exited with error: %v — shutting down bus", s.Name(), err)
			} else {
				b.logger.Infof(ctx, "bus: source %s exited cleanly before shutdown — shutting down bus", s.Name())
			}
			select {
			case b.shutdownCh <- struct{}{}:
			default:
			}
		}(src)
	}
}

// acceptLoop accepts connections until the listener is closed.
func (b *Bus) acceptLoop(ctx context.Context) {
	for {
		conn, err := b.listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			select {
			case <-ctx.Done():
				return
			default:
			}
			b.logger.Warnf(ctx, "bus: accept error: %v", err)
			return
		}
		go b.handleConn(conn)
	}
}

// signalShutdown is exported (lowercase but called from conn.go) to let
// in-package handlers ask for an orderly exit.
func (b *Bus) signalShutdown() {
	select {
	case b.shutdownCh <- struct{}{}:
	default:
	}
}
