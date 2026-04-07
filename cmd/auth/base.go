package auth

import (
	"tmeet/internal"

	"github.com/spf13/cobra"
)

// NewBaseCmd creates the auth command.
func NewBaseCmd(tmeet *internal.Tmeet) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Authentication commands",
	}

	cmd.AddCommand(
		// Login
		newLoginCmd(tmeet),
		// Logout
		newLogoutCmd(tmeet),
		// Status query
		newStatusCmd(tmeet),
	)

	return cmd
}
