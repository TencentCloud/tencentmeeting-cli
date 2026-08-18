// subreg.go — per-EventKey subscriber refcount used to drive upstream
// SUBSCRIBE frames.
//
// The hub already tracks Subscribers (one per consumer connection); each
// Subscriber is pinned to exactly one EventKey for its lifetime.  What
// the hub does NOT natively track is the count of subscribers per key,
// which is what a Subscribable Source needs to know when to fire
// upstream SUBSCRIBE: once on the 0→1 transition, never again until the
// next reconnect's Replay.
//
// We keep this in a small dedicated type so the bookkeeping is testable
// in isolation and the hub stays focused on fan-out logic.  The contract
// chosen for Q4 ("Tencent Meeting auto-cancels subscriptions server-side
// when consumers disappear") means we deliberately do NOT report 1→0
// transitions — the upstream never receives an UNSUBSCRIBE frame.

package bus

import "sync"

// subRegistry is a goroutine-safe refcount keyed by EventKey.
//
// Empty value is usable; Add/Remove/Snapshot acquire the mutex
// internally so no external synchronisation is required.  All public
// methods complete in O(1) amortised; Snapshot is O(N) in the number of
// distinct keys.
type subRegistry struct {
	mu    sync.Mutex
	count map[string]int
}

// newSubRegistry returns an empty registry.  Wraps make() so the rest of
// the bus code never touches the internal map type directly.
func newSubRegistry() *subRegistry {
	return &subRegistry{count: make(map[string]int)}
}

// Add increments the refcount for eventKey.  Returns true iff this was a
// 0→1 transition — i.e., the caller should fire the upstream SUBSCRIBE
// for this key.  Subsequent Add calls for the same key return false.
//
// Empty eventKey is treated as a programming error (Subscribers always
// have a non-empty EventKey, validated at Hello time) but handled
// defensively: refcount stays untouched and false is returned so a
// caller can't fire SUBSCRIBE for an empty string.
func (r *subRegistry) Add(eventKey string) (transitionedToOne bool) {
	if eventKey == "" {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	prev := r.count[eventKey]
	r.count[eventKey] = prev + 1
	return prev == 0
}

// Remove decrements the refcount for eventKey.  Returns true iff this
// was the 1→0 transition.  Per Q4 the bus does NOT act on the return
// value (no UNSUBSCRIBE frame is ever sent), but we expose it so
// alternative gateways with explicit unsubscribe semantics could plug
// in without changing the registry API.
//
// Remove of an unknown / already-zero key is a no-op (returns false).
// This is forgiving by design: a consumer disconnect that happens
// while the hub is mid-Register / mid-Unregister could otherwise race.
func (r *subRegistry) Remove(eventKey string) (transitionedToZero bool) {
	if eventKey == "" {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	prev, ok := r.count[eventKey]
	if !ok || prev <= 0 {
		return false
	}
	if prev == 1 {
		delete(r.count, eventKey)
		return true
	}
	r.count[eventKey] = prev - 1
	return false
}

// Snapshot returns the set of currently-subscribed event keys.  Used by
// the bus's Replay path on every reconnect: the source needs the full
// current set so the gateway resumes pushing the right events.
//
// The returned slice is a fresh allocation so callers may freely
// retain / mutate it without locking.
func (r *subRegistry) Snapshot() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, 0, len(r.count))
	for k := range r.count {
		out = append(out, k)
	}
	return out
}

// Count returns the current refcount for eventKey (0 if absent).  Test-
// only convenience kept here so unit tests don't need to fish around in
// the internal map.
func (r *subRegistry) Count(eventKey string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.count[eventKey]
}
