package record

import (
	"tmeet/internal"

	"github.com/spf13/cobra"
)

// NewBaseCmd is the record-related command.
//
// The record command group has been consolidated from 17 legacy sub-commands
// to 6 (see iwiki.woa.com/p/4041907759):
//   - search           : find / search records (list + search merged)
//   - content          : read record content, dispatched via --action
//   - access           : query access info, dispatched via --action
//   - share-set        : modify sharing config & members (write)
//   - settings-set     : modify subtitle / AI switches (write)
//   - permission-apply : apply for record permission (--action prepare|commit)
func NewBaseCmd(tmeet *internal.Tmeet) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "record",
		Short: "record related commands",
	}

	cmd.AddCommand(
		// 1. Find / search records (list + search merged).
		newSearchCmd(tmeet),
		// 2. Read record content: --action minutes|todos|transcript|paragraphs|timeline.
		newContentCmd(tmeet),
		// 3. Query access info: --action address|sharing|members|settings.
		newAccessCmd(tmeet),
		// 4. Modify sharing config & members (write op, confirm before executing).
		newShareSetCmd(tmeet),
		// 5. Modify subtitle / AI switches (write op, confirm before executing).
		newSettingsSetCmd(tmeet),
		// 6. Apply for record permission: --action prepare|commit (default prepare).
		newPermissionApplyCmd(tmeet),
	)

	return cmd
}
