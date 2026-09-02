package minute

import (
	"tmeet/internal"

	"github.com/spf13/cobra"
)

// NewBaseCmd is the minute-related command.
func NewBaseCmd(tmeet *internal.Tmeet) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "minute",
		Short: "minute related commands",
	}

	cmd.AddCommand(
		// Search minutes
		newSearchCmd(tmeet),
		// Get minutes detail
		newGetCmd(tmeet),
	)

	return cmd
}
