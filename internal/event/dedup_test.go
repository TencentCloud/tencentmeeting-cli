// dedup_test.go — unit tests for Dedup.
//
// We pin five behaviours:
//
//  1. Empty trace_id is NEVER deduped (protocol invariant).
//  2. First sighting of a trace_id returns false; second returns true.
//  3. Eviction kicks in once capacity is exceeded; the oldest entry
//     becomes "unknown" again and re-records on next Seen.
//  4. Distinct trace_ids never collide regardless of insertion order.
//  5. Concurrent Seen calls are race-free under -race.

package event

import (
	"fmt"
	"sync"
	"testing"
)

func TestDedup_EmptyTraceIDNeverDeduped(t *testing.T) {
	d := NewDedup(0) // exercise default capacity path
	for i := 0; i < 5; i++ {
		if d.Seen("") {
			t.Fatalf("empty trace_id reported as seen on attempt %d", i)
		}
	}
	if got := d.Len(); got != 0 {
		t.Errorf("Len = %d after only empty inserts, want 0", got)
	}
}

func TestDedup_FirstSightingFalseSecondTrue(t *testing.T) {
	d := NewDedup(8)

	if d.Seen("trc-1") {
		t.Fatal("first sighting reported as seen")
	}
	if !d.Seen("trc-1") {
		t.Fatal("second sighting NOT reported as seen")
	}
	// Adding a second distinct id must not affect the first.
	if d.Seen("trc-2") {
		t.Fatal("trc-2 first sighting reported as seen")
	}
	if !d.Seen("trc-1") {
		t.Fatal("trc-1 lost after trc-2 added")
	}
}

func TestDedup_EvictionByCapacity(t *testing.T) {
	d := NewDedup(3)

	// Insert 4 distinct ids into a capacity-3 ring; the first should
	// evict.
	for i := 0; i < 4; i++ {
		if d.Seen(fmt.Sprintf("t%d", i)) {
			t.Fatalf("t%d reported as seen on first sighting", i)
		}
	}

	// Re-Seening the oldest entry (t0) should now look fresh.
	if d.Seen("t0") {
		t.Errorf("t0 should have been evicted but is still tracked")
	}
	// The newer entries must still be tracked.
	for _, id := range []string{"t2", "t3"} {
		if !d.Seen(id) {
			t.Errorf("%s should still be tracked", id)
		}
	}
	// After re-seening t0 we have {t0,t2,t3} (capacity 3) — t1 should
	// have been evicted by the most-recent t0 insert above.
	if d.Seen("t1") {
		t.Errorf("t1 should have been evicted; got reported as seen")
	}
}

func TestDedup_LenCapsAtCapacity(t *testing.T) {
	d := NewDedup(4)
	for i := 0; i < 100; i++ {
		d.Seen(fmt.Sprintf("t%d", i))
	}
	if got := d.Len(); got > 4 {
		t.Errorf("Len = %d, want <= cap=4", got)
	}
}

func TestDedup_ConcurrentSafe(t *testing.T) {
	// Race detector validates this; logic check is a smoke test.
	d := NewDedup(64)
	var wg sync.WaitGroup
	for w := 0; w < 8; w++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				_ = d.Seen(fmt.Sprintf("w%d-%d", worker, i))
			}
		}(w)
	}
	wg.Wait()
	if d.Len() == 0 {
		t.Error("Len should be > 0 after concurrent inserts")
	}
}

func TestDedup_DistinctIdsDoNotCollide(t *testing.T) {
	d := NewDedup(1024)
	for i := 0; i < 500; i++ {
		id := fmt.Sprintf("trace-%d", i)
		if d.Seen(id) {
			t.Fatalf("%s collided with a prior entry", id)
		}
	}
	if got := d.Len(); got != 500 {
		t.Errorf("Len = %d, want 500", got)
	}
}
