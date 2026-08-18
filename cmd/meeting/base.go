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
		// Search meetings
		newSearchCmd(tmeet),
		// List meeting invitees
		newInviteesCmd(tmeet),
		// Add meeting invitees
		newInviteesAddCmd(tmeet),
		// Remove meeting invitees
		newInviteesRemoveCmd(tmeet),
		// Replace meeting invitees
		newInviteesReplaceCmd(tmeet),
		// Join as agent: agent joins a meeting to listen and auto-enable real-time transcription
		newJoinAsAgentCmd(tmeet),
		// Leave as agent: agent leaves a meeting it previously joined
		newLeaveAsAgentCmd(tmeet),
	)
	return cmd
}
