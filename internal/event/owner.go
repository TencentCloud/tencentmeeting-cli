// owner.go — bus owner identity & metadata file.
//
// The bus runs as a long-lived daemon and binds at startup to the OpenId of
// the user who forked it.  Subsequent consumers (and `event status`) need a
// way to verify that the bus they connected to belongs to the same logged-in
// user, otherwise stale-owner scenarios (logout-while-bus-running) would
// silently leak events to the wrong account.
//
// We embed a short hash of the OpenId (not the OpenId itself) into Hello
// frames and into bus.meta:
//
//   - sha256(openId) truncated to 12 bytes hex (24 chars) — collision risk is
//     negligible at single-machine scale and the value fits comfortably in
//     log lines / status output.
//   - storing only the hash means crash-dumps of bus.meta or stderr logs
//     never leak the user's actual OpenId.
//
// On bus startup the daemon writes bus.meta atomically (tmp + rename); on
// every Hello frame the bus compares Hello.OpenIDHash against meta.OpenIDHash
// and returns HelloAck{Error: WrongOwner} on mismatch.

package event

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"time"
	"tmeet/internal/exception"
)

// OpenIDHash returns the canonical short fingerprint used everywhere the bus
// checks owner identity.  Empty input returns the empty string so callers
// can distinguish "no user" from a legitimate zero-fingerprint user.
func OpenIDHash(openID string) string {
	if openID == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(openID))
	return hex.EncodeToString(sum[:12])
}

// BusMeta is the on-disk shape of bus.meta.  All fields are mandatory; an
// empty BusMeta on read is treated as a corruption signal by callers.
//
// Versioned with `meta_version` so a future schema bump can be detected
// without parsing pitfalls.  If we ever need to add fields, bump the version
// and have ReadBusMeta return a typed error on unknown values.
type BusMeta struct {
	MetaVersion int    `json:"meta_version"`
	OpenIDHash  string `json:"openid_hash"`
	StartedAt   string `json:"started_at"` // RFC3339, UTC
	BusVersion  string `json:"bus_version"`
	PID         int    `json:"pid"`
}

// busMetaCurrentVersion is the on-disk schema version this binary writes.
// Bump iff a backward-incompatible field is added or removed.
const busMetaCurrentVersion = 1

// NewBusMeta is a small constructor that fills in MetaVersion / StartedAt so
// callers don't accidentally write a meta file with the wrong version or a
// non-UTC timestamp.
func NewBusMeta(openIDHash, busVersion string, pid int) BusMeta {
	return BusMeta{
		MetaVersion: busMetaCurrentVersion,
		OpenIDHash:  openIDHash,
		StartedAt:   time.Now().UTC().Format(time.RFC3339),
		BusVersion:  busVersion,
		PID:         pid,
	}
}

// WriteBusMeta atomically replaces bus.meta with the given metadata.
//
// Atomicity strategy: write a sibling .tmp file, fsync it, then os.Rename
// over the destination.  Rename is atomic on POSIX (same dir) and on NTFS
// when the destination already exists; readers therefore observe either
// the old file or the new file but never a half-written one.
func WriteBusMeta(meta BusMeta) error {
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return exception.EventInternalError.With("bus meta: marshal: %v", err)
	}
	// trailing newline so cat-ing the file looks tidy and editors don't fight us.
	data = append(data, '\n')

	return atomicWriteFile(BusMetaFile(), data, "bus meta")
}

// ReadBusMeta reads bus.meta from BusDir().
//
// Returns:
//   - meta, true,  nil   — file exists and parses cleanly
//   - {},  false, nil    — file does not exist (no bus has ever run)
//   - {},  false, err    — file exists but is malformed; callers should treat
//     this as a corrupt-state signal (status reports
//     stale_owner, stop --force cleans up)
func ReadBusMeta() (BusMeta, bool, error) {
	path := BusMetaFile()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return BusMeta{}, false, nil
		}
		return BusMeta{}, false, exception.EventInternalError.With("bus meta: read %s: %v", path, err)
	}
	var meta BusMeta
	if err = json.Unmarshal(data, &meta); err != nil {
		return BusMeta{}, false, exception.EventInternalError.With("bus meta: parse %s: %v", path, err)
	}
	if meta.MetaVersion == 0 || meta.OpenIDHash == "" {
		return BusMeta{}, false, exception.EventInternalError.With("bus meta: %s missing required fields (version=%d, owner=%q)",
			path, meta.MetaVersion, meta.OpenIDHash)
	}
	return meta, true, nil
}

// RemoveBusMeta deletes bus.meta if present.  Idempotent: missing file is
// not an error.  Used by `event stop --force` to scrub the bus dir.
func RemoveBusMeta() error {
	err := os.Remove(BusMetaFile())
	if err == nil || os.IsNotExist(err) {
		return nil
	}
	return exception.EventInternalError.With("bus meta: remove: %v", err)
}
