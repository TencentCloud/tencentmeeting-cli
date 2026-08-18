// wssource.go — Tencent Meeting WSS event source.
//
// WSSource is the production counterpart to MockSource: it dials a WSS
// endpoint, performs an AuthBindReq handshake, accepts SUBSCRIBE
// requests from the bus, decodes inbound push frames into RawEvents,
// and emits them through the bus's emit callback.  All connection-
// management complexity (TLS / handshake / heartbeat / reconnect)
// lives here so the bus and its hub stay protocol-agnostic.
//
// Wire format
// ===========
//
// All frames are protobuf-encoded ConnMsg envelopes (binary WS frames):
//
//	cmd_type=0  upstream req   (auth.bind / event.subscribe / auth.refresh)
//	cmd_type=1  upstream rsp   (paired by msg_id with a prior cmd_type=0)
//	cmd_type=2  downstream push (carries event payload as Data)
//	cmd_type=3  downstream ack  (we send these for every push; correlates
//	                             by msg_id so the gateway can confirm
//	                             at-least-once delivery)
//
// See internal/event/protocol/wsspb for the codec helpers.
//
// Auth + Subscribe lifecycle
// ==========================
//
// 1. Dial(ctx, url, headers).  AuthHook (if set) decorates the request
//    with custom headers.  401/403 responses become a fatal authError.
// 2. Send AuthBindReq immediately; wait for matching AuthBindRsp.
//    Non-zero AuthBindRsp.ret_code == fatal authError; the bus daemon
//    exits.
// 3. Fire OnReconnected (if set).  The bus uses this to drive Replay:
//    it calls Subscribe(currentSnapshot) so the gateway knows which
//    EventKeys this freshly-connected session cares about.
// 4. Read loop: every incoming frame is decoded via pbFrameDecoder.
//    Push frames produce a RawEvent + an automatic ack write.  Rsp
//    frames complete a pending Subscribe / AuthBind future.
// 5. On any error/close, tear down ping ticker and inflight futures,
//    return.  Run's outer loop classifies the error and reconnects.
//
// Reconnect is exponential with cap.  We DO NOT exit Run() on a
// transient network error by default; instead we transition to
// "reconnecting", sleep, and retry.  Run() returns only on ctx cancel
// OR an unrecoverable error (auth_failed, malformed handshake
// response, token-refresh failure).
//
// Optional circuit breaker: when MaxConsecutiveFailures > 0, Run()
// gives up after that many back-to-back failed runOnce attempts and
// returns the last error.  The counter resets whenever a runOnce
// stayed connected for more than 60s, so a long-lived session that
// occasionally blips is not penalised.  Default 0 keeps the legacy
// "retry forever" behaviour for callers that don't opt in.

package source

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"

	eventruntime "tmeet/internal/event"
	"tmeet/internal/event/protocol"
	"tmeet/internal/event/protocol/wsspb"
	"tmeet/internal/exception"
	tlog "tmeet/internal/log"
)

// FrameDecoder converts a single WS frame (text or binary) into a RawEvent.
//
// Returning (nil, nil) is permitted for "skip this frame silently"
// (heartbeat-style protocol noise the framework can't filter out).
// Returning (nil, err) logs a WARN and continues; the frame is dropped.
//
// Note: the production decoder used by Tencent Meeting is
// DecodePbPushFrame (binary frames carrying ConnMsg push); the legacy
// DecodeNDJSONFrame is retained for tests / dev mocks that prefer JSON.
type FrameDecoder func(messageType int, data []byte) (*eventruntime.RawEvent, error)

// AuthHook customises the dial-time HTTP request.  Receives the request
// gorilla/websocket is about to upgrade so callers can set Authorization,
// X-Forwarded-For, custom headers, etc.  Called once per Dial attempt
// (i.e. each reconnect re-runs it, in case tokens have rotated).
type AuthHook func(*http.Request) error

// Defaults.  Conservative tuning until production load shapes are known.
const (
	defaultHandshakeTimeout = 10 * time.Second
	defaultReadTimeout      = 5 * time.Minute
	// defaultHeartbeatInterval is the bootstrap cadence used for the very
	// first business-layer heartbeat sent right after AuthBind.  Once a
	// HeartRsp arrives, its heart_interval becomes the authoritative
	// schedule and this value is no longer used for the live session.
	defaultHeartbeatInterval = 25 * time.Second
	defaultMinBackoff        = 1 * time.Second
	defaultMaxBackoff        = 30 * time.Second
	defaultRspTimeout        = 10 * time.Second
	wsSourceName             = "wss"
)

