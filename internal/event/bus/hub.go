// hub.go — subscriber registry + event fan-out.
//
// Compared to lark-cli, the wemeet hub is dramatically simpler:
//
//   - No subscriptionID / cleanupInProgress dance: there's no
//     PreShutdownCheck/Ack in our protocol, so the cleanup-lock TOCTOU race the
//     lark hub prevents simply doesn't apply here.
//   - No per-subscriber Seq: the protocol omits the seq field; gap
//     detection via control+kind=dropped is the sole mechanism.
//   - Single global owner check happens in conn.go's Hello path; the hub
//     itself doesn't know about owners.
//
// The hub is concerned with three things:
//
//	1. Who is subscribed to which EventKey.
//	2. When a RawEvent arrives, push it to every matching subscriber with
//	   drop-oldest back-pressure.
//	3. Aggregate "I dropped N for you between t0 and now" notifications and
//	   broadcast source_status frames best-effort.

package bus

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	eventruntime "tmeet/internal/event"
	"tmeet/internal/event/protocol"
	tlog "tmeet/internal/log"
)

// dropAggregateInterval — minimum gap between two dropped-frame notifications
// to the same conn for the same EventKey.  Without it a slow consumer would
// be hammered with one notification per dropped event, which both spams the
// stderr log and competes with real events for the send chan slot.
//
// 1 s matches the documented "aggregate dropped events within 1 s" policy.
const dropAggregateInterval = time.Second

// Subscriber is the contract a connection must satisfy for the hub to fan out
// to it.  Defined as an interface so hub_test can substitute a fake Conn.
type Subscriber interface {
	EventKey() string
	Params() map[string]string
	// AgentOpenID returns the agent (子账号) open_id this subscriber
	// carried in Hello, or "" for master/none events.  Forwarded to the
	// upstream SubscribeReq so the master connection can subscribe on
	// behalf of the agent.
	AgentOpenID() string
	PID() int
	Received() int64
	IncrementReceived()
	DroppedCount() int64
	IncrementDropped()

	// PushDropOldest enqueues msg with drop-oldest back-pressure.
	// Returns (enqueued, dropped) — see Conn.PushDropOldest for semantics.
	PushDropOldest(msg interface{}) (enqueued, dropped bool)

	// TrySend enqueues without back-pressure (best-effort).  Used for
	// control frames (source_status, dropped notifications) that must
	// share the sender goroutine's serialisation guarantees but are not
	// worth applying back-pressure for.
	TrySend(msg interface{}) bool
}

// Hub tracks subscribers and routes RawEvents to them.  All public methods
// are goroutine-safe.
type Hub struct {
	mu          sync.RWMutex
	subscribers map[Subscriber]*subState

	// subreg tracks per-EventKey refcount so the hub can fire upstream
	// SUBSCRIBE on the 0→1 transition.  See subreg.go for the rationale
	// behind keeping it separate from the subscribers map.
	subreg *subRegistry

	// onFirstSubscribe is invoked (under no lock) when subreg.Add reports
	// the 0→1 transition for an EventKey.  The bus injects the closure
	// that calls Subscribable.Subscribe on the active source; nil means no
	// upstream is interested (e.g. MockSource path) and the hub silently
	// skips the call.  agentOpenID is the subscriber's agent (子账号)
	// open_id (empty for master/none events).
	onFirstSubscribe func(eventKey, agentOpenID string)

	logger atomic.Pointer[tlog.Logger]
}

// subState tracks per-subscriber drop-aggregation timing.  Kept inside the
// hub (rather than on Conn) so reading/writing aggregation state is gated by
// h.mu without leaking into Conn's writeMu.
type subState struct {
	muDrop          sync.Mutex // protects the aggregation fields
	pendingDrops    int64      // events dropped since last broadcast
	firstDropUnix   int64      // earliest drop in the pending window
	lastBroadcastAt time.Time
}

// NewHub constructs an empty hub.  Logger is attached lazily via SetLogger.
func NewHub() *Hub {
	return &Hub{
		subscribers: make(map[Subscriber]*subState),
		subreg:      newSubRegistry(),
	}
}

// SetOnFirstSubscribe installs the callback fired on every 0→1 EventKey
// transition.  Pass nil to disable (the default).  Safe to call exactly
// once at bus startup; calling it after Subscribers have already
// registered is supported but the existing keys' 0→1 transitions have
// already passed — if the caller wants to seed an upstream after that,
// it should iterate SubscribedKeys() itself and call Subscribe for each.
func (h *Hub) SetOnFirstSubscribe(fn func(eventKey, agentOpenID string)) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.onFirstSubscribe = fn
}

