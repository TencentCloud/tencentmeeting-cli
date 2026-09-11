// dedup.go — bounded trace_id deduplication.
//
// A WSS reconnect can cause the upstream service to replay events the
// bus has already received: same trace_id, same payload, but a fresh
// JSON frame.  Without dedup the hub would fan out the duplicate to
// every subscribed consumer and downstream consumers would see double
// counts in --max-events / corrupted Agent state machines.
//
// Implementation: a ring-buffer of N most-recently-seen trace_ids paired
// with a map for O(1) membership tests.  When the buffer is full, the
// oldest slot is overwritten and its trace_id evicted from the map.
//
// We pick N=512 by default — large enough to absorb a brief reconnect
// burst (typical WSS catch-up windows are tens of events) while keeping
// peak memory under 1 KiB worst case.  Adjust via NewDedup if a
// deployment has a noisier upstream.
//
// Thread-safety: protected by a single sync.Mutex.  The bus's emit
// callback is single-goroutine (one per Source — see bus.go:startSources)
// so contention is essentially nil; the lock is there to defend against
// future multi-source layouts and to keep the type usable from tests
// that fan in events from multiple goroutines.

package event

import "sync"

// DefaultDedupCapacity is the trace_id ring-buffer size used when callers
// pass 0 to NewDedup.  Tuned for "absorb a typical reconnect replay
// burst" — see file header for derivation.
const DefaultDedupCapacity = 512

// Dedup is a bounded trace_id seen-set.  Construct via NewDedup; the
// zero value is NOT usable (capacity defaults take effect only via the
// constructor).
//
// Empty trace_ids are never deduped — Seen("") always returns false and
// records nothing.  This matches the protocol invariant that real events
// always carry a non-empty trace_id; if we ever start synthesising
// events without one we want them all to flow through, not silently
// drop after the first.
type Dedup struct {
	mu      sync.Mutex
	cap     int
	ring    []string // ring[next] is the slot we'll overwrite next
	next    int      // index into ring; advances modulo cap
	present map[string]struct{}
}

// NewDedup returns a Dedup with the given capacity, or DefaultDedupCapacity
// when capacity <= 0.  Capacity is fixed for the lifetime of the
// instance — there's no Resize because shrinking would require deciding
// which existing entries to evict and growing offers no behavioural
// improvement until the new capacity is hit.
func NewDedup(capacity int) *Dedup {
	if capacity <= 0 {
		capacity = DefaultDedupCapacity
	}
	return &Dedup{
		cap:     capacity,
		ring:    make([]string, capacity),
		present: make(map[string]struct{}, capacity),
	}
}

// Seen reports whether traceID has already been recorded.  When it
// returns false the caller should process the event AND traceID is
// recorded for future Seen calls.  When it returns true the caller
// should drop the event — it's a duplicate from a WSS replay.
//
// Empty input always returns false and records nothing — see type doc.
func (d *Dedup) Seen(traceID string) bool {
	if traceID == "" {
		return false
	}
	d.mu.Lock()
	defer d.mu.Unlock()

	if _, ok := d.present[traceID]; ok {
		return true
	}

	// Evict the slot we're about to overwrite.  Empty string at startup
	// (the ring is zero-initialised) is fine — present has no "" key.
	if old := d.ring[d.next]; old != "" {
		delete(d.present, old)
	}
	d.ring[d.next] = traceID
	d.next = (d.next + 1) % d.cap
	d.present[traceID] = struct{}{}
	return false
}

// Len returns the current number of distinct trace_ids tracked.
// Test-only convenience — callers don't normally care.  Capped at
// capacity even after many evictions.
func (d *Dedup) Len() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.present)
}