// WSSource is a Source backed by a single long-lived WebSocket connection.
//
// All public fields are written once before Run() is called.  conn,
// pendingRsps, etc. are managed under writeMu by the connect/run goroutine.
type WSSource struct {
	URL              string
	HandshakeTimeout time.Duration
	ReadTimeout      time.Duration
	// PingPeriod is the bootstrap interval used for the FIRST business-layer
	// heartbeat right after AuthBind succeeds.  Subsequent heartbeats are
	// driven by HeartRsp.heart_interval returned by the gateway.  Kept
	// named PingPeriod for backward compatibility with existing callers.
	PingPeriod time.Duration
	MinBackoff time.Duration
	MaxBackoff time.Duration

	// MaxConsecutiveFailures bounds how many back-to-back runOnce
	// failures Run() tolerates before giving up and returning the last
	// error.  A runOnce that stayed connected for more than 60s resets
	// the counter (so an established session that occasionally blips
	// is not penalised).  Zero (the default) disables the breaker —
	// Run() retries forever until ctx cancels or it hits a fatal
	// auth/server-closed error.
	MaxConsecutiveFailures int

	// Token / OpenID / CLIUniqID feed the AuthBindReq sent immediately
	// after the WS handshake.  All three are required: an empty Token
	// would never pass gateway auth; OpenID identifies the calling user
	// and is also embedded inside CLIUniqID by the bus daemon (the same
	// "<openId>*<machineId>" value used in the REST proxy's
	// Tmeet-Unique-ID header) so the gateway can correlate the two
	// transports for a single CLI instance.  All are validated in
	// applyDefaults; an empty value yields a fatal config error from
	// Run().
	//
	// Token is the INITIAL access-token used for the first AuthBindReq.
	// At runtime the token can rotate (A3 — RefreshToken before each
	// reconnect / heartbeat); the live value is held in tokenPtr below
	// and accessed via loadToken() / storeToken().  Callers MUST NOT
	// read the Token field after Run() has started — it is frozen at
	// applyDefaults time and only kept exported for construction-time
	// assertions in tests (see build_test.go).
	Token     string
	OpenID    string
	CLIUniqID string

	// tokenPtr holds the live access-token for the running session.
	// Seeded from Token in applyDefaults; rewritten by the heartbeat
	// path when TokenProvider returns a refreshed value (A3) and read
	// by doAuthBind / future AuthRefresh paths.  Atomic.Pointer keeps
	// reads/writes lock-free and gives the race detector an explicit
	// happens-before across goroutines (the runOnce main goroutine
	// reads in doAuthBind; the heartbeatLoop child goroutine writes).
	tokenPtr atomic.Pointer[string]

	// AuthHook is invoked just before each Dial; nil = no extra headers.
	AuthHook AuthHook

	// TokenProvider, if set, is invoked twice per session lifecycle:
	//
	//   1. Before each (re)connect, so the AuthBindReq carries the
	//      freshest access-token (matters when the previous session
	//      was killed by token expiry — without a refresh here the
	//      reconnect would just AuthBind with the same dead token and
	//      flap forever).
	//   2. Before each heartbeat, so a token rotation that happens
	//      mid-session is propagated to the gateway via AuthRefreshReq
	//      BEFORE the next ping (gateway tracks the last-bound token
	//      per session and would otherwise close the conn the moment
	//      the old token's TTL elapses).
	//
	// Implementations should be cheap on the hot path: TmeetAuth's
	// RefreshToken is a no-op when the cached token is still valid.
	// A nil TokenProvider keeps the legacy behaviour (token frozen at
	// construction time) — used by tests and by the MockSource fork.
	TokenProvider func(ctx context.Context) (string, error)

	// OnAuthFailed, when non-nil, is invoked at most once per Run()
	// lifetime IFF the gateway rejected AuthBindReq at the envelope
	// layer with a token-expiry status code (Head.Status ==
	// ServerCodeWssTokenExpired or ServerCodeWssHeadTokenExpired).
	// It is the bus daemon's chance to wipe the now-unusable
	// credentials from the keychain so the next `tmeet event consume`
	// invocation cleanly re-prompts login instead of looping on a
	// token the gateway has already declared dead.
	//
	// Scope is deliberately narrow — we DO NOT call it for:
	//   - Other envelope-level Head.Status codes (e.g. 10004 "token
	//     invalid"; the credential may still be valid for other
	//     endpoints or the rejection may be transient),
	//   - AuthBindRsp.ret_code != 0 (business-layer reject; the
	//     credential may still be valid elsewhere),
	//   - HTTP 401/403 on dial (network layer; could be a misrouted
	//     request, a reverse-proxy hiccup, etc.),
	//   - TokenProvider / refresh-token failures (already covered by
	//     RefreshToken's own ClearUserConfigUnResource hook).
	// Only the specific token-expiry codes are treated as the
	// authoritative "this token is dead at the gateway" signal.
	//
	// code carries Head.Status; err is the underlying *authError.
	// A panicking hook is logged via Logger but does NOT alter Run's
	// return value — the daemon is exiting either way and a failed
	// keychain wipe is recoverable on next login.
	OnAuthFailed func(ctx context.Context, code int, err error)

	// Decoder converts an inbound WS frame into a RawEvent.  Defaults to
	// DecodePbPushFrame when nil.
	Decoder FrameDecoder

	// Headers is a static set of headers added to every Dial request.
	// AuthHook runs AFTER these so dynamic headers can override.
	Headers http.Header

	// reconnects / connectedSince are atomic counters surfaced through
	// ws.state for `event status` diagnostics.
	reconnects     atomic.Int64
	connectedSince atomic.Int64 // unix nanos; 0 when not connected

	// onReconnected is invoked synchronously after every successful
	// AuthBind handshake (including the very first connect).  The bus
	// installs this via SetOnReconnected to drive Replay.  Calls run on
	// the connect goroutine so a slow callback delays the start of the
	// read loop \u2014 keep them quick.
	onReconnected func()

	// OnSubscribeResult is invoked from a background watcher goroutine
	// AFTER each Subscribe / SubscribeBatch upstream call, once the
	// matching SubscribeRsp arrives (or times out / the session dies).
	//
	// Contract:
	//   - Called with code == 0 and empty msg on success (no-op for the
	//     bus today, but kept symmetric so future diagnostics can hook
	//     in without touching this layer).
	//   - Called with code != 0 and SubscribeRsp.msg on a gateway-side
	//     rejection \u2014 the bus uses this to route a subscribe_error
	//     control frame to the affected consumers.
	//   - NOT called when Subscribe / SubscribeBatch returns a write
	//     error: the upstream call never made it onto the wire so there
	//     is nothing to wait for.  Callers receiving the write error
	//     are responsible for whatever surfacing they need.
	//
	// nil disables the callback (tests / MockSource never wire it).
	// The callback runs on a watcher goroutine spawned per upstream
	// call; implementations must be goroutine-safe and quick (a slow
	// callback piles up watcher goroutines on a noisy Replay burst).
	OnSubscribeResult func(eventKeys []string, code uint32, msg string)

	// Logger, when non-nil, receives best-effort lifecycle diagnostics
	// (currently: AuthBind success, SubscribeRsp outcomes).  The bus daemon
	// wires this to the bus *tlog.Logger via factory.Build so connection
	// handshake progress is visible alongside hub / conn events without
	// dragging the source package into a dependency on the bus package.
	// nil is tolerated — unit tests routinely leave it unset — because
	// *tlog.Logger's formatted helpers are nil-receiver safe.
	Logger *tlog.Logger

	// seq generates monotonic seq_no for outbound frames.  Shared across
	// AuthBind / Subscribe / Ack writes so the server sees a single
	// well-ordered upstream sequence.
	seq wsspb.SeqGen

	// writeMu serialises ALL outbound WS writes.  gorilla/websocket's
	// Conn.WriteMessage is documented as not safe for concurrent use,
	// and we have at least three writers: the ping ticker, the read
	// loop's auto-ack, and Subscribe() invoked by the bus from arbitrary
	// goroutines.
	writeMu sync.Mutex

	// connState couples the live conn pointer with the inflight rsp
	// futures.  Replaced wholesale on every dial; reads should snapshot
	// the pointer once before use.
	connState atomic.Pointer[connSession]
}

// connSession is the per-dial mutable state.  Reset on every new dial.
type connSession struct {
	conn    *websocket.Conn
	mu      sync.Mutex // protects pending
	pending map[string]chan *wsspb.ConnMsg
	closed  bool

	// fatalErr, when non-nil, holds a session-scoped fatal error that
	// runOnce should return INSTEAD of whatever readLoop surfaces after
	// the conn was force-closed.  The heartbeat goroutine sets this
	// before closing the conn so an authError raised by the in-session
	// token-refresh path can short-circuit Run()'s reconnect loop
	// (otherwise readLoop just reports a generic "use of closed
	// connection" and Run() would happily retry).
	fatalErr atomic.Pointer[error]
}

// SetOnReconnected installs the bus-supplied Replay callback.  Implements
// source.ReconnectNotifiable.  Single-shot setter \u2014 calling it after
// Run has started is allowed but the next reconnect is when the new
// callback first fires.
func (s *WSSource) SetOnReconnected(fn func()) { s.onReconnected = fn }

// SetOnSubscribeResult installs the bus-supplied subscribe-rsp watcher
// callback.  Implements source.SubscribeResultNotifiable.  Calling it
// after Run has started is allowed; future Subscribe calls pick up the
// new callback (in-flight watchers keep the snapshot they were spawned
// with, which is fine because we only ever swap nil \u2192 real callback at
// startup).
func (s *WSSource) SetOnSubscribeResult(fn func(eventKeys []string, code uint32, msg string)) {
	s.OnSubscribeResult = fn
}

// Subscribe sends a SubscribeReq for a single eventKey.  Implements
// source.Subscribable.
//
// Concurrency: callers can invoke this from any goroutine; we serialise
// on writeMu and do not block the caller waiting for SubscribeRsp \u2014 a
// Replay burst could otherwise stack up rsp-wait timeouts and starve the
// read loop.  Instead we register a pending future BEFORE the write and
// hand the wait off to a background watcher goroutine that surfaces the
// gateway's verdict via OnSubscribeResult (the bus uses that hook to
// fan out a subscribe_error control frame to the affected consumers).
//
// If the connection is not yet established (Run is between dials), we
// drop the subscribe silently: the next reconnect's onReconnected
// callback will Replay the full snapshot anyway, so there's nothing
// useful to retry here.
func (s *WSSource) Subscribe(ctx context.Context, eventKey, agentOpenID string) error {
	if eventKey == "" {
		return exception.InvalidArgsError.With("wssource: empty eventKey")
	}
	return s.subscribeKeys(ctx, []string{eventKey}, agentOpenID)
}

// SubscribeBatch sends a SubscribeReq carrying multiple eventKeys in a
// single frame.  Used by the Replay path to coalesce all keys into one
// upstream call rather than firing N individual frames.  agentOpenID is
// the (single) agent the batch subscribes on behalf of — the bus groups
// keys by agent before calling this so one batch never mixes agents.
func (s *WSSource) SubscribeBatch(ctx context.Context, eventKeys []string, agentOpenID string) error {
	if len(eventKeys) == 0 {
		return nil
	}
	return s.subscribeKeys(ctx, eventKeys, agentOpenID)
}