// SetLogger attaches a logger; nil-tolerated.  Stored atomically so concurrent
// Publish goroutines see updates without locking h.mu.
func (h *Hub) SetLogger(l *tlog.Logger) { h.logger.Store(l) }

// Register adds s to the subscriber set.  Idempotent: re-registering the
// same Subscriber is a no-op (preserves existing subState).
//
// As a side effect, when this is the first subscriber for s.EventKey()
// (refcount 0→1), the configured onFirstSubscribe callback is invoked
// AFTER the lock is released so the upstream SUBSCRIBE call can take
// any time it needs without blocking the hub.
func (h *Hub) Register(s Subscriber) {
	h.mu.Lock()
	if _, exists := h.subscribers[s]; exists {
		h.mu.Unlock()
		return
	}
	h.subscribers[s] = &subState{}
	transitioned := h.subreg.Add(s.EventKey())
	cb := h.onFirstSubscribe
	h.mu.Unlock()

	if transitioned && cb != nil {
		cb(s.EventKey(), s.AgentOpenID())
	}
}

// Unregister removes s.  Returns true iff s was registered.
//
// Per Q4 ("the gateway garbage-collects subscriptions server-side when
// consumers disappear") we deliberately do NOT fire any callback on
// the 1→0 transition: the upstream is informed implicitly via the
// natural absence of refresh frames.  We DO update the registry so
// SubscribedKeys() / Replay see the post-removal state immediately.
func (h *Hub) Unregister(s Subscriber) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, exists := h.subscribers[s]; !exists {
		return false
	}
	delete(h.subscribers, s)
	_ = h.subreg.Remove(s.EventKey())
	return true
}

// ConnCount returns the current number of subscribers.
func (h *Hub) ConnCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.subscribers)
}

// SubscribedKeys returns the deduplicated EventKey set across all subscribers.
// Used by `event status` to populate StatusResponse.SubscribedKeys.
func (h *Hub) SubscribedKeys() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	seen := make(map[string]struct{}, len(h.subscribers))
	out := make([]string, 0, len(h.subscribers))
	for s := range h.subscribers {
		k := s.EventKey()
		if _, dup := seen[k]; dup {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, k)
	}
	return out
}

// SubscribedKeysByAgent groups the currently-subscribed EventKeys by the
// agent (子账号) open_id their subscribers carried in Hello.  Used by the
// reconnect Replay path: a single SubscribeReq carries one event_list AND
// one agent_open_id, so keys belonging to different agents (or to the
// master, keyed by "") must be re-subscribed in separate upstream calls.
//
// Keys are deduplicated within each agent group.  In practice there is at
// most one agent per master, so the returned map has 1–2 entries ("" for
// master/none events plus the single agent), but grouping keeps the
// contract correct if that ever changes.
func (h *Hub) SubscribedKeysByAgent() map[string][]string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make(map[string][]string)
	seen := make(map[string]map[string]struct{})
	for s := range h.subscribers {
		agent := s.AgentOpenID()
		key := s.EventKey()
		if seen[agent] == nil {
			seen[agent] = make(map[string]struct{})
		}
		if _, dup := seen[agent][key]; dup {
			continue
		}
		seen[agent][key] = struct{}{}
		out[agent] = append(out[agent], key)
	}
	return out
}

// Consumers returns a snapshot of registered consumers for status responses.
func (h *Hub) Consumers() []protocol.ConsumerInfo {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make([]protocol.ConsumerInfo, 0, len(h.subscribers))
	for s := range h.subscribers {
		out = append(out, protocol.ConsumerInfo{
			PID:      s.PID(),
			EventKey: s.EventKey(),
			Received: s.Received(),
			Dropped:  s.DroppedCount(),
		})
	}
	return out
}

