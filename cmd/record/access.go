package record

import (
	"tmeet/internal"
	"tmeet/internal/cmdutil"
	middleWare "tmeet/internal/cmdutil/middleware"

	"github.com/spf13/cobra"
)

// Access sub-action values.
const (
	accessActionAddress  = "address"
	accessActionSharing  = "sharing"
	accessActionMembers  = "members"
	accessActionSettings = "settings"
)

// accessLongDoc is the --help long description of `tmeet record access`.
// Kept as a package-level constant next to the command shell so that all
// dispatch-time facts (action names, flag layout) live in one place and
// stay in sync with newAccessCmd's flag registration.
const accessLongDoc = "Query record access-related information. Choose one of the following via --action.\n" +
	"\n" +
	"Actions and their scenarios:\n" +
	"  address  - List record file download / playback URLs for a meeting record\n" +
	"             (paginated), typically used to obtain playable / downloadable links.\n" +
	"  sharing  - Query the sharing settings of a recording (share switches / password /\n" +
	"             expiration / download permission / etc.).\n" +
	"  members  - List members authorized to view the recording (paginated by\n" +
	"             --page-token), typically used to audit access grants.\n" +
	"  settings - Query the smart recording config of a record file (subtitle switch /\n" +
	"             AI switch / support flag).\n" +
	"\n" +
	"Actions and their flags (required = <>, optional = []):\n" +
	"  --action address  <--meeting-record-id> [--page-token] [--page-size]\n" +
	"  --action sharing  <--meeting-record-id>\n" +
	"  --action members  <--meeting-record-id> [--page-token]\n" +
	"  --action settings <--record-file-id>"

// AccessOptions holds the union of options for `tmeet record access`. The
// meaning of each flag depends on --action; per-action semantics are enforced
// by each action handler (see access_<action>.go files).
type AccessOptions struct {
	tmeet *internal.Tmeet

	Action string // required: address | sharing | members | settings

	MeetingRecordID string // required for address / sharing / members
	RecordFileID    string // required for settings

	PageSize  int    // used by address
	PageToken string // used by address / members
}

// newAccessCmd builds the `tmeet record access` command. This is a pure
// command shell: it wires flags, assembles the ActionRegistry from
// per-action build functions (declared in access_<action>.go), then hands
// dispatch off to cmdutil.ActionRegistry.Dispatch.
func newAccessCmd(tmeet *internal.Tmeet) *cobra.Command {
	opts := &AccessOptions{tmeet: tmeet}

	registry := cmdutil.ActionRegistry{
		accessActionAddress:  opts.buildAddressHandler(),
		accessActionSharing:  opts.buildSharingHandler(),
		accessActionMembers:  opts.buildMembersHandler(),
		accessActionSettings: opts.buildSettingsHandler(),
	}

	cmd := &cobra.Command{
		Use:   "access",
		Short: "query record access-related info (--action address|sharing|members|settings)",
		Long:  accessLongDoc,
		RunE: middleWare.Chain(
			func(cmd *cobra.Command, args []string) error {
				return registry.Dispatch(cmd, opts.Action, "action")
			},
			middleWare.WithApiCmd(registry.BuildApiCmdResolver("action")),
			middleWare.WithCompact(tmeet),
		),
	}

	registerAccessFlags(cmd, opts)
	return cmd
}

// registerAccessFlags declares the union of flags consumed by any access
// action. Per-action rules on which flags are required / forbidden are
// enforced by each handler's Validate + FlagRules.
func registerAccessFlags(cmd *cobra.Command, opts *AccessOptions) {
	cmdutil.SingleStringVar(cmd.Flags(), &opts.Action, "action", "",
		"access type: address | sharing | members | settings (required)")
	_ = cmd.MarkFlagRequired("action")

	cmd.Flags().StringVar(&opts.MeetingRecordID, "meeting-record-id", "",
		"meeting record id (required for address/sharing/members)")
	cmd.Flags().StringVar(&opts.RecordFileID, "record-file-id", "",
		"record file id (required for settings)")

	cmd.Flags().IntVar(&opts.PageSize, "page-size", 30,
		"page size (address only, default 30, max 30)")
	cmd.Flags().StringVar(&opts.PageToken, "page-token", "",
		"page token for pagination (address/members)")
}