// subscribeKeys is the shared body of Subscribe / SubscribeBatch.
//
// Lifecycle of one upstream subscribe:
//
//  1. Encode a SubscribeReq carrying eventKeys, capture msg_id.
//  2. Register a pending future on the live session keyed by msg_id so
//     dispatchPb \u2192 deliverRsp can route the inbound SubscribeRsp back
//     to us.  Done BEFORE the write to avoid a window where the rsp
//     could arrive before we registered.
//  3. Write the frame under writeMu.  On write failure: cancel the
//     future and surface the error to the caller \u2014 no watcher needed
//     because nothing is in flight upstream.
//  4. Spawn a watcher goroutine that waits for the future (bounded by
//     defaultRspTimeout), decodes SubscribeRsp, and invokes
//     OnSubscribeResult.  We deliberately do NOT block the caller: hub
//     fan-out / Replay must stay responsive even if the gateway is slow.
func (s *WSSource) subscribeKeys(ctx context.Context, eventKeys []string, agentOpenID string) error {
	sess := s.connState.Load()
	if sess == nil {
		return errors.New("wssource: not connected (yet); subscribe will be retried on reconnect Replay")
	}

	wire, msgID, err := wsspb.EncodeSubscribeReq(&s.seq, eventKeys, agentOpenID)
	if err != nil {
		return fmt.Errorf("wssource: encode subscribe: %w", err)
	}

	// Register the pending future BEFORE the write so a fast gateway
	// can't deliver the rsp before we're listening for it.
	ch := sess.registerPending(msgID)
	if ch == nil {
		return errors.New("wssource: session closed before subscribe register")
	}

	s.writeMu.Lock()
	if sess.isClosed() {
		s.writeMu.Unlock()
		sess.cancelPending(msgID)
		return errors.New("wssource: session closed mid-subscribe")
	}
	_ = sess.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	werr := sess.conn.WriteMessage(websocket.BinaryMessage, wire)
	s.writeMu.Unlock()
	if werr != nil {
		// Nothing made it onto the wire \u2014 tear the future down so we
		// don't leak an entry and don't trigger a spurious "rsp timeout"
		// callback later.
		sess.cancelPending(msgID)
		return werr
	}

	// Hand the wait off to a watcher goroutine.  Snapshot the keys so a
	// caller-owned slice mutation can't race with the eventual callback.
	keysCopy := append([]string(nil), eventKeys...)
	go s.awaitSubscribeRsp(ctx, sess, msgID, keysCopy, ch)
	return nil
}

// awaitSubscribeRsp blocks the watcher goroutine on a single SubscribeRsp.
// On any terminal outcome (rsp arrived / timeout / ctx cancel / session
// closed) it invokes OnSubscribeResult exactly once, then exits.
//
// Treatment of the four exit paths:
//
//   - rsp received with code == 0   \u2192 success callback (no-op for the
//     bus today, but kept symmetric).
//   - rsp received with code != 0   \u2192 error callback carrying code +
//     SubscribeRsp.msg verbatim; the bus translates this into a
//     subscribe_error control frame routed to consumers of these keys.
//   - defaultRspTimeout elapsed     \u2192 error callback with code=0 and a
//     synthetic "rsp timeout" message; consumers exit just as they
//     would on a real rejection (the gateway is unresponsive, so the
//     pessimistic assumption is appropriate).
//   - ctx cancel / session torn down \u2192 silent: the bus is shutting down
//     OR a reconnect will Replay these keys, so a callback would be
//     redundant noise.
func (s *WSSource) awaitSubscribeRsp(ctx context.Context, sess *connSession,
	msgID string, eventKeys []string, ch chan *wsspb.ConnMsg) {
	// Always cancel the pending entry: if the rsp arrived we want to
	// drop it from the map; if we exit via timeout / ctx, deliverRsp
	// would otherwise try to send into a chan no one reads.
	defer sess.cancelPending(msgID)

	timer := time.NewTimer(defaultRspTimeout)
	defer timer.Stop()

	cb := s.OnSubscribeResult

	select {
	case <-ctx.Done():
		// Bus / source shutting down; nothing useful to report.
		return
	case <-timer.C:
		s.Logger.Warnf(ctx, "wssource: SubscribeRsp timeout msgID=%s keys=%v", msgID, eventKeys)
		if cb != nil {
			cb(eventKeys, 0, "subscribe rsp timeout")
		}
		return
	case cm := <-ch:
		if cm == nil {
			// Session torn down by markClosed.  The next reconnect's
			// Replay will retry; emitting a callback here would race
			// with the source_status=disconnected broadcast and just
			// confuse the consumer.
			return
		}

		rsp, derr := wsspb.DecodeSubscribeRsp(cm)
		if derr != nil {
			s.Logger.Warnf(ctx, "wssource: SubscribeRsp decode error msgID=%s keys=%v err=%v",
				msgID, eventKeys, derr)
			if cb != nil {
				cb(eventKeys, 0, "decode SubscribeRsp: "+derr.Error())
			}
			return
		}
		s.Logger.Infof(ctx, "wssource: SubscribeRsp msgID=%s keys=%v code=%d msg=%v has_sub_list=%v",
			msgID, eventKeys, rsp.GetCode(), rsp.GetMsg(), rsp.GetData().GetHasSubEventList())
		if cb != nil {
			cb(eventKeys, rsp.GetCode(), rsp.GetMsg())
		}
	}
}

// Name implements Source.
func (s *WSSource) Name() string { return wsSourceName }

// Run drives the connect \u2192 auth \u2192 read \u2192 reconnect loop until ctx is
// cancelled or an unrecoverable error is hit.
//
// Returns nil on graceful ctx cancellation.  Returns an error only when
// the caller passed a fundamentally invalid configuration OR auth was
// rejected (which is non-recoverable until the user re-logs in).
func (s *WSSource) Run(ctx context.Context, emit func(*eventruntime.RawEvent), notify StatusNotifier) error {
	if s.URL == "" {
		return errors.New("wssource: empty URL")
	}
	if _, err := url.Parse(s.URL); err != nil {
		return fmt.Errorf("wssource: parse URL: %w", err)
	}
	// Auth credentials are required for the production pb path; tests that
	// wire a custom Decoder (legacy NDJSON mocks) opt out of the AuthBind
	// handshake entirely, so we only enforce them when no Decoder is
	// supplied.
	if s.Decoder == nil {
		if s.Token == "" {
			return errors.New("wssource: empty Token (refusing to dial without credentials)")
		}
		if s.OpenID == "" {
			return errors.New("wssource: empty OpenID")
		}
		if s.CLIUniqID == "" {
			return errors.New("wssource: empty CLIUniqID")
		}
	}

	s.applyDefaults()

	backoff := s.MinBackoff
	firstAttempt := true
	// consecutiveFailures tracks back-to-back transient runOnce failures
	// for the MaxConsecutiveFailures circuit breaker.  Reset whenever a
	// runOnce stayed connected for more than 60s (the same heuristic
	// used to reset backoff).
	consecutiveFailures := 0

	for {
		if ctx.Err() != nil {
			s.notifyAndPersist(notify, protocol.SourceStateDisconnected, "ctx cancelled")
			return nil
		}

		if firstAttempt {
			s.notifyAndPersist(notify, protocol.SourceStateConnecting, "dialing "+s.safeURL())
			firstAttempt = false
		} else {
			s.notifyAndPersist(notify, protocol.SourceStateReconnecting,
				"backoff "+backoff.String()+" then retry")
			s.reconnects.Add(1)
			if !sleepCtx(ctx, backoff) {
				s.notifyAndPersist(notify, protocol.SourceStateDisconnected, "ctx cancelled during backoff")
				return nil
			}
			backoff = nextBackoff(backoff, s.MaxBackoff)
		}

		err, sessionDur := s.runOnce(ctx, emit, notify)
		// sessionDur is the duration the just-finished runOnce stayed in
		// the steady state (post-AuthBind).  Returned directly by runOnce
		// rather than read off a shared field so a failed dial / auth on
		// the NEXT runOnce cannot accidentally inherit the previous
		// session's stability and wrongly reset the backoff /
		// consecutiveFailures counters.
		wasStable := sessionDur > 60*time.Second
		switch {
		case ctx.Err() != nil:
			s.notifyAndPersist(notify, protocol.SourceStateDisconnected, "ctx cancelled")
			return nil
		case isAuthError(err):
			var statusErr *wsspb.UpstreamRspStatusError
			if errors.As(err, &statusErr) &&
				(statusErr.Status == exception.ServerCodeWssTokenExpired ||
					statusErr.Status == exception.ServerCodeWssHeadTokenExpired) {
				s.notifyAndPersist(notify, protocol.SourceStateAuthExpired,
					"auth expired: "+err.Error())

				if s.OnAuthFailed != nil {
					func() {
						defer func() {
							if r := recover(); r != nil {
								s.Logger.Warnf(ctx, "wssource: OnAuthFailed hook panicked: %v", r)
							}
						}()
						s.OnAuthFailed(ctx, int(statusErr.Status), err)
					}()
				}
			} else {
				s.notifyAndPersist(notify, protocol.SourceStateAuthFailed,
					"auth rejected: "+err.Error())
			}
			return err
		case isServerClosed(err):
			// Server-initiated WebSocket Close frame (any close code).
			// Per protocol contract the gateway never sends Close on
			// graceful shutdown / restart (those are surfaced as silent
			// link failures the heartbeat loop detects), so an explicit
			// Close frame is treated as an authoritative "do not reconnect"
			// signal regardless of the code carried.
			s.notifyAndPersist(notify, protocol.SourceStateDisconnected,
				"server closed connection: "+err.Error())
			return err
		case err != nil:
			s.notifyAndPersist(notify, protocol.SourceStateReconnecting, "lost: "+err.Error())
			if wasStable {
				// A long-lived session that just hiccuped — fully
				// restart the breaker so we don't penalise legitimate
				// network blips.
				backoff = s.MinBackoff
				consecutiveFailures = 0
			} else {
				consecutiveFailures++
				if s.MaxConsecutiveFailures > 0 &&
					consecutiveFailures >= s.MaxConsecutiveFailures {
					s.notifyAndPersist(notify, protocol.SourceStateDisconnected,
						fmt.Sprintf("giving up after %d consecutive failures: %s",
							consecutiveFailures, err.Error()))
					return fmt.Errorf("wssource: %d consecutive failures, last error: %w",
						consecutiveFailures, err)
				}
			}
		default:
			// nil err means runOnce exited because ctx cancelled mid-session.
		}
	}
}

