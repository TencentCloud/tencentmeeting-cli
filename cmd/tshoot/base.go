package tshoot

import (
	"tmeet/internal"

	"github.com/spf13/cobra"
)

// NewBaseCmd is the tshoot command.
func NewBaseCmd(tmeet *internal.Tmeet) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tshoot",
		Short: "about troubleshoot operator cmd",
	}

	cmd.AddCommand(
		// Get meeting log
		newLogCmd(tmeet),
		// Report troubleshooting feedback
		newFeedbackCmd(tmeet),
	)

	return cmd
}
