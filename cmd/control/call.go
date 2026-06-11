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

// CallOptions holds the options for calling meeting members.
type CallOptions struct {
	tmeet     *internal.Tmeet
	MeetingID string   // Meeting ID
	Users     []string // List of users to call, filled with user open_id
}

// newCallCmd calls users into a meeting.
func newCallCmd(tmeet *internal.Tmeet) *cobra.Command {
	opts := &CallOptions{tmeet: tmeet}
	cmd := &cobra.Command{
		Use:   "call",
		Short: "call meeting members (in-meeting invite call)",
		RunE: middleWare.Chain(
			opts.Run,
			middleWare.WithApiCmd(cmdutil.StaticApiCmd(cmdutil.ApiCmdControlCall)),
		),
	}

	// fill flags
	cmd.Flags().StringVar(&opts.MeetingID, "meeting-id", "", "meeting ID (required)")
	cmd.Flags().StringSliceVar(&opts.Users, "users", nil,
		"user open_id list to call, comma-separated or repeat the flag, max 20"+
			"eg. --users open_id1,open_id2 or --users open_id1 --users open_id2 (required)")

	// mark required
	_ = cmd.MarkFlagRequired("meeting-id")
	_ = cmd.MarkFlagRequired("users")

	return cmd
}

// Run executes the call command.
func (o *CallOptions) Run(cmd *cobra.Command, args []string) error {
	if len(o.Users) == 0 {
		return exception.InvalidArgsError.With("--users is required")
	}

	callList, err := cmdutil.PackageMeetingControlUsers("--users", o.Users)
	if err != nil {
		return err
	}

	params := map[string]interface{}{
		"operator_id":      o.tmeet.UserConfig.OpenId,
		"operator_id_type": 2, // OpenId
		"users":            callList,
	}

	req := &thttp.Request{
		ApiURI: "/v1/meetings/{meeting_id}/batch-call",
		Body:   params,
		PathParams: thttp.PathParams{
			"meeting_id": o.MeetingID,
		},
	}
	rsp, err := restProxy.RequestProxy(cmd.Context(), http.MethodPost, o.tmeet, req)
	if err != nil {
		return err
	}

	output.FormatPrint(cmd, rsp.TraceId, rsp.Message, rsp.Data)
	return nil
}
