package meeting

import (
	"net/http"
	"tmeet/internal"
	"tmeet/internal/core/thttp"
	"tmeet/internal/log"
	restProxy "tmeet/internal/proxy/rest-proxy"

	"github.com/spf13/cobra"
)

// CancelOptions holds the options for canceling a meeting.
type CancelOptions struct {
	tmeet        *internal.Tmeet
	MeetingID    string // Meeting ID
	SubMeetingID string // Sub-meeting ID for canceling a specific recurring sub-meeting
	MeetingType  int    // Meeting type, default 0. 0: normal meeting, 1: recurring meeting
}

// newCancelCmd cancels a meeting.
func newCancelCmd(tmeet *internal.Tmeet) *cobra.Command {
	opts := &CancelOptions{tmeet: tmeet}
	cmd := &cobra.Command{
		Use:   "cancel",
		Short: "cancel meeting",
		RunE: func(cmd *cobra.Command, args []string) error {
			return opts.Run(cmd, args)
		},
	}

	// 填充参数
	cmd.Flags().StringVar(&opts.MeetingID, "meeting-id", "", "meeting ID (required)")
	cmd.Flags().StringVar(&opts.SubMeetingID, "sub-meeting-id", "", "sub-meeting ID for canceling a specific recurring meeting")
	cmd.Flags().IntVar(&opts.MeetingType, "meeting-type", 0, "meeting type: 0-normal/single sub-meeting, 1-entire recurring meeting")

	// 设置必填参数
	_ = cmd.MarkFlagRequired("meeting-id")

	return cmd
}

func (o *CancelOptions) Run(cmd *cobra.Command, args []string) error {
	params := map[string]interface{}{
		"userid":       o.tmeet.UserConfig.OpenId,
		"reason_code":  0, // fixed value
		"instanceid":   1, // PC, fixed value
		"meetingId":    o.MeetingID,
		"meeting_type": o.MeetingType,
	}
	if o.SubMeetingID != "" {
		params["sub_meeting_id"] = o.SubMeetingID
	}

	req := &thttp.Request{
		ApiURI: "/v1/meetings/{meetingId}/cancel",
		Body:   params,
		PathParams: thttp.PathParams{
			"meetingId": o.MeetingID,
		},
	}
	rsp, err := restProxy.RequestProxy(cmd.Context(), http.MethodPost, o.tmeet, req)
	if err != nil {
		return err
	}

	log.FormatPrint(cmd, rsp.TraceId, rsp.Message, rsp.Data)
	return nil
}
