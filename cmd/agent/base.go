package agent

import (
	"tmeet/internal"

	"github.com/spf13/cobra"
)

// NewBaseCmd creates the agent command.
func NewBaseCmd(tmeet *internal.Tmeet) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent",
		Short: "Sub-account (agent) commands",
	}

	cmd.AddCommand(
		// Create
		newCreateCmd(tmeet),
		// Delete
		newDeleteCmd(tmeet),
		// Token
		newTokenCmd(tmeet),
		// List
		newListCmd(tmeet),
		// Get
		newGetCmd(tmeet),
	)

	return cmd
}
