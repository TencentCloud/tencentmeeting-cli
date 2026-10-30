package update

import (
	"tmeet/internal"

	"github.com/spf13/cobra"
)

// NewBaseCmd builds the `tmeet update` command tree.
//
// Four subcommands are intentionally kept separate rather than one command
// with a slew of flags, mirroring the auth/meeting/record style that is
// already familiar to Agent-side skills:
//
//	tmeet update check            inspect the cached version info (passive)
//	tmeet update get              dump the local cache + preference
//	tmeet update set              write and report the user's preference
//	tmeet update run              actually perform the upgrade
//
// All four require the user to be logged in — `tmeet update get` is
// pure-local but the files it reads are keyed by OpenId, so without a
// login there is nothing meaningful to show. The default PreCheck chain
// in cmd/root.go handles that; none of the subcommands opts out.
func NewBaseCmd(tmeet *internal.Tmeet) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: "manage tmeet CLI version check and upgrade",
	}

	cmd.AddCommand(
		newCheckCmd(tmeet),
		newGetCmd(tmeet),
		newSetCmd(tmeet),
		newRunCmd(tmeet),
	)
	return cmd
}
