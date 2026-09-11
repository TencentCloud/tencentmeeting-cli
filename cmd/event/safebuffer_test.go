// safebuffer_test.go — thread-safe stand-in for *bytes.Buffer used by
// the cmd-layer end-to-end tests.
//
// Why this exists: runConsumeLoop pumps stderr lines from a goroutine
// (the consume reader loop) while the main test goroutine calls
// stderr.String() in waitForReadyMarker / inline assertions.
// *bytes.Buffer is documented as not safe for concurrent use, so under
// `go test -race` every cmd/event consume test trips DATA RACE warnings
// on bytes.Buffer.grow/Buffer.String.
//
// safeBuffer wraps bytes.Buffer with a single mutex, exposing the same
// API surface that callers use today:
//
//	Write(p []byte) (int, error)   -- io.Writer for cobra.SetOut/SetErr
//	String() string                -- snapshot read, race-free
//	Bytes() []byte                 -- snapshot read, returns a copy
//
// Existing call sites only need a literal swap: `&bytes.Buffer{}`
// becomes `newSafeBuffer()` and any `*bytes.Buffer` typed parameter
// becomes `*safeBuffer`.  The .Bytes() result is a fresh slice (unlike
// bytes.Buffer.Bytes which aliases the internal storage) so downstream
// readers never observe a write happening underneath them.
//
// Test-only by design — file name carries the `_test.go` suffix so the
// type does not bleed into production builds.

package event

import (
	"bytes"
	"sync"
)

// safeBuffer is a goroutine-safe wrapper around bytes.Buffer.
//
// All methods take the same mutex; readers therefore see a consistent
// snapshot and writers cannot interleave with a snapshot in progress.
// The lock is held only for the duration of the buffer call, so a
// blocked Write cannot starve a String() reader (and vice versa).
type safeBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

// newSafeBuffer returns an empty *safeBuffer ready to be passed to
// cobra.Command.SetOut / SetErr.  Tiny convenience over `&safeBuffer{}`
// to keep the call-site visual diff against the prior `&bytes.Buffer{}`
// idiom minimal.
func newSafeBuffer() *safeBuffer {
	return &safeBuffer{}
}

// Write satisfies io.Writer.  The cobra command writes through this
// without knowing it is locked — that's the whole point.
func (b *safeBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

// String returns the contents accumulated so far.  Race-free: held
// under the same lock as Write, so the returned string reflects a
// well-defined point-in-time snapshot.
func (b *safeBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// Bytes returns a *copy* of the contents accumulated so far.
//
// bytes.Buffer.Bytes() aliases internal storage which would race the
// moment a concurrent Write triggered a grow().  Copying once under
// the lock gives the caller a stable slice that survives subsequent
// writes without further synchronisation, at the cost of one
// allocation per call — fine for a test helper.
func (b *safeBuffer) Bytes() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	src := b.buf.Bytes()
	out := make([]byte, len(src))
	copy(out, src)
	return out
}

// Len returns the current byte count.  Provided for parity with
// bytes.Buffer; not currently used by tests but cheap to expose and
// makes the wrapper a drop-in replacement for any future call.
func (b *safeBuffer) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Len()
}

// Reset clears the buffer.  Same parity reasoning as Len.
func (b *safeBuffer) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buf.Reset()
}
