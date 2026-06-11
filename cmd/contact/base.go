package contact

import (
	"tmeet/internal"

	"github.com/spf13/cobra"
)

// NewBaseCmd is the contact command.
func NewBaseCmd(tmeet *internal.Tmeet) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "contact",
		Short: "about contact operator cmd",
	}

	cmd.AddCommand(
		// Search contacts
		newSearchCmd(tmeet),
		// Look up users by phone number
		newLookupByPhoneCmd(tmeet),
		// Look up users by email address
		newLookupByEmailCmd(tmeet),
	)

	return cmd
}
