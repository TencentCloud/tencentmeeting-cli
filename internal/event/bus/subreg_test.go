// subreg_test.go — unit tests for subRegistry.
//
// We pin five behaviours:
//
//  1. First Add reports the 0→1 transition; subsequent Adds do not.
//  2. Remove of the last subscriber reports the 1→0 transition;
//     intermediate Removes do not.
//  3. Q4 contract: even after 1→0 we keep the helper available, but the
//     bus is expected to ignore the return value (no UNSUBSCRIBE frame).
//     We assert Snapshot reflects the deletion immediately so Replay
//     post-1→0 doesn't include the gone key.
//  4. Empty eventKey is rejected defensively.
//  5. Concurrent Add/Remove are race-free under -race.

package bus

import (
	"sync"
	"sync/atomic"
	"testing"
)

func TestSubRegistry_AddTransitions(t *testing.T) {
	r := newSubRegistry()

	if !r.Add("k1") {
		t.Error("first Add(k1): want true, got false")
	}
	if r.Add("k1") {
		t.Error("second Add(k1): want false, got true")
	}
	if r.Count("k1") != 2 {
		t.Errorf("Count(k1) = %d, want 2", r.Count("k1"))
	}

	if !r.Add("k2") {
		t.Error("first Add(k2): want true (separate key)")
	}
}

func TestSubRegistry_RemoveTransitions(t *testing.T) {
	r := newSubRegistry()
	r.Add("k1")
	r.Add("k1")

	if r.Remove("k1") {
		t.Error("first Remove (count: 2->1): want false, got true")
	}
	if !r.Remove("k1") {
		t.Error("second Remove (count: 1->0): want true, got false")
	}
	if r.Count("k1") != 0 {
		t.Errorf("Count(k1) after full removal = %d, want 0", r.Count("k1"))
	}
}

func TestSubRegistry_RemoveUnknownIsNoop(t *testing.T) {
	r := newSubRegistry()
	if r.Remove("never-added") {
		t.Error("Remove of unknown key: want false")
	}
	r.Add("k")
	r.Remove("k")
	// Already at 0; further Remove must not panic / not report transition.
	if r.Remove("k") {
		t.Error("Remove of already-zero key: want false")
	}
}

func TestSubRegistry_SnapshotReflectsDeletions(t *testing.T) {
	// Q4 enforcement: after 1→0 the key MUST disappear from Snapshot so
	// Replay (called on every reconnect) doesn't keep re-subscribing
	// abandoned keys.
	r := newSubRegistry()
	r.Add("k1")
	r.Add("k2")
	r.Add("k2")

	snap := r.Snapshot()
	if len(snap) != 2 {
		t.Errorf("Snapshot len = %d, want 2: %v", len(snap), snap)
	}

	r.Remove("k1") // 1→0 for k1
	r.Remove("k2") // 2→1 for k2 (still tracked)

	snap = r.Snapshot()
	if len(snap) != 1 || snap[0] != "k2" {
		t.Errorf("Snapshot after partial removal = %v, want [k2]", snap)
	}

	r.Remove("k2") // 1→0 for k2
	if got := r.Snapshot(); len(got) != 0 {
		t.Errorf("Snapshot after all removed = %v, want []", got)
	}
}

func TestSubRegistry_EmptyKeyRejected(t *testing.T) {
	r := newSubRegistry()
	if r.Add("") {
		t.Error("Add(\"\"): want false (defensive reject)")
	}
	if r.Remove("") {
		t.Error("Remove(\"\"): want false")
	}
}

func TestSubRegistry_ConcurrentSafe(t *testing.T) {
	// -race validates the lock; logic check is "every transitionedToOne
	// is balanced by a transitionedToZero across N parallel
	// add-then-remove cycles".
	r := newSubRegistry()
	const workers = 8
	const cycles = 200

	var (
		toOne  atomic.Int64
		toZero atomic.Int64
	)

	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < cycles; i++ {
				if r.Add("shared") {
					toOne.Add(1)
				}
				if r.Remove("shared") {
					toZero.Add(1)
				}
			}
		}()
	}
	wg.Wait()

	// Final state: refcount must be 0 (every Add paired with Remove).
	if c := r.Count("shared"); c != 0 {
		t.Errorf("final Count = %d, want 0", c)
	}
	// Each 0→1 transition has a matching 1→0 transition (possibly
	// interleaved differently per goroutine but balanced overall).
	if toOne.Load() != toZero.Load() {
		t.Errorf("0→1 count (%d) != 1→0 count (%d)", toOne.Load(), toZero.Load())
	}
}
