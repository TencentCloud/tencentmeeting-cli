package meeting

import (
	"tmeet/internal"

	"github.com/spf13/cobra"
)

// NewBaseCmd is the meeting command.
func NewBaseCmd(tmeet *internal.Tmeet) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "meeting",
		Short: "about meeting operator cmd",
	}

	cmd.AddCommand(
		// Create meeting
		newCreateCmd(tmeet),
		// Cancel meeting
		newCancelCmd(tmeet),
		// Get meeting details
		newGetCmd(tmeet),
		// Update meeting
		newUpdateCmd(tmeet),
		// List meetings
		newListCmd(tmeet),
		// List ended meetings
		newListEndedCmd(tmeet),
		// List meeting invitees
		newInviteesCmd(tmeet),
	)
	return cmd
}
