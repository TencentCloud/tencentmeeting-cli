package control

import (
	"tmeet/internal"

	"github.com/spf13/cobra"
)

// NewBaseCmd is the control command.
func NewBaseCmd(tmeet *internal.Tmeet) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "control",
		Short: "about in-meeting control operator cmd",
	}

	cmd.AddCommand(
		// Call meeting members
		newCallCmd(tmeet),
		// Kick meeting members out
		newKickCmd(tmeet),
	)

	return cmd
}
