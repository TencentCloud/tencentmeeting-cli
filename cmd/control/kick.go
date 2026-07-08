package control

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

// KickOptions holds the options for kicking members out of a meeting.
type KickOptions struct {
	tmeet       *internal.Tmeet
	MeetingID   string   // Meeting ID
	AllowRejoin bool     // Allow re join after kick out
	Users       []string // List of users to kick out, filled with user open_id
	SipUsers    []string // List of sip users to kick out, filled with user ms_open_id
	PstnUsers   []string // List of pstn users to kick out, filled with user ms_open_id
}

// newKickCmd kicks users out of a meeting.
func newKickCmd(tmeet *internal.Tmeet) *cobra.Command {
	opts := &KickOptions{tmeet: tmeet}
	cmd := &cobra.Command{
		Use:   "kick",
		Short: "kick meeting members out (in-meeting kick-out)",
		RunE: middleWare.Chain(
			opts.Run,
			middleWare.WithApiCmd(cmdutil.StaticApiCmd(cmdutil.ApiCmdControlKick)),
		),
	}

	// fill flags
	cmd.Flags().StringVar(&opts.MeetingID, "meeting-id", "", "meeting ID (required)")
	cmd.Flags().BoolVar(&opts.AllowRejoin, "allow-rejoin", true,
		"allow kicked-out members to rejoin the meeting, default true. "+
			"pass --allow-rejoin=false to disallow rejoin")
	cmd.Flags().StringSliceVar(&opts.Users, "users", nil,
		"user open_id list to kick out, not contains sip/pstn device, comma-separated or repeat the flag, "+
			"the total number of --users/--sip-users/--pstn-users is max 20. "+
			"eg. --users open_id1,open_id2 or --users open_id1 --users open_id2")
	cmd.Flags().StringSliceVar(&opts.SipUsers, "sip-users", nil,
		"sip user ms_open_id list to kick out, comma-separated or repeat the flag, "+
			"the total number of --users/--sip-users/--pstn-users is max 20. "+
			"eg. --sip-users ms_open_id1,ms_open_id2 or --sip-users ms_open_id1 --sip-users ms_open_id2")
	cmd.Flags().StringSliceVar(&opts.PstnUsers, "pstn-users", nil,
		"pstn user ms_open_id list to kick out, comma-separated or repeat the flag, "+
			"the total number of --users/--sip-users/--pstn-users is max 20. "+
			"eg. --pstn-users ms_open_id1,ms_open_id2 or --pstn-users ms_open_id1 --pstn-users ms_open_id2")

	// mark required
	_ = cmd.MarkFlagRequired("meeting-id")
	cmd.MarkFlagsOneRequired("users", "sip-users", "pstn-users")

	return cmd
}

// Run executes the kick command.
func (o *KickOptions) Run(cmd *cobra.Command, args []string) error {
	if len(o.Users) == 0 && len(o.SipUsers) == 0 && len(o.PstnUsers) == 0 {
		return exception.InvalidArgsError.With("--users/--sip-users/--pstn-users, at least one of them is required")
	}

	if total := len(o.Users) + len(o.SipUsers) + len(o.PstnUsers); total > cmdutil.MeetingControlUsersListMax {
		return exception.InvalidArgsError.With(
			"the total number of --users/--sip-users/--pstn-users is too long, max is %d, got %d",
			cmdutil.MeetingControlUsersListMax, total)
	}

	kickList, err := cmdutil.PackageMeetingControlUsers("--users", o.Users)
	if err != nil {
		return err
	}
	kickSipList, err := cmdutil.PackageMeetingControlSpecialUsers("--sip-users", o.SipUsers, 9)
	if err != nil {
		return err
	}
	kickPstnList, err := cmdutil.PackageMeetingControlSpecialUsers("--pstn-users", o.PstnUsers, 0)
	if err != nil {
		return err
	}
	kickList = append(kickList, kickSipList...)
	kickList = append(kickList, kickPstnList...)

	params := map[string]interface{}{
		"operator_id":      o.tmeet.UserConfig.OpenId,
		"operator_id_type": 2, // OpenId
		"instanceid":       1, // PC, fixed value
		"allow_rejoin":     o.AllowRejoin,
		"users":            kickList,
	}

	req := &thttp.Request{
		ApiURI: "/v1/real-control/meetings/{meeting_id}/kickout",
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
