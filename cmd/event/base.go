package event

import (
	"github.com/spf13/cobra"

	"tmeet/internal"
)

// NewBaseCmd builds the top-level `event` command and attaches its subcommands.
//
// Mirrors the cmd/meeting/base.go pattern (NewBaseCmd(tmeet *internal.Tmeet))
// so registration in cmd/root.go is a one-liner.
//
// `event list` / `event schema` are read-only and can run without an active
// login.  `event status` reads only the local bus directory and likewise needs
// no login; both annotate themselves with skipPreCheck so users can poke at
// them before the first `tmeet auth login`.
//
// `_bus`, `consume`, `stop` will be wired in later batches.
func NewBaseCmd(tmeet *internal.Tmeet) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "event",
		Short: "Real-time event consumption",
		Long: `Real-time event consumption via a per-host bus daemon.

Subscribe to Tencent Meeting events (e.g. meeting.started) over a single
shared WebSocket connection managed by a background bus process.  Use
'event list' / 'event schema' to discover what is available, 'event consume'
to start a subscription, and 'event status' / 'event stop' to inspect or
terminate the bus.`,
	}

	cmd.AddCommand(
		newListCmd(tmeet),
		newSchemaCmd(tmeet),
		newStatusCmd(tmeet),
		newStopCmd(tmeet),
		newConsumeCmd(tmeet),
		newBusCmd(tmeet),
	)
	return cmd
}
