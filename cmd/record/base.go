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
		// Get record sharing settings
		newSettingsGetCmd(tmeet),
		// Download a recording file from the official download address
		newDownloadCmd(tmeet),
		// Search records
		newSearchCmd(tmeet),
		// Get smart minutes
		newSmartMinutesCmd(tmeet),
		// Get transcript details
		newTranscriptGetCmd(tmeet),
		// Get transcript paragraphs
		newTranscriptParagraphsCmd(tmeet),
		// Search transcript content
		newTranscriptSearchCmd(tmeet),
		// Preview record permission application
		newPermissionApplyPrepareCmd(tmeet),
		// Commit record permission application
		newPermissionApplyCommitCmd(tmeet),
	)

	return cmd
}
