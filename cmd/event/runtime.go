// Package event hosts the `tmeet event` command group.
//
// It is intentionally thin: each subcommand is a small file (list.go, schema.go,
// status.go, ...) calling helpers in the runtime sibling package
// (tmeet/internal/event).  Output is rendered as plain JSON / pretty JSON
// directly from the command — the global output.FormatPrint envelope
// (`{trace_id, message, data}`) does not fit the shapes the event family
// emits (e.g. `event list` returns a top-level JSON array), so this package
// emits raw structures via output.EventPrint, and informational stderr
// diagnostics via output.EventStderr.
package event

import (
	"tmeet/internal/config"
	eventruntime "tmeet/internal/event"
)

// currentOpenIDHash returns the OpenIDHash of the locally-logged-in user, or
// an empty string when no user is logged in.
//
// Used by `event status` and `event stop` to compare against bus.meta's
// owner_hash — a mismatch is the signal for the "stale_owner" state:
// bus is alive but bound to a previous user identity, the new login has
// a different OpenId.
//
// The function never errors: a missing UserConfig is a legitimate state
// (status can be invoked after `auth logout`) and we treat it as "no current
// user", which downgrades stale_owner to a hint without crashing.
func currentOpenIDHash() string {
	usr, err := config.GetUserConfig()
	if err != nil || usr == nil {
		return ""
	}
	return eventruntime.OpenIDHash(usr.OpenId)
}
