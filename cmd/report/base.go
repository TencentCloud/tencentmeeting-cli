package report

import (
	"tmeet/internal"

	"github.com/spf13/cobra"
)

// NewBaseCmd is the report command.
func NewBaseCmd(tmeet *internal.Tmeet) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "report",
		Short: "about report operator cmd",
	}

	cmd.AddCommand(
		// Get participants list
		newParticipantsCmd(tmeet),
		// Get waiting room members list
		newWaitingRoomCmd(tmeet),
	)

	return cmd
}
