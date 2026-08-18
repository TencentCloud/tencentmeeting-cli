// Package source is a pluggable event-source abstraction.
//
// A Source produces RawEvents and pushes them through the bus's emit callback.
// Sources own their own connection lifecycle (connect / heartbeat / reconnect)
// and report state changes through StatusNotifier so the bus can broadcast
// source_status control frames to all subscribers.
//
// Wemeet has at most one active Source at any given time (the mock source in
// batch 2.2; a real WSS source in batch 4).  The package keeps a process-wide
// registry mostly for symmetry with the lark-cli design and for tests that
// want to swap in a fake.
package source

import (
	"context"
	"sync"

	eventruntime "tmeet/internal/event"
)

// StatusNotifier surfaces lifecycle state changes to the bus.
//
// state values come from protocol.SourceState* constants; detail is a
// free-form, non-sensitive string (e.g. "auth refresh ok", "code=401").
// detail MUST NOT contain access tokens or other secrets — it ends up in
// stderr of consume processes when the user hasn't passed --quiet.
type StatusNotifier func(state, detail string)

// Source produces events.
//
// Run blocks until ctx.Done() (graceful shutdown) or an unrecoverable error.
// emit MUST be called from a single goroutine and MUST return quickly: it
// hands the event to the hub which fan-outs to subscribers without blocking.
// The bus treats Run returning at all (with or without error) as a signal to
// shut down the daemon — there is no automatic restart at this layer.
type Source interface {
	// Name returns a stable identifier (used in source_status frames and logs).
	Name() string

	// Run drives the source until ctx cancellation or fatal error.
	Run(ctx context.Context, emit func(*eventruntime.RawEvent), notify StatusNotifier) error
}

// Subscribable is an optional interface a Source MAY implement to receive
// per-EventKey subscription notifications from the hub.
//
// Why optional?  Sources that synthesise their own event stream (the
// development MockSource is the prime example) never need an upstream
// subscribe call; forcing them to implement an empty Subscribe stub would
// add boilerplate without value.  The hub therefore type-asserts:
//
//	if sb, ok := src.(Subscribable); ok { _ = sb.Subscribe(ctx, key, agentOpenID) }
//
// Semantics (matches the consume contract + WSSource's wire spec):
//
//   - Subscribe is invoked by the bus when the *first* consumer of a given
//     EventKey appears (refcount 0→1 transition).  Subsequent consumers of
//     the same key DO NOT re-trigger Subscribe.
//   - Subscribe is also invoked at *each successful (re-)connect* with the
//     full snapshot of currently-subscribed keys (Replay).  The contract
//     here is "fire-and-forget per key": the implementation may choose to
//     batch internally (WSSource's SubscribeReq.event_list takes a list)
//     but the hub-facing API is per-key.
//   - agentOpenID carries the agent (子账号) open_id for events whose
//     SubscribeRole == agent; it is forwarded into the upstream
//     SubscribeReq so the master connection subscribes on behalf of the
//     agent.  Empty for master/none events.
//   - There is intentionally NO Unsubscribe: per the Tencent Meeting WSS
//     contract the gateway garbage-collects subscriptions server-side when
//     a consumer disappears (no SUBSCRIBE refresh from the connection
//     within the TTL).  Adding an UNSUBSCRIBE here would be premature.
//
// Errors returned from Subscribe are logged at WARN level by the bus and
// otherwise ignored: subscription failures are best-effort, retries
// happen implicitly on the next reconnect cycle.
type Subscribable interface {
	Subscribe(ctx context.Context, eventKey, agentOpenID string) error
}

// ReconnectNotifiable is an optional interface a Source MAY implement to
// receive a callback after every successful (re-)connect.  The bus
// installs the callback once at startup; it is then the source's
// responsibility to invoke fn synchronously after auth completes.
//
// The bus uses this hook to drive Replay: on every reconnect it iterates
// the current hub.SubscribedKeys() and re-invokes Subscribable.Subscribe
// for each.  Without this hook the source would have no way to know when
// to refresh server-side state after a network blip.
//
// Sources that don't multiplex (MockSource) leave this unimplemented; the
// bus skips the registration via type-assert.
type ReconnectNotifiable interface {
	SetOnReconnected(fn func())
}

// SubscribeResultNotifiable is an optional interface a Source MAY
// implement to surface the gateway's verdict on an upstream
// Subscribable.Subscribe / SubscribeBatch call.
//
// fn is invoked once per upstream subscribe (regardless of how many keys
// were batched) when:
//
//   - The matching SubscribeRsp arrived (code != 0 \u2192 gateway rejected
//     the subscribe; non-empty msg carries the gateway's diagnostic).
//   - The wait for SubscribeRsp timed out (code == 0, msg synthesised
//     by the source \u2014 treated by the bus the same as a real rejection
//     because the gateway is unresponsive).
//
// fn is NOT invoked when Subscribe / SubscribeBatch returned a write
// error: the upstream call never made it onto the wire so there is
// nothing to notify about.  Implementations should be quick and
// goroutine-safe \u2014 the source may invoke fn from a background watcher
// goroutine without serialising calls.
//
// The bus uses this to fan out a subscribe_error control frame to the
// affected consumers (per the consume contract: subscribe failure \u2192
// consumer exits 1 with reason=subscribe_failed).  Sources that don't
// have an upstream-rsp concept (MockSource) leave this unimplemented.
type SubscribeResultNotifiable interface {
	SetOnSubscribeResult(fn func(eventKeys []string, code uint32, msg string))
}

var (
	registry   []Source
	registryMu sync.Mutex
)

// All returns a snapshot of registered sources.  The bus iterates this once
// at startup and starts each in its own goroutine.
func All() []Source {
	registryMu.Lock()
	defer registryMu.Unlock()
	out := make([]Source, len(registry))
	copy(out, registry)
	return out
}
