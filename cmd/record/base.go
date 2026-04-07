package record

import (
	"tmeet/internal"

	"github.com/spf13/cobra"
)

// NewBaseCmd is the record-related command.
func NewBaseCmd(tmeet *internal.Tmeet) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "record",
		Short: "record related commands",
	}

	cmd.AddCommand(
		// List records
		newListCmd(tmeet),
		// Get record download address
		newAddressCmd(tmeet),
		// Get smart minutes
		newSmartMinutesCmd(tmeet),
		// Get transcript details
		newTranscriptGetCmd(tmeet),
		// Get transcript paragraphs
		newTranscriptParagraphsCmd(tmeet),
		// Search transcript content
		newTranscriptSearchCmd(tmeet),
	)

	return cmd
}
