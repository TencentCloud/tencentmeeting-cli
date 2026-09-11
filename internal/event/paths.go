// Package event hosts the tmeet event subsystem: command-side helpers (registry,
// schemas) and the runtime artefacts (paths, protocol, transport) shared by the
// bus daemon, consumer command and control-plane commands (status / stop).
//
// All filesystem locations are global-singleton under config.GetConfigDir()/event/.
// OpenId is never embedded into pathnames; ownership is verified through bus.meta.
package event

import (
	"path/filepath"
	"runtime"

	"tmeet/internal/config"
)

// File names under BusDir(). Kept as constants so future renames touch one place.
const (
	fileBusSock  = "bus.sock" // unix socket; on Windows BusSockPath returns a named pipe address instead
	fileBusPID   = "bus.pid"  // line1: pid; line2: RFC3339 start time
	fileBusAlive = "bus.alive.lock"
	fileBusFork  = "bus.fork.lock"
	fileBusMeta  = "bus.meta" // JSON {openid_hash, started_at, version}
	fileBusLog   = "bus.log"  // bus daemon stdout/stderr redirect
	fileWSState  = "ws.state" // optional diagnostic snapshot written by the WSS source
)

// windowsPipePrefix is the Named Pipe namespace for the bus IPC endpoint on Windows.
// The actual pipe object lives in the kernel; only an address string is needed.
const windowsPipePrefix = `\\.\pipe\tmeet-event-bus`

// BusDir returns the global-singleton directory holding all event runtime artefacts.
// It honours TMEET_CLI_CONFIG_DIR via config.GetConfigDir() so tests can sandbox.
func BusDir() string {
	return filepath.Join(config.GetConfigDir(), "event")
}

// BusSockPath returns the address consumers Dial to reach the bus.
//
// On unix this is a real filesystem socket inside BusDir().
// On Windows the bus has no on-disk socket file; the address is a Named Pipe path
// in the kernel namespace. This helper hides the platform difference so callers
// can pass the result straight into transport.IPC.Dial / Listen.
func BusSockPath() string {
	if runtime.GOOS == "windows" {
		return windowsPipePrefix
	}
	return filepath.Join(BusDir(), fileBusSock)
}

// BusPIDFile returns the path to bus.pid (atomic-replace written by the bus on startup).
func BusPIDFile() string {
	return filepath.Join(BusDir(), fileBusPID)
}

// BusAliveLock returns the process-lifetime exclusive lock used as the truth for
// "is the bus alive?" (TryLock => ErrHeld means alive).
func BusAliveLock() string {
	return filepath.Join(BusDir(), fileBusAlive)
}

// BusForkLock returns the mutex that serialises concurrent bus-fork attempts
// from competing consume processes.
func BusForkLock() string {
	return filepath.Join(BusDir(), fileBusFork)
}

// BusMetaFile returns the owner-metadata file (openid_hash, started_at, version).
// Owner mismatch between Hello.openid_hash and bus.meta drives the WrongOwner /
// stale_owner detection.
func BusMetaFile() string {
	return filepath.Join(BusDir(), fileBusMeta)
}

// BusLogFile returns the path the bus daemon redirects its stdout/stderr to.
func BusLogFile() string {
	return filepath.Join(BusDir(), fileBusLog)
}

// WSStateFile returns an optional diagnostic file the WSS source may write to.
// Not required for batch 1 — exposed here so all paths live in one place.
func WSStateFile() string {
	return filepath.Join(BusDir(), fileWSState)
}