// runOnce performs a single dial + auth + read loop.  Returns when the
// conn closes for any reason.  Auth failures are surfaced as *authError
// so Run() can short-circuit retry.
//
// The second return value is how long this call stayed in the steady
// state (post-AuthBind) before tearing down — zero when the call never
// reached steady state (dial failed, auth rejected, token refresh
// failed, etc).  Returned per-call rather than via a shared field on
// the receiver so a subsequent runOnce that fails before steady state
// cannot accidentally inherit the previous session's duration and
// trick Run() into thinking it was stable.
func (s *WSSource) runOnce(ctx context.Context, emit func(*eventruntime.RawEvent), notify StatusNotifier) (retErr error, sessionDur time.Duration) {
	// Refresh the access-token before dialling so the AuthBindReq we'll
	// send right after the WS handshake carries the freshest value.
	// A no-op when the cached token is still valid (TmeetAuth.RefreshToken
	// short-circuits on Expires > now).  A failure here is fatal-ish:
	// without a token the dial would just AuthBind with the empty / stale
	// value and bounce on auth_failed.  We mirror the REST proxy's policy
	// (proxy.RequestProxy returns immediately when auth.RefreshToken
	// fails) and treat ANY refresh failure as fatal: the TokenProvider
	// is responsible for distinguishing "transient network blip on the
	// refresh endpoint" from "refresh-token expired" internally and
	// returning success on the former.  A failure surfacing here means
	// the credential is unusable, so we bubble it up as authError to
	// short-circuit Run()'s reconnect loop instead of flapping forever
	// on a dead token.
	if err := s.refreshAndStoreToken(ctx); err != nil {
		return &authError{code: 0, err: fmt.Errorf("refresh token before dial: %w", err)}, 0
	}

	dialer := &websocket.Dialer{
		HandshakeTimeout: s.HandshakeTimeout,
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.URL, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err), 0
	}
	for k, vs := range s.Headers {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	if s.AuthHook != nil {
		if err := s.AuthHook(req); err != nil {
			return fmt.Errorf("auth hook: %w", err), 0
		}
	}

	conn, resp, err := dialer.DialContext(ctx, req.URL.String(), req.Header)
	if err != nil {
		if resp != nil {
			if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
				return &authError{code: resp.StatusCode, err: err}, 0
			}
		}
		return fmt.Errorf("dial: %w", err), 0
	}
	defer func() { _ = conn.Close() }()

	// Establish per-session state.  We deliberately do NOT publish sess
	// to s.connState yet.  Two scenarios must be served correctly:
	//
	//   1. First connect, AuthBind in flight: an external Subscribe()
	//      call (e.g. hub 0→1 transition fired by a fresh consumer's
	//      Hello) MUST NOT write a SubscribeReq onto a connection that
	//      hasn't completed AuthBind — the gateway would silently drop
	//      it.  Such early Subscribes need to see connState == nil so
	//      they take the "will be retried on reconnect Replay" branch;
	//      the onReconnected() callback fired below right after AuthBind
	//      success then sweeps the hub's current key snapshot via
	//      SubscribeBatch and the Subscribe is delivered for real.
	//
	//   2. Steady state (AuthBind already ok): Subscribe() sees a non-nil
	//      sess and writes the SubscribeReq directly — the normal path.
	//
	// Publishing sess only after doAuthBind succeeds is the single switch
	// that makes (1) and (2) correct under the same connState.Load() test.
	sess := &connSession{
		conn:    conn,
		pending: make(map[string]chan *wsspb.ConnMsg),
	}
	sessPublished := false
	defer func() {
		sess.markClosed()
		if sessPublished {
			s.connState.Store((*connSession)(nil))
		}
	}()

	// ctx-cancel watcher.
	closeOnCancel := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = conn.Close()
		case <-closeOnCancel:
		}
	}()
	defer close(closeOnCancel)

	// Heartbeat.
	//
	// We rely entirely on the business-layer heartbeat (cmd=/conn/ping)
	// for liveness: gorilla's WS-level ping/pong is not used because the
	// gateway drives cadence via HeartRsp.heart_interval, and mixing the
	// two would mask server-side issues with client-side ping replies.
	// SetPongHandler is therefore not installed; instead each successful
	// HeartRsp resets the read deadline below.
	_ = conn.SetReadDeadline(time.Now().Add(s.ReadTimeout))

	// Send AuthBindReq + wait for AuthBindRsp.  Failure here is fatal:
	// either the token is invalid (auth_failed) or the gateway is
	// uncommunicative (transient — reconnect via Run loop).
	//
	// Tests that wire a custom Decoder (legacy NDJSON mocks) skip this
	// step — they're targeting unit tests of the read/reconnect loop
	// rather than the auth path.  See Run for the matching gating.
	if s.Decoder == nil {
		if err := s.doAuthBind(ctx, sess); err != nil {
			return err, 0
		}
	}

	// AuthBind has succeeded (or was skipped for the NDJSON mock path) —
	// only NOW do we publish sess so external Subscribe / SubscribeBatch
	// callers can write frames.  Anything that arrived earlier saw
	// connState == nil and took the "retry on reconnect Replay" branch,
	// which is exactly what the onReconnected callback below drives.
	s.connState.Store(sess)
	sessPublished = true

	// AuthBind success is the single most important lifecycle event for
	// the WS session — emit one line into bus.log so an operator can
	// confirm that a freshly-spawned bus actually finished the handshake
	// (and on reconnects, when each one completes).  We log only for the
	// real protobuf path; the NDJSON test path has no AuthBind to report.
	if s.Decoder == nil {
		s.Logger.Infof(ctx, "wssource: AuthBind success openID=%s cliUniqID=%s", s.OpenID, s.CLIUniqID)
	}

	steadySince := time.Now()
	s.connectedSince.Store(steadySince.UnixNano())
	s.notifyAndPersist(notify, protocol.SourceStateSteady, "connected")
	defer func() {
		// Snapshot the session duration BEFORE clearing connectedSince
		// so the named return value reflects how long we stayed in the
		// steady state.  Only set when we actually reached steady state
		// (the early-return paths above leave sessionDur as zero).
		sessionDur = time.Since(steadySince)
		s.connectedSince.Store(0)
	}()

	// Replay: bus-supplied callback re-subscribes the current key set.
	if cb := s.onReconnected; cb != nil {
		cb()
	}

	// Heartbeat goroutine.  Started AFTER AuthBind succeeds so the very
	// first /conn/ping carries an authenticated session.  Skipped for
	// the legacy NDJSON Decoder path (tests).
	hbDone := make(chan struct{})
	hbStop := make(chan struct{})
	if s.Decoder == nil {
		go s.heartbeatLoop(ctx, sess, hbStop, hbDone, notify)
		defer func() {
			select {
			case <-hbStop:
			default:
				close(hbStop)
			}
			<-hbDone
		}()
	} else {
		close(hbDone)
	}

	readErr := s.readLoop(ctx, sess, emit, notify)
	// If the heartbeat goroutine pinned a fatal error before forcing
	// the conn shut, prefer it over readLoop's generic "use of closed
	// network connection" / EOF.  This is what propagates the
	// in-session token-refresh failure as authError up to Run() so the
	// reconnect loop can short-circuit instead of flapping forever.
	//
	// We use bare returns below so the deferred block above is the
	// single authoritative writer of sessionDur — writing ", 0" here
	// would be misleading since the defer would overwrite it anyway.
	if fatal := sess.loadFatalErr(); fatal != nil {
		retErr = fatal
		return
	}
	retErr = readErr
	return
}

