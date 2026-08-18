// mock.go — synthetic event source used until the real Tencent Meeting WSS
// integration lands (batch 4).
//
// MockSource fires one RawEvent per tick on a configurable interval, cycling
// through a small set of EventKeys so consumers can verify hub fan-out and
// dropped-frame behaviour without depending on the WSS protocol or live
// network.  It also drives the lifecycle state machine end-to-end so
// source_status control frames are exercised.
//
// MockSource is NOT registered automatically; cmd/event/bus.go installs it
// only when the bus is started without a real source compiled in.  This
// keeps unit-test behaviour deterministic and prevents accidental "synthetic
// events leaked into production" footguns.

package source

import (
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"time"

	eventruntime "tmeet/internal/event"
	"tmeet/internal/event/protocol"
)

// MockEventKeys is the rotation set used by MockSource.  These match the
// EventKeys registered in internal/event/schemas.go so consumers subscribing
// by name see real fan-out.
var mockEventKeys = []string{
	"meeting.started",
	"meeting.end",
}

// MockSource emits synthetic events at a fixed interval.
//
// The zero value uses a 5 s interval; pass a non-zero Interval to override.
// All fields are read-only after Run is called; concurrent Run on the same
// instance is unsupported.
type MockSource struct {
	// Interval between emitted events.  Zero means default5s.
	Interval time.Duration

	// counter ensures every emitted event has a unique trace_id even within
	// the same wall-clock second; atomic so callers can safely read it for
	// diagnostics.
	counter atomic.Uint64
}

// Name returns the canonical source identifier.
func (s *MockSource) Name() string { return "mock" }

// Run drives the lifecycle:
//
//	connecting → steady (immediately) → emit on each tick → disconnected
//	(when ctx cancels) → return.
//
// We don't simulate reconnect / auth_failed in batch 2.2 — those code paths
// are exercised by tests that drive notify() directly.
func (s *MockSource) Run(ctx context.Context, emit func(*eventruntime.RawEvent), notify StatusNotifier) error {
	if notify != nil {
		notify(protocol.SourceStateConnecting, "mock source starting")
	}

	interval := s.Interval
	if interval <= 0 {
		interval = 5 * time.Second
	}

	// Transition to steady immediately — a real WSS would wait for the
	// SUBSCRIBE ack frame, but the mock has nothing to wait on.
	if notify != nil {
		notify(protocol.SourceStateSteady, fmt.Sprintf("emitting every %s", interval))
	}

	tick := time.NewTicker(interval)
	defer tick.Stop()

	for {
		select {
		case <-ctx.Done():
			if notify != nil {
				notify(protocol.SourceStateDisconnected, "context cancelled")
			}
			// nil return is "graceful shutdown"; bus treats any Run return
			// as "shut down the daemon", so this exit code is fine.
			return nil

		case t := <-tick.C:
			ev := s.buildEvent(t)
			if ev != nil && emit != nil {
				emit(ev)
			}
		}
	}
}

// buildEvent crafts a synthetic RawEvent.  Trace IDs are unique per process
// (counter + nanos) so the bus's dedup filter does not collapse distinct
// mock events into one.
func (s *MockSource) buildEvent(t time.Time) *eventruntime.RawEvent {
	n := s.counter.Add(1)
	key := mockEventKeys[int(n-1)%len(mockEventKeys)]

	// Synthesise a payload shape matching schemas.go for the chosen key.
	// Tencent Meeting webhooks deliver `payload` as a length-1 array of
	// objects; we mirror that shape here so MatchPayload's array-index
	// PayloadPath ("0.meeting_info.meeting_id", etc.) lands on real
	// fields rather than missing.  Keep contents deliberately simple —
	// the goal is to validate plumbing, not to fake real data.
	inner := map[string]interface{}{
		"operate_time": t.UnixMilli(),
		"operator": map[string]interface{}{
			"userid":      fmt.Sprintf("mock-user-%d", n),
			"instance_id": "2",
		},
		"meeting_info": map[string]interface{}{
			"meeting_id":   fmt.Sprintf("mock-meeting-%d", n%10),
			"meeting_code": fmt.Sprintf("mock-code-%d", n%10),
			"subject":      "mock subject",
			"creator": map[string]interface{}{
				"userid":      fmt.Sprintf("mock-creator-%d", n%10),
				"instance_id": "2",
			},
			"meeting_type":        0,
			"start_time":          t.Unix(),
			"end_time":            t.Unix() + 3600,
			"meeting_create_mode": 1,
			"meeting_create_from": 1,
		},
	}
	if key == "meeting.end" {
		// Per https://cloud.tencent.com/document/product/1095/51619 the
		// end event carries an extra meeting_end_type at the array element
		// level (0=manual, 1=last-leave-after-deadline, etc.).
		inner["meeting_end_type"] = int(n) % 4
	}
	payloadArr := []interface{}{inner}

	raw, err := json.Marshal(payloadArr)
	if err != nil {
		return nil
	}
	return &eventruntime.RawEvent{
		Event:   key,
		TraceID: fmt.Sprintf("mock-%d-%d", t.UnixNano(), n),
		Payload: raw,
	}
}