// Publish fans out raw to every matching subscriber.  Matching is two-step:
//
//  1. EventKey equality (every subscriber pinned to one key at Hello time).
//  2. Per-subscriber --param filter via eventruntime.MatchPayload.
//
// The §2.3 contract says params-mismatched events are silently dropped: not
// delivered, NOT counted as a drop (it isn't back-pressure, it's a filter).
// recordDrop is therefore only invoked when PushDropOldest evicts something
// from a full chan, never from a params miss.
//
// Non-blocking: each subscriber gets its own PushDropOldest, which evicts
// one queued event on overflow rather than stalling the source.
func (h *Hub) Publish(raw *eventruntime.RawEvent) {
	if raw == nil || raw.Event == "" {
		return
	}

	// Look up the schema once per Publish — cheap (map lookup) and lets us
	// pass it into MatchPayload without each Subscriber re-resolving.
	def, _ := eventruntime.Lookup(raw.Event)
	var schema map[string]eventruntime.ParamDef
	if def != nil {
		schema = def.ParamsSchema
	}

	// Snapshot under RLock — the actual send happens outside the lock so a
	// slow consumer can't stall fan-out to others.
	h.mu.RLock()
	matches := make([]Subscriber, 0, len(h.subscribers))
	states := make([]*subState, 0, len(h.subscribers))
	for s, st := range h.subscribers {
		if s.EventKey() != raw.Event {
			continue
		}
		if !eventruntime.MatchPayload(schema, s.Params(), raw.Payload) {
			continue
		}
		matches = append(matches, s)
		states = append(states, st)
	}
	h.mu.RUnlock()

	if len(matches) == 0 {
		return
	}

	// Build the wire-level Event once; PushDropOldest doesn't mutate it so
	// sharing across subscribers is safe.  (lark-cli has to build per-sub
	// because of the seq field; we don't have one.)
	msg := protocol.NewEvent(raw.Event, raw.TraceID, raw.Payload)

	for i, s := range matches {
		enqueued, dropped := s.PushDropOldest(msg)
		if dropped {
			s.IncrementDropped()
			h.recordDrop(s, states[i], raw.Event)
		}
		if enqueued {
			s.IncrementReceived()
		}
	}
}

// recordDrop accumulates dropped counts and emits at most one aggregated
// control+kind=dropped per dropAggregateInterval per subscriber.
//
// We deliberately do NOT track per-EventKey drop windows: a subscriber is
// pinned to one EventKey at Hello time, so "drops since X" always describe
// the same key.  If we ever support multi-key subscriptions the eventKey
// arg lets us extend without an API churn.
func (h *Hub) recordDrop(s Subscriber, st *subState, eventKey string) {
	if st == nil {
		return
	}
	st.muDrop.Lock()
	st.pendingDrops++
	if st.firstDropUnix == 0 {
		st.firstDropUnix = time.Now().Unix()
	}
	now := time.Now()
	if now.Sub(st.lastBroadcastAt) < dropAggregateInterval {
		st.muDrop.Unlock()
		return
	}
	count := st.pendingDrops
	since := st.firstDropUnix
	st.pendingDrops = 0
	st.firstDropUnix = 0
	st.lastBroadcastAt = now
	st.muDrop.Unlock()

	notif := protocol.NewControlDropped(eventKey, count, since)
	if !s.TrySend(notif) {
		// Send chan was full — the consumer is *really* slow.  We log but
		// don't retry: the next dropped event will retry the aggregation.
		// h.logger.Load() returning nil is fine: *tlog.Logger.Warnf is
		// nil-receiver safe (see internal/log/logging.go:send).
		h.logger.Load().Warnf(context.Background(),
			"hub: dropped-notification fell off the wire for pid=%d key=%s count=%d",
			s.PID(), eventKey, count)
	}
}

// BroadcastSourceStatus best-effort sends a source_status control frame to
// every subscriber.  TrySend so a slow consumer doesn't stall the source.
func (h *Hub) BroadcastSourceStatus(srcName, state, detail string) {
	msg := protocol.NewControlSourceStatus(srcName, state, detail)
	h.mu.RLock()
	defer h.mu.RUnlock()
	for s := range h.subscribers {
		_ = s.TrySend(msg)
	}
}

// BroadcastSubscribeError routes a subscribe_error control frame ONLY to
// subscribers of eventKey.  Used when the upstream WsCLISubscribeEvent
// returned a non-zero code: the gateway will never push this key, so the
// affected consumers must be told (and per the consume contract they exit
// 1 with reason=subscribe_failed).
//
// Why per-EventKey instead of broadcast: a consumer subscribed to key A
// shouldn't be torn down because a different consumer's key B failed to
// subscribe upstream.  Routing by EventKey keeps the blast radius tight.
//
// TrySend best-effort \u2014 same rationale as BroadcastSourceStatus.  A
// consumer whose send chan is full will miss this notification, but the
// next reconnect's Replay will retry the upstream subscribe and produce
// a fresh subscribe_error if the failure is sticky.
func (h *Hub) BroadcastSubscribeError(eventKey string, code uint32, msg string) {
	if eventKey == "" {
		return
	}
	frame := protocol.NewControlSubscribeError(eventKey, code, msg)
	h.mu.RLock()
	defer h.mu.RUnlock()
	for s := range h.subscribers {
		if s.EventKey() != eventKey {
			continue
		}
		_ = s.TrySend(frame)
	}
}