// setFatalErr pins a session-scoped fatal error.  First writer wins —
// later calls are dropped silently (we only care about the first cause
// of the teardown).  Safe to call from any goroutine.
func (s *connSession) setFatalErr(err error) {
	if err == nil {
		return
	}
	s.fatalErr.CompareAndSwap(nil, &err)
}

// loadFatalErr returns the pinned fatal error, or nil when no goroutine
// has set one for this session.
func (s *connSession) loadFatalErr() error {
	if p := s.fatalErr.Load(); p != nil {
		return *p
	}
	return nil
}

// heartbeatLoop drives the business-layer /conn/ping ↔ HeartRsp cadence.
//
// Lifecycle:
//
//  1. Send the first heartbeat IMMEDIATELY after AuthBind succeeds.
//  2. Register a pending future on the session keyed by the heartbeat's
//     msg_id; the readLoop's deliverRsp routes the matching cmd_type=1
//     reply back to us.
//  3. Decode HeartRsp, refresh the read deadline, then sleep for
//     heart_interval seconds before sending the next heartbeat.  If the
//     gateway returned a zero / negative interval, fall back to
//     PingPeriod so we never spin in a tight loop.
//  4. On any write/read error or timeout, close the underlying conn so
//     the readLoop unblocks and runOnce returns; the outer Run() loop
//     will then reconnect.
//
// Exits when ctx cancels, hbStop closes, or the session dies.
func (s *WSSource) heartbeatLoop(ctx context.Context, sess *connSession,
	hbStop <-chan struct{}, hbDone chan<- struct{}, notify StatusNotifier) {

	defer close(hbDone)

	interval := s.PingPeriod
	first := true

	for {
		if !first {
			select {
			case <-ctx.Done():
				return
			case <-hbStop:
				return
			case <-time.After(interval):
			}
		}
		first = false

		if sess.isClosed() {
			return
		}

		nextInterval, err := s.sendHeartbeat(ctx, sess, hbStop)
		if err != nil {
			// hbStop closed mid-flight = orderly teardown, not a
			// failure worth surfacing.
			select {
			case <-hbStop:
				return
			default:
			}
			s.notifyAndPersist(notify, protocol.SourceStateSteady,
				"heartbeat failed: "+err.Error())
			// If sendHeartbeat surfaced a fatal-class error (currently:
			// authError from the in-session token-refresh path), pin it
			// on the session so runOnce returns it verbatim — otherwise
			// Run()'s outer switch only sees the generic "use of closed
			// connection" readLoop reports after we Close() below, and
			// the reconnect loop would happily retry forever on a dead
			// refresh-token.  Non-fatal heartbeat errors (write timeout,
			// rsp timeout, transient network blip) deliberately fall
			// through to the legacy reconnect path.
			if isAuthError(err) {
				sess.setFatalErr(err)
			}
			// Close the conn so readLoop unblocks → runOnce returns →
			// Run() handles the result (reconnect OR short-circuit on
			// the pinned fatalErr).
			_ = sess.conn.Close()
			return
		}
		// HeartRsp doubles as a liveness signal: refresh the read
		// deadline so an idle stream (no business pushes) doesn't trip
		// ReadTimeout.
		_ = sess.conn.SetReadDeadline(time.Now().Add(s.ReadTimeout))
		if nextInterval > 0 {
			interval = nextInterval
		}
	}
}

// sendHeartbeat writes a /conn/ping frame and waits for the matching
// HeartRsp.  Returns the heart_interval the gateway wants us to honour
// for the next round (zero when the gateway omitted it).
func (s *WSSource) sendHeartbeat(ctx context.Context, sess *connSession,
	hbStop <-chan struct{}) (time.Duration, error) {
	// Refresh the access-token before each heartbeat round.  When the
	// cached token is still valid this is a cheap no-op; when it
	// rotated, refreshTokenIfNeeded sends an AuthRefreshReq BEFORE
	// the ping so the gateway re-binds the session to the new token.
	//
	// Failure policy mirrors the dial-time refreshAndStoreToken path
	// (and the REST proxy's auth.RefreshToken handling): a single
	// failure is treated as fatal.  We surface it as *authError so
	// heartbeatLoop tears down the conn AND Run()'s outer loop
	// short-circuits the reconnect — flapping forever on a dead
	// refresh-token would just bury the real error and burn rate
	// limits.  The TokenProvider itself owns the "transient blip vs
	// permanent expiry" distinction; by the time the error reaches
	// us the credential is considered unusable.
	if err := s.refreshTokenIfNeeded(ctx, sess, hbStop); err != nil {
		return 0, &authError{code: 0, err: fmt.Errorf("heartbeat refresh token: %w", err)}
	}

	wire, msgID, err := wsspb.EncodeHeartReq(&s.seq)
	if err != nil {
		return 0, fmt.Errorf("encode heart: %w", err)
	}

	ch := sess.registerPending(msgID)
	if ch == nil {
		return 0, errors.New("heartbeat: session closed before register")
	}
	// Always tear down the future on exit so a slow/lost rsp doesn't
	// leak entries in pending.
	defer sess.cancelPending(msgID)

	s.writeMu.Lock()
	if sess.isClosed() {
		s.writeMu.Unlock()
		return 0, errors.New("heartbeat: session closed mid-write")
	}
	_ = sess.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	werr := sess.conn.WriteMessage(websocket.BinaryMessage, wire)
	s.writeMu.Unlock()
	if werr != nil {
		return 0, fmt.Errorf("write heart: %w", werr)
	}

	// Wait for HeartRsp.  Use the same defaultRspTimeout budget as
	// AuthBind so a stalled gateway is detected within ~10s rather than
	// waiting for the full ReadTimeout.  We also honour hbStop so an
	// orderly runOnce teardown isn't blocked by an in-flight rsp wait.
	timer := time.NewTimer(defaultRspTimeout)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	case <-hbStop:
		return 0, errors.New("heartbeat: stop requested")
	case <-timer.C:
		return 0, errors.New("heartbeat: rsp timeout")
	case cm := <-ch:
		if cm == nil {
			return 0, errors.New("heartbeat: session closed while waiting for rsp")
		}
		rsp, derr := wsspb.DecodeHeartRsp(cm)
		if derr != nil {
			return 0, fmt.Errorf("decode HeartRsp: %w", derr)
		}
		return time.Duration(rsp.GetHeartInterval()) * time.Second, nil
	}
}

