// wsstate.go — diagnostic snapshot of the WSS upstream connection.
//
// `event status` reports a {state, connected_at, reconnect_count} sub-
// object on the running bus entry.  Without an
// out-of-band place to read this from, status would have to query the
// bus over IPC for the source state, but the bus's IPC handler is
// itself the place we'd inject the read — circular and slow.
//
// Instead, the WSS source writes a small JSON file at every state
// transition.  status reads it; the file is best-effort (missing /
// stale / corrupt is treated as "unknown") so a half-written snapshot
// never blocks the status report.
//
// Atomicity: same tmp+rename pattern as bus.meta.  Reads tolerate
// transient ENOENT (the bus may not have written the first snapshot
// yet, e.g. between Listen() and the source's first state notify).

package event

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

// WSState is the on-disk shape of ws.state.
//
// State mirrors protocol.SourceState* string values; we keep it as a
// plain string here to avoid pulling protocol into this package (which
// is also imported by config-adjacent code paths).
//
// All fields are optional from the reader's perspective: a fresh bus
// that hasn't written its first state yet returns
// (WSState{}, false, nil).
type WSState struct {
	// State is one of the protocol.SourceState* values.
	State string `json:"state"`

	// ConnectedAt is the RFC3339 UTC timestamp of the most recent
	// successful WSS handshake.  Empty before the first connect.
	ConnectedAt string `json:"connected_at,omitempty"`

	// LastChangeAt is when the State field last transitioned.  Useful
	// for "how long has it been reconnecting?" diagnostics.
	LastChangeAt string `json:"last_change_at,omitempty"`

	// ReconnectCount is the cumulative count of reconnect attempts since
	// the bus started (NOT since the binary was installed).  Resets when
	// a fresh bus boots.
	ReconnectCount int64 `json:"reconnect_count"`

	// Detail carries the most recent notify() detail string (e.g.
	// "lost: read tcp ...", "dialing wss://...").  May be redacted /
	// truncated by future privacy passes — never trust this for parsing.
	Detail string `json:"detail,omitempty"`
}

// WriteWSState atomically replaces ws.state with the given snapshot.
//
// Same atomicity contract as WriteBusMeta: tmp file + rename, with the
// rename being atomic on POSIX/NTFS.  Errors propagate to the caller —
// the WSS source treats a write failure as a soft warning (logs but
// keeps running) since stale ws.state never gates correctness.
func WriteWSState(state WSState) error {
	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("ws state: marshal: %w", err)
	}
	// trailing newline so cat-ing the file looks tidy.
	data = append(data, '\n')

	return atomicWriteFile(WSStateFile(), data, "ws state")
}

// ReadWSState reads ws.state.
//
// Returns:
//   - state, true,  nil   — file present and parses cleanly
//   - {},   false, nil    — file does not exist (no bus has written yet)
//   - {},   false, err    — file present but malformed; caller decides
//     whether to surface or downgrade.  status downgrades to "unknown"
//     so a half-written snapshot doesn't break the report.
func ReadWSState() (WSState, bool, error) {
	path := WSStateFile()
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return WSState{}, false, nil
		}
		return WSState{}, false, fmt.Errorf("ws state: read %s: %w", path, err)
	}
	var st WSState
	if err = json.Unmarshal(data, &st); err != nil {
		return WSState{}, false, fmt.Errorf("ws state: parse %s: %w", path, err)
	}
	return st, true, nil
}

// RemoveWSState deletes ws.state if present.  Idempotent.  Used by the
// bus daemon's deferred shutdown to scrub the snapshot, since a stale
// state file outliving the bus would mislead the next status call.
func RemoveWSState() error {
	err := os.Remove(WSStateFile())
	if err == nil || os.IsNotExist(err) {
		return nil
	}
	return fmt.Errorf("ws state: remove: %w", err)
}
