package meeting

import (
	"net/http"
	"tmeet/internal"
	"tmeet/internal/cmdutil"
	middleWare "tmeet/internal/cmdutil/middleware"
	"tmeet/internal/core/thttp"
	"tmeet/internal/exception"
	"tmeet/internal/output"
	restProxy "tmeet/internal/proxy/rest-proxy"

	"github.com/spf13/cobra"
)

// InviteesAddOptions holds the options for adding invitees to a meeting.
type InviteesAddOptions struct {
	tmeet     *internal.Tmeet
	MeetingID string   // Meeting ID
	Invitees  []string // List of invitees to add, filled with user open_id
}

// newInviteesAddCmd adds invitees to a meeting.
func newInviteesAddCmd(tmeet *internal.Tmeet) *cobra.Command {
	opts := &InviteesAddOptions{tmeet: tmeet}
	cmd := &cobra.Command{
		Use:   "invitees-add",
		Short: "add meeting invitees",
		RunE: middleWare.Chain(
			opts.Run,
			middleWare.WithApiCmd(cmdutil.StaticApiCmd(cmdutil.ApiCmdMeetingInviteAdd)),
		),
	}

	// fill flags
	cmd.Flags().StringVar(&opts.MeetingID, "meeting-id", "", "meeting ID (required)")
	cmd.Flags().StringSliceVar(&opts.Invitees, "invitees", nil,
		"new invitee open_id list, comma-separated or repeat the flag, max 100"+
			"eg. --invitees open_id1,open_id2 or --invitees open_id1 --invitees open_id2 (required)")

	// mark required
	_ = cmd.MarkFlagRequired("meeting-id")
	_ = cmd.MarkFlagRequired("invitees")

	return cmd
}

// Run executes the invite add command.
func (o *InviteesAddOptions) Run(cmd *cobra.Command, args []string) error {
	if len(o.Invitees) == 0 {
		return exception.InvalidArgsError.With("--invitees is required")
	}

	inviteesList, err := cmdutil.PackageApiInviteesUsers("--invitees", o.Invitees)
	if err != nil {
		return err
	}

	params := map[string]interface{}{
		"operator_id":      o.tmeet.UserConfig.OpenId,
		"operator_id_type": 2, // OpenId
		"instanceid":       1, // PC, fixed value
		"add_invitees":     inviteesList,
	}

	req := &thttp.Request{
		ApiURI: "/v1/meetings/{meeting_id}/modify-invitees",
		Body:   params,
		PathParams: thttp.PathParams{
			"meeting_id": o.MeetingID,
		},
	}
	rsp, err := restProxy.RequestProxy(cmd.Context(), http.MethodPut, o.tmeet, req)
	if err != nil {
		return err
	}

	output.FormatPrint(cmd, rsp.TraceId, rsp.Message, rsp.Data)
	return nil
}