// doAuthBind sends AuthBindReq and blocks until the matching rsp comes
// back via the read loop.  Because the read loop is not yet running at
// the call site, we synthesise a minimal one-shot reader inline that
// reads exactly one frame (the rsp) and then returns.
//
// The contract: any frame arriving before AuthBindRsp is a protocol
// violation \u2014 push frames before auth would imply the gateway accepted
// us implicitly, which contradicts the wire spec.  We surface such
// rogue frames as a fatal handshake error.
//
// sess is passed in directly rather than fetched via s.connState.Load()
// because at this point in runOnce the session has been created but not
// yet published to connState — we publish only after AuthBind succeeds
// so external Subscribe() callers can't race in and write a SubscribeReq
// onto an un-authed connection.
func (s *WSSource) doAuthBind(ctx context.Context, sess *connSession) error {
	if sess == nil {
		return errors.New("doAuthBind: no session")
	}

	wire, msgID, err := wsspb.EncodeAuthBindReq(&s.seq, s.loadToken(), s.OpenID, s.CLIUniqID)
	if err != nil {
		return fmt.Errorf("doAuthBind: encode: %w", err)
	}

	s.writeMu.Lock()
	_ = sess.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	err = sess.conn.WriteMessage(websocket.BinaryMessage, wire)
	s.writeMu.Unlock()
	if err != nil {
		return fmt.Errorf("doAuthBind: write: %w", err)
	}

	// Inline one-shot reader.  We can't use the regular readLoop because
	// it is not running yet \u2014 if it were, the rsp would race with our
	// blocking wait below.
	deadline := time.Now().Add(defaultRspTimeout)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	_ = sess.conn.SetReadDeadline(deadline)

	mtype, data, rerr := sess.conn.ReadMessage()
	if rerr != nil {
		// Server-initiated WS Close during the handshake window also
		// counts as "do not reconnect" — bubble up as serverClosedError
		// without the doAuthBind: prefix so Run() can short-circuit.
		if sce := asServerClosed(rerr); sce != nil {
			return sce
		}
		return fmt.Errorf("doAuthBind: read rsp: %w", rerr)
	}
	if mtype != websocket.BinaryMessage {
		return fmt.Errorf("doAuthBind: rsp came on non-binary frame type=%d", mtype)
	}
	cm, derr := wsspb.Decode(data)
	if derr != nil {
		return fmt.Errorf("doAuthBind: decode rsp: %w", derr)
	}
	if cm.Head.Cmd != wsspb.CmdAuthBind || cm.Head.CmdType != wsspb.CmdTypeUpstreamRsp {
		return fmt.Errorf("doAuthBind: unexpected first frame cmd=%q cmd_type=%d",
			cm.Head.Cmd, cm.Head.CmdType)
	}
	if cm.Head.MsgId != msgID {
		// Strict correlation: any other msg_id means the gateway
		// confused us with another session.  Bail rather than
		// risk acting on someone else's auth result.
		return fmt.Errorf("doAuthBind: rsp msg_id=%q != req msg_id=%q",
			cm.Head.MsgId, msgID)
	}
	// Two layers of success signal must both be 0:
	//
	//   1. Head.status (envelope) — checked inside DecodeAuthBindRsp;
	//      surfaces as *wsspb.UpstreamRspStatusError when the gateway
	//      rejected before the auth module ran (Data may be empty).
	//   2. AuthBindRsp.ret_code (body) — finer-grained business signal,
	//      pairs with the gateway's textual Msg for diagnostics.
	//
	// Both routes wrap into authError so Run() short-circuits the
	// reconnect loop instead of churning against a credential the
	// gateway will never accept.
	rsp, derr := wsspb.DecodeAuthBindRsp(cm)
	if derr != nil {
		var statusErr *wsspb.UpstreamRspStatusError
		if errors.As(derr, &statusErr) {
			return &authError{
				code: int(statusErr.Status),
				err:  fmt.Errorf("AuthBind envelope rejected: %w", statusErr),
			}
		}
		return fmt.Errorf("doAuthBind: decode AuthBindRsp body: %w", derr)
	}
	if rsp.RetCode != 0 {
		// Non-zero ret_code from the gateway = fatal auth failure.
		// Wrap in authError so Run() short-circuits the reconnect.
		// rsp.Msg is included verbatim for diagnostics; it never
		// contains the token (the gateway echoes only its own error
		// strings) so it's safe to surface through StatusNotifier.
		return &authError{
			code: int(rsp.RetCode),
			err:  fmt.Errorf("AuthBindRsp.ret_code=%d msg=%q", rsp.RetCode, rsp.Msg),
		}
	}
	// Reset read deadline to the steady-state value.
	_ = sess.conn.SetReadDeadline(time.Now().Add(s.ReadTimeout))
	return nil
}

// readLoop is the steady-state frame reader.  Exits when conn closes.
//
// Frame dispatch:
//
//	cmd_type=1 (rsp)  \u2192 deliver to pending[msg_id] if any; else log + drop
//	cmd_type=2 (push) \u2192 if Head.Cmd == CmdWsCLIPushEvent: decode Data
//	                    (JSON RawEvent) and emit + send back cmd_type=3
//	                    ack; otherwise drop silently (non-business push)
//	cmd_type=3 (ack)  \u2192 unexpected (we never solicit acks); log + drop
//	cmd_type=0 (req)  \u2192 the gateway never initiates upstream-style req
//	                    against the client; log + drop
//
// pbFrameDecoder is invoked for cmd_type=2 to convert the inner Data
// into a RawEvent.  For NDJSON-mode tests that wire a custom Decoder,
// the frame is also routed through the decoder regardless of cmd_type
// (legacy compatibility).
func (s *WSSource) readLoop(ctx context.Context, sess *connSession,
	emit func(*eventruntime.RawEvent), notify StatusNotifier) error {

	dec := s.Decoder
	for {
		mtype, data, rerr := sess.conn.ReadMessage()
		if rerr != nil {
			// A *websocket.CloseError means the server proactively sent a
			// Close control frame.  Per gateway contract this only happens
			// when the server wants the client to stop — never on routine
			// restart / shutdown — so we short-circuit reconnect for ANY
			// close code.  Other read errors (RST/EOF/timeout/heartbeat-
			// triggered Close) fall through to the normal reconnect path.
			if sce := asServerClosed(rerr); sce != nil {
				return sce
			}
			return rerr
		}

		// Custom decoder path: tests wire a Decoder to bypass pb framing.
		if dec != nil {
			ev, derr := dec(mtype, data)
			if derr != nil {
				s.notifyAndPersist(notify, protocol.SourceStateSteady,
					"decode error (skipped): "+derr.Error())
				continue
			}
			if ev != nil {
				emit(ev)
			}
			continue
		}

		// Production pb path.
		if mtype != websocket.BinaryMessage {
			s.notifyAndPersist(notify, protocol.SourceStateSteady,
				fmt.Sprintf("unexpected non-binary frame type=%d (skipped)", mtype))
			continue
		}
		cm, derr := wsspb.Decode(data)
		if derr != nil {
			s.notifyAndPersist(notify, protocol.SourceStateSteady,
				"pb decode error (skipped): "+derr.Error())
			continue
		}
		s.dispatchPb(cm, emit, notify)
	}
}

// dispatchPb routes a decoded ConnMsg by cmd_type.  Pulled out of
// readLoop to keep the read goroutine's body small and to ease testing.
func (s *WSSource) dispatchPb(cm *wsspb.ConnMsg, emit func(*eventruntime.RawEvent),
	notify StatusNotifier) {

	switch cm.Head.CmdType {
	case wsspb.CmdTypeUpstreamRsp:
		// Rsp correlation: deliver to the awaiting future, if any.
		// AuthBind / AuthRefresh / Heartbeat / Subscribe all register
		// pending futures (see doAuthBind, refreshTokenIfNeeded,
		// sendHeartbeat, subscribeKeys); the matching cmd_type=1 reply
		// lands here and is routed back to the awaiting goroutine.
		// Anything that arrives without a registered future is
		// silently dropped \u2014 a stale rsp from a torn-down session
		// would otherwise wedge an unrelated future on collision.
		s.deliverRsp(cm)

	case wsspb.CmdTypeDownstreamPush:
		// Per the wire contract only WsCLIPushEvent carries a business
		// event notification; any other cmd on a cmd_type=2 frame is
		// non-business (reserved control / unknown extension) and is
		// dropped without an ack — acking would tell the gateway we
		// consumed something we never delivered to the bus.
		if cm.Head.Cmd != wsspb.CmdWsCLIPushEvent {
			s.notifyAndPersist(notify, protocol.SourceStateSteady,
				fmt.Sprintf("downstream push dropped: cmd=%q (not %s)",
					cm.Head.Cmd, wsspb.CmdWsCLIPushEvent))
			return
		}
		// Data is a JSON-encoded RawEvent: the inner "event" field is
		// the business EventKey the bus's hub routes by, "trace_id"
		// drives downstream dedup, and "payload" is the per-event
		// schema body.  We treat decode failures as non-fatal so a
		// single malformed event can't kill the stream.
		ev, err := decodePushToRawEvent(cm)
		if err != nil {
			s.notifyAndPersist(notify, protocol.SourceStateSteady,
				"push decode error (skipped): "+err.Error())
			return
		}
		emit(ev)
		// Send ack best-effort; ack write failure is logged but does
		// NOT trigger reconnect (the read loop's next ReadMessage
		// will surface real connection death).  cmd is mirrored from
		// the inbound push so the gateway sees a literal echo.
		if err := s.writeAck(cm.Head.Cmd, cm.Head.MsgId); err != nil {
			s.notifyAndPersist(notify, protocol.SourceStateSteady,
				"ack write failed (continuing): "+err.Error())
		}

	default:
		// cmd_type=0 (req from server) and cmd_type=3 (ack received from
		// server) aren't part of the documented contract.  Log and drop.
		s.notifyAndPersist(notify, protocol.SourceStateSteady,
			fmt.Sprintf("unexpected cmd_type=%d cmd=%q (dropped)",
				cm.Head.CmdType, cm.Head.Cmd))
	}
}

