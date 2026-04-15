package meeting

import (
	"net/http"
	"strconv"
	"tmeet/internal"
	"tmeet/internal/core/thttp"
	"tmeet/internal/log"
	restProxy "tmeet/internal/proxy/rest-proxy"

	"github.com/spf13/cobra"
)

// InviteesOptions holds the options for getting meeting invitees.
type InviteesOptions struct {
	tmeet     *internal.Tmeet
	MeetingID string // Meeting ID
	Pos       int    // Starting position value for paginated retrieval of invited members
}

// newInviteesCmd gets meeting invitees.
func newInviteesCmd(tmeet *internal.Tmeet) *cobra.Command {
	opts := &InviteesOptions{tmeet: tmeet}
	cmd := &cobra.Command{
		Use:   "invitees-list",
		Short: "get meeting invitees",
		RunE: func(cmd *cobra.Command, args []string) error {
			return opts.Run(cmd, args)
		},
	}

	// 填充参数
	cmd.Flags().StringVar(&opts.MeetingID, "meeting-id", "", "meeting ID (required)")
	cmd.Flags().IntVar(&opts.Pos, "pos", 0, "pos starting position value for retrieving the list of invited members in pagination.")

	// 设置必填参数
	_ = cmd.MarkFlagRequired("meeting-id")

	return cmd
}

func (o *InviteesOptions) Run(cmd *cobra.Command, args []string) error {
	queryParams := thttp.QueryParams{}
	queryParams.Set("userid", o.tmeet.UserConfig.OpenId)
	queryParams.Set("instanceid", "1") // PC, fixed value
	queryParams.Set("pos", strconv.FormatInt(int64(o.Pos), 10))

	req := &thttp.Request{
		ApiURI:      "/v1/meetings/{meeting_id}/invitees",
		QueryParams: queryParams,
		PathParams: thttp.PathParams{
			"meeting_id": o.MeetingID,
		},
	}
	rsp, err := restProxy.RequestProxy(cmd.Context(), http.MethodGet, o.tmeet, req)
	if err != nil {
		return err
	}

	log.FormatPrint(cmd, rsp.TraceId, rsp.Message, rsp.Data)
	return nil
}
