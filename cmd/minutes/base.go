package minutes

import (
	"tmeet/internal"

	"github.com/spf13/cobra"
)

// NewBaseCmd is the minutes-related command.
func NewBaseCmd(tmeet *internal.Tmeet) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "minutes",
		Short: "minutes related commands",
	}

	cmd.AddCommand(
		// Search minutes
		newSearchCmd(tmeet),
		// Get minutes detail
		newGetCmd(tmeet),
	)

	return cmd
}