// deliverRsp routes cm to a registered future, if one is waiting.
// Drops silently when no future matches \u2014 e.g. an rsp arriving after the
// awaiter timed out, or a stray frame whose msg_id no caller registered.
func (s *WSSource) deliverRsp(cm *wsspb.ConnMsg) {
	sess := s.connState.Load()
	if sess == nil {
		return
	}
	sess.mu.Lock()
	ch, ok := sess.pending[cm.Head.MsgId]
	if ok {
		delete(sess.pending, cm.Head.MsgId)
	}
	sess.mu.Unlock()
	if !ok {
		return
	}
	// Buffered chan size 1: the writer (us) never blocks; the reader
	// (the awaiting goroutine) gets the rsp on the next select.
	select {
	case ch <- cm:
	default:
	}
}

// writeAck serialises an outbound cmd_type=3 frame.  cmd is mirrored
// verbatim from the corresponding push's Head.Cmd (currently always
// CmdWsCLIPushEvent under the active protocol) so the gateway sees a
// literal echo and can correlate by msg_id.
func (s *WSSource) writeAck(cmd, msgID string) error {
	if msgID == "" {
		return nil // nothing to ack
	}
	sess := s.connState.Load()
	if sess == nil || sess.isClosed() {
		return errors.New("writeAck: no session")
	}
	wire, err := wsspb.EncodeAck(&s.seq, cmd, msgID)
	if err != nil {
		return err
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_ = sess.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	return sess.conn.WriteMessage(websocket.BinaryMessage, wire)
}

// decodePushToRawEvent extracts a RawEvent from a WsCLIPushEvent push.
//
// Wire shape: Head.Cmd == CmdWsCLIPushEvent (caller has already
// filtered out anything else); Data is the JSON encoding of a
// RawEvent: {"event": "<business key>", "trace_id": "...",
// "payload": {...}}.  The inner "event" field is the business
// EventKey the bus's hub routes by — Head.Cmd is no longer the source
// of truth for that.
//
// trace_id falls back to Head.MsgId when absent so downstream dedup
// always has a stable per-push identifier.  We additionally copy Data
// before unmarshal because RawEvent.Payload is a json.RawMessage that
// would otherwise alias gorilla/websocket's read buffer (reused across
// frames).
func decodePushToRawEvent(cm *wsspb.ConnMsg) (*eventruntime.RawEvent, error) {
	if cm == nil || cm.Head == nil {
		return nil, errors.New("decode push: missing head")
	}
	if len(cm.Data) == 0 {
		return nil, errors.New("decode push: empty data")
	}
	if !json.Valid(cm.Data) {
		return nil, errors.New("decode push: data is not valid JSON")
	}
	// Copy because RawEvent.Payload (json.RawMessage) would alias
	// gorilla/websocket's reusable read buffer otherwise.
	buf := make([]byte, len(cm.Data))
	copy(buf, cm.Data)
	var ev eventruntime.RawEvent
	if err := json.Unmarshal(buf, &ev); err != nil {
		return nil, fmt.Errorf("decode push: %w", err)
	}
	if ev.Event == "" {
		return nil, errors.New("decode push: payload missing event field")
	}
	if ev.TraceID == "" {
		ev.TraceID = cm.Head.MsgId
	}
	return &ev, nil
}

// markClosed flips the closed flag and drains pending futures with nil
// (signalling "session gone, don't wait").  Safe to call multiple times.
func (s *connSession) markClosed() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.closed = true
	for _, ch := range s.pending {
		select {
		case ch <- nil:
		default:
		}
	}
	s.pending = nil
}

// registerPending allocates a buffered future for an outbound upstream
// req keyed by msgID.  Returns nil when the session has already been
// torn down so the caller can short-circuit.  The buffer of 1 keeps the
// reader (deliverRsp) non-blocking even if the awaiter is slow.
func (s *connSession) registerPending(msgID string) chan *wsspb.ConnMsg {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.pending == nil {
		return nil
	}
	ch := make(chan *wsspb.ConnMsg, 1)
	s.pending[msgID] = ch
	return ch
}

// cancelPending removes a previously-registered future.  Idempotent —
// safe to call from a defer even after deliverRsp already consumed the
// entry.
func (s *connSession) cancelPending(msgID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pending == nil {
		return
	}
	delete(s.pending, msgID)
}

// isClosed is a quick lockless-ish check.  Reads s.closed under the
// mutex to avoid a race with markClosed; the call is rare enough that
// holding the lock briefly is fine.
func (s *connSession) isClosed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closed
}

// connectedFor returns how long the current session has lasted, or 0 when
// not connected.
func (s *WSSource) connectedFor() time.Duration {
	since := s.connectedSince.Load()
	if since == 0 {
		return 0
	}
	return time.Since(time.Unix(0, since))
}

// safeURL returns the URL with credentials redacted for logging.
func (s *WSSource) safeURL() string {
	u, err := url.Parse(s.URL)
	if err != nil {
		return s.URL
	}
	if u.User != nil {
		u.User = url.UserPassword(u.User.Username(), "***")
	}
	u.RawQuery = ""
	return u.String()
}

// loadToken returns the live access-token.  Falls back to the empty
// string when storeToken has never been called (which only happens if
// applyDefaults was bypassed — defensive only).
func (s *WSSource) loadToken() string {
	if p := s.tokenPtr.Load(); p != nil {
		return *p
	}
	return ""
}

// storeToken atomically replaces the live access-token.  The pointer
// indirection is required because atomic.Pointer needs a stable
// addressable value; we copy the string into a fresh local so the
// caller's variable can go out of scope without dangling.
func (s *WSSource) storeToken(t string) {
	s.tokenPtr.Store(&t)
}

// refreshAndStoreToken invokes TokenProvider (if set) and persists
// the result into tokenPtr so the next AuthBindReq picks it up.
// No-op when TokenProvider is nil (tests / MockSource path).  An empty
// string from the provider is rejected: the gateway would reject the
// AuthBind anyway, and surfacing the error here lets Run()'s backoff
// kick in instead of wasting a dial.
func (s *WSSource) refreshAndStoreToken(ctx context.Context) error {
	if s.TokenProvider == nil {
		return nil
	}
	newToken, err := s.TokenProvider(ctx)
	if err != nil {
		return fmt.Errorf("token provider: %w", err)
	}
	if newToken == "" {
		return errors.New("token provider returned empty token")
	}
	s.storeToken(newToken)
	return nil
}

