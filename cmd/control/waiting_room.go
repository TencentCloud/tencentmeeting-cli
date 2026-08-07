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
	"tmeet/internal/utils/enumerate"

	"github.com/spf13/cobra"
)

// WaitingRoomOptions holds the options for waiting room management.
type WaitingRoomOptions struct {
	tmeet       *internal.Tmeet
	MeetingID   string   // Meeting ID
	OperateType string   // Operation type: enter-meeting, back-to-waiting, expel
	AllowRejoin bool     // Allow rejoin after expel (only valid when operate_type=expel)
	Users       []string // List of users, filled with user open_id
	SipUsers    []string // List of sip users, filled with user ms_open_id
	PstnUsers   []string // List of pstn users, filled with user ms_open_id
}

// newWaitingRoomCmd manages waiting room members in a meeting.
func newWaitingRoomCmd(tmeet *internal.Tmeet) *cobra.Command {
	opts := &WaitingRoomOptions{tmeet: tmeet}
	cmd := &cobra.Command{
		Use:   "waiting-room",
		Short: "manage waiting room members in the meeting (in-meeting waiting room control)",
		Long: `Manage waiting room members in a meeting. Supports three operation types:
  - enter-meeting: Admit waiting room members into the meeting (host moves waiting room members into the meeting)
  - back-to-waiting: Move in-meeting members back to the waiting room (host moves in-meeting members into the waiting room)
  - expel: Expel waiting room members from the meeting (host removes waiting room members)`,
		RunE: middleWare.Chain(
			opts.Run,
			middleWare.WithApiCmd(cmdutil.StaticApiCmd(cmdutil.ApiCmdControlWaitingRoom)),
		),
	}

	// fill flags
	cmd.Flags().StringVar(&opts.MeetingID, "meeting-id", "", "meeting ID (required)")
	cmd.Flags().Var(&cmdutil.EnumValue{Value: &opts.OperateType, Allowed: []string{"enter-meeting", "back-to-waiting", "expel"}}, "operate-type",
		"operation type (required): enter-meeting (host admits waiting room members into the meeting), back-to-waiting (host moves in-meeting members back to the waiting room), expel (host expels waiting room members from the meeting)")
	cmd.Flags().BoolVar(&opts.AllowRejoin, "allow-rejoin", false,
		"allow rejoin after expel (only valid when --operate-type=expel). true: allow, false: disallow (default). "+
			"eg. --operate-type=expel --allow-rejoin=true or --operate-type=expel --allow-rejoin=false")
	cmd.Flags().StringSliceVar(&opts.Users, "users", nil,
		"user open_id list to operate, not contains sip/pstn device, comma-separated or repeat the flag, "+
			"the total number of --users/--sip-users/--pstn-users is max 20. "+
			"eg. --users open_id1,open_id2 or --users open_id1 --users open_id2")
	cmd.Flags().StringSliceVar(&opts.SipUsers, "sip-users", nil,
		"sip user ms_open_id list to operate, comma-separated or repeat the flag, "+
			"the total number of --users/--sip-users/--pstn-users is max 20. "+
			"eg. --sip-users ms_open_id1,ms_open_id2 or --sip-users ms_open_id1 --sip-users ms_open_id2")
	cmd.Flags().StringSliceVar(&opts.PstnUsers, "pstn-users", nil,
		"pstn user ms_open_id list to operate, comma-separated or repeat the flag, "+
			"the total number of --users/--sip-users/--pstn-users is max 20. "+
			"eg. --pstn-users ms_open_id1,ms_open_id2 or --pstn-users ms_open_id1 --pstn-users ms_open_id2")

	// mark required
	_ = cmd.MarkFlagRequired("meeting-id")
	_ = cmd.MarkFlagRequired("operate-type")
	cmd.MarkFlagsOneRequired("users", "sip-users", "pstn-users")

	return cmd
}

// Run executes the waiting room command.
func (o *WaitingRoomOptions) Run(cmd *cobra.Command, args []string) error {
	if len(o.Users) == 0 && len(o.SipUsers) == 0 && len(o.PstnUsers) == 0 {
		return exception.InvalidArgsError.With("--users/--sip-users/--pstn-users, at least one of them is required")
	}

	if total := len(o.Users) + len(o.SipUsers) + len(o.PstnUsers); total > cmdutil.MeetingControlUsersListMax {
		return exception.InvalidArgsError.With(
			"the total number of --users/--sip-users/--pstn-users is too long, max is %d, got %d",
			cmdutil.MeetingControlUsersListMax, total)
	}

	admitList, err := cmdutil.PackageMeetingControlUsers("--users", o.Users)
	if err != nil {
		return err
	}
	admitSipList, err := cmdutil.PackageMeetingControlSpecialUsers("--sip-users", o.SipUsers, 9)
	if err != nil {
		return err
	}
	admitPstnList, err := cmdutil.PackageMeetingControlSpecialUsers("--pstn-users", o.PstnUsers, 0)
	if err != nil {
		return err
	}
	admitList = append(admitList, admitSipList...)
	admitList = append(admitList, admitPstnList...)

	// --allow-rejoin only takes effect when --operate-type=expel
	allowRejoinChanged := cmd.Flags().Changed("allow-rejoin")
	if allowRejoinChanged && o.OperateType != "expel" {
		return exception.InvalidArgsError.With("--allow-rejoin only takes effect when --operate-type=expel")
	}

	// map string operate type to downstream API uint32 enum
	operateType, ok := enumerate.WaitingRoomOperateTypeValue(o.OperateType)
	if !ok {
		return exception.InvalidArgsError.With("invalid --operate-type: %s", o.OperateType)
	}

	params := map[string]interface{}{
		"operator_id":      o.tmeet.UserConfig.OpenId,
		"operator_id_type": 2, // OpenId
		"instanceid":       1, // PC, fixed value
		"operate_type":     uint32(operateType),
		"users":            admitList,
	}
	// only set allow_rejoin when user explicitly specified it; otherwise omit the field
	if allowRejoinChanged {
		params["allow_rejoin"] = o.AllowRejoin
	}

	req := &thttp.Request{
		ApiURI: "/v1/real-control/meetings/{meeting_id}/waiting-room",
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
