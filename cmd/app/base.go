package app

import (
	"tmeet/internal"

	"github.com/spf13/cobra"
)

// NewBaseCmd is the app command.
func NewBaseCmd(tmeet *internal.Tmeet) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "app",
		Short: "about app operator cmd",
	}

	cmd.AddCommand(
		// Set app info
		newSetCmd(tmeet),
		// Get app info
		newGetCmd(tmeet),
	)
	return cmd
}