// refreshTokenIfNeeded is the heartbeat-path companion to
// refreshAndStoreToken.  It detects an in-session token rotation and,
// when one happened, sends an AuthRefreshReq to re-bind the live WS
// session to the new token BEFORE the heartbeat ping goes out.
//
// Why the AuthRefresh frame matters: the gateway tracks the bound
// token per session.  If we silently swap tokenPtr but never tell the
// gateway, the moment the OLD token's TTL elapses the gateway will
// close the conn even though we already have a valid replacement on
// our side.  Sending AuthRefresh keeps the gateway's per-session
// binding in sync with our local view.
//
// Failure modes:
//
//   - TokenProvider error / empty token: bubble up; heartbeatLoop
//     closes the conn and Run() reconnects (where the dial-time
//     refresh tries again with the same provider).
//   - AuthRefresh write / ack timeout: same — bubble up, close conn,
//     reconnect.  Critically, we storeToken BEFORE writing the
//     AuthRefresh frame: that way even if the frame never makes it
//     out, the upcoming reconnect's doAuthBind picks up the new
//     token and the system self-heals on the next dial.
//
// Validation: we surface an error on either envelope-level rejection
// (Head.status != 0, via DecodeAuthRefreshRsp) or body-level failure
// (AuthRefreshRsp.ret_code != 0).  Both paths propagate up so the
// heartbeatLoop closes the conn and Run() reconnects, where the
// dial-time doAuthBind re-binds with the freshly stored token.
func (s *WSSource) refreshTokenIfNeeded(ctx context.Context, sess *connSession,
	hbStop <-chan struct{}) error {
	if s.TokenProvider == nil {
		return nil
	}
	newToken, err := s.TokenProvider(ctx)
	if err != nil {
		return fmt.Errorf("token provider: %w", err)
	}
	if newToken == "" {
		return errors.New("token provider returned empty token")
	}
	if newToken == s.loadToken() {
		// Token unchanged — the common case, since access-tokens live
		// far longer than the 25s heartbeat cadence.  Skip the
		// AuthRefresh frame entirely; no bytes on the wire.
		return nil
	}

	// Persist the new token FIRST.  If the AuthRefresh write or its
	// ack-wait below fails, the heartbeatLoop will close the conn and
	// Run() reconnects — the next runOnce's doAuthBind will read the
	// fresh value via loadToken() and re-authenticate cleanly, even
	// though we never successfully delivered the in-session refresh.
	s.storeToken(newToken)

	wire, msgID, err := wsspb.EncodeAuthRefreshReq(&s.seq, newToken, s.OpenID, s.CLIUniqID)
	if err != nil {
		return fmt.Errorf("encode auth refresh: %w", err)
	}

	ch := sess.registerPending(msgID)
	if ch == nil {
		return errors.New("auth refresh: session closed before register")
	}
	defer sess.cancelPending(msgID)

	s.writeMu.Lock()
	if sess.isClosed() {
		s.writeMu.Unlock()
		return errors.New("auth refresh: session closed mid-write")
	}
	_ = sess.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	werr := sess.conn.WriteMessage(websocket.BinaryMessage, wire)
	s.writeMu.Unlock()
	if werr != nil {
		return fmt.Errorf("write auth refresh: %w", werr)
	}

	timer := time.NewTimer(defaultRspTimeout)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-hbStop:
		return errors.New("auth refresh: stop requested")
	case <-timer.C:
		return errors.New("auth refresh: rsp timeout")
	case cm := <-ch:
		if cm == nil {
			return errors.New("auth refresh: session closed while waiting for rsp")
		}
		// Envelope-level Head.status is checked inside the decoder; a
		// non-zero status surfaces as an *wsspb.UpstreamRspStatusError
		// here.  We then layer the body-level ret_code on top because
		// the gateway populates AuthRefreshRsp.Msg with a human-readable
		// reason that aids diagnostics.
		rsp, derr := wsspb.DecodeAuthRefreshRsp(cm)
		if derr != nil {
			return fmt.Errorf("auth refresh: decode rsp: %w", derr)
		}
		if rsp.GetRetCode() != 0 {
			return fmt.Errorf("auth refresh: AuthRefreshRsp.ret_code=%d msg=%q",
				rsp.GetRetCode(), rsp.GetMsg())
		}
		return nil
	}
}

func (s *WSSource) applyDefaults() {
	if s.HandshakeTimeout <= 0 {
		s.HandshakeTimeout = defaultHandshakeTimeout
	}
	// Seed the live token from the construction-time Token field.
	// Done unconditionally so re-entry into applyDefaults (Run called
	// twice on the same instance — not a supported pattern but cheap
	// to be tolerant of) doesn't strand a stale tokenPtr.
	s.storeToken(s.Token)
	if s.ReadTimeout <= 0 {
		s.ReadTimeout = defaultReadTimeout
	}
	if s.PingPeriod <= 0 {
		s.PingPeriod = defaultHeartbeatInterval
	}
	if s.MinBackoff <= 0 {
		s.MinBackoff = defaultMinBackoff
	}
	if s.MaxBackoff <= 0 {
		s.MaxBackoff = defaultMaxBackoff
	}
	if s.ReadTimeout < 2*s.PingPeriod {
		s.ReadTimeout = 2 * s.PingPeriod
	}
}

// nextBackoff doubles cur, capped at max.
func nextBackoff(cur, max time.Duration) time.Duration {
	n := cur * 2
	if n > max {
		return max
	}
	return n
}

// sleepCtx sleeps for d or returns false if ctx cancels first.
func sleepCtx(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return ctx.Err() == nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return true
	case <-ctx.Done():
		return false
	}
}

// notifyMaybe is a nil-safe wrapper.  Sources should never assume a notifier
// is wired (a unit test driving Run directly may pass nil).
func notifyMaybe(n StatusNotifier, state, detail string) {
	if n != nil {
		n(state, detail)
	}
}

// notifyAndPersist routes a state change through both the StatusNotifier
// (in-memory, broadcast to subscribers via control+kind=source_status)
// and ws.state on disk (best-effort, picked up by `event status`).
func (s *WSSource) notifyAndPersist(n StatusNotifier, state, detail string) {
	notifyMaybe(n, state, detail)
	snap := eventruntime.WSState{
		State:          state,
		ReconnectCount: s.reconnects.Load(),
		Detail:         detail,
		LastChangeAt:   time.Now().UTC().Format(time.RFC3339),
	}
	if since := s.connectedSince.Load(); since > 0 {
		snap.ConnectedAt = time.Unix(0, since).UTC().Format(time.RFC3339)
	}
	_ = eventruntime.WriteWSState(snap)
}

// authError marks a fatal authentication failure.  Run() short-circuits
// on this rather than retrying.
type authError struct {
	code int
	err  error
}

func (e *authError) Error() string {
	return "auth failed: HTTP " + strconv.Itoa(e.code) + ": " + e.err.Error()
}

// Unwrap exposes the wrapped cause so errors.As / errors.Is can reach
// the underlying error — notably *wsspb.UpstreamRspStatusError when
// the gateway rejected AuthBind at the envelope layer.  Run() relies
// on this to scope the OnAuthFailed hook to the envelope path only
// (see WSSource.OnAuthFailed doc).  isAuthError still matches the
// outer *authError type directly, so its semantics are unchanged.
func (e *authError) Unwrap() error {
	return e.err
}

func isAuthError(err error) bool {
	var ae *authError
	return errors.As(err, &ae)
}

// serverClosedError marks a server-initiated WebSocket Close frame.
// Run() short-circuits on this rather than retrying — see the gateway
// contract note in readLoop for why "any close code" is fatal here.
type serverClosedError struct {
	code int
	text string
}

func (e *serverClosedError) Error() string {
	if e.text == "" {
		return "server closed: code=" + strconv.Itoa(e.code)
	}
	return "server closed: code=" + strconv.Itoa(e.code) + " text=" + strconv.Quote(e.text)
}

func isServerClosed(err error) bool {
	var se *serverClosedError
	return errors.As(err, &se)
}

// asServerClosed extracts a *serverClosedError from a raw read error
// when the underlying cause is a gorilla *websocket.CloseError.  Returns
// nil when err did not originate from a server-sent Close frame.
//
// Code 1006 (CloseAbnormalClosure) is explicitly excluded: gorilla
// synthesises it when the peer TCP-closes without sending a proper WS
// Close frame (e.g. process crash, network partition, or test helpers
// calling conn.Close()).  This is indistinguishable from a transient
// link failure and must trigger the normal reconnect path — not the
// "server told us to stop" short-circuit.
func asServerClosed(err error) *serverClosedError {
	var ce *websocket.CloseError
	if errors.As(err, &ce) {
		if ce.Code == websocket.CloseAbnormalClosure {
			return nil
		}
		return &serverClosedError{code: ce.Code, text: ce.Text}
	}
	return nil
}

// DecodeNDJSONFrame is a legacy FrameDecoder retained for tests / dev
// mocks.  It accepts text frames containing one JSON object of the
// RawEvent shape; binary / other frames are silently skipped.
//
// Production WSSource against Tencent Meeting uses the built-in pb path
// (Decoder left nil); this function exists for mock servers that cannot
// or do not want to speak protobuf.
func DecodeNDJSONFrame(messageType int, data []byte) (*eventruntime.RawEvent, error) {
	if messageType != websocket.TextMessage {
		return nil, nil
	}
	if len(data) == 0 {
		return nil, nil
	}
	var ev eventruntime.RawEvent
	if err := json.Unmarshal(data, &ev); err != nil {
		return nil, fmt.Errorf("ndjson decode: %w", err)
	}
	if ev.Event == "" {
		return nil, errors.New("ndjson decode: missing event field")
	}
	return &ev, nil
}
