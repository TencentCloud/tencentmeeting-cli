package meeting

import (
	"net/http"
	"strconv"
	"tmeet/internal"
	"tmeet/internal/cmdutil"
	middleWare "tmeet/internal/cmdutil/middleware"
	"tmeet/internal/core/thttp"
	"tmeet/internal/output"
	restProxy "tmeet/internal/proxy/rest-proxy"

	"github.com/spf13/cobra"
)

// InviteesOptions holds the options for getting meeting invitees.
type InviteesOptions struct {
	tmeet     *internal.Tmeet
	MeetingID string // Meeting ID
	Pos       int    // Starting position value for paginated retrieval of invited members, deprecated use PageToken instead
	PageToken string // Page token for pagination
	PageSize  int    // Page size for pagination, max 30
}

// newInviteesCmd gets meeting invitees.
func newInviteesCmd(tmeet *internal.Tmeet) *cobra.Command {
	opts := &InviteesOptions{tmeet: tmeet}
	cmd := &cobra.Command{
		Use:   "invitees-list",
		Short: "get meeting invitees",
		RunE: middleWare.Chain(
			opts.Run,
			middleWare.WithApiCmd(cmdutil.StaticApiCmd(cmdutil.ApiCmdMeetingInviteList)),
		),
	}

	// 填充参数
	cmd.Flags().StringVar(&opts.MeetingID, "meeting-id", "", "meeting ID (required)")
	cmd.Flags().IntVar(&opts.Pos, "pos", -1, ""+
		"pos starting position value for retrieving the list of invited members in pagination. (deprecated, use --page-token instead)")
	cmd.Flags().StringVar(&opts.PageToken, "page-token", "", "page token for pagination")
	cmd.Flags().IntVar(&opts.PageSize, "page-size", 30, "page size for pagination, max 30, default 30")

	// mark required
	_ = cmd.MarkFlagRequired("meeting-id")
	// mark deprecated
	_ = cmd.Flags().MarkDeprecated("pos", "use --page-token instead")

	return cmd
}

func (o *InviteesOptions) Run(cmd *cobra.Command, args []string) error {
	queryParams := thttp.QueryParams{}
	queryParams.Set("userid", o.tmeet.UserConfig.OpenId)
	queryParams.Set("instanceid", "1") // PC, fixed value

	// pagination compatibility
	pageValue, pageType := cmdutil.ChoosePosOrToken(o.Pos, o.PageToken)
	queryParams.Set("page_type", strconv.Itoa(pageType))
	if pageType == cmdutil.PageTypeOld {
		queryParams.Set("pos", pageValue)
	} else {
		queryParams.Set("page_token", pageValue)
		pageSize, err := cmdutil.ClampingPageSize(cmd, o.PageSize, cmdutil.PageSizeMaxMeetings)
		if err != nil {
			return err
		}
		queryParams.Set("page_size", strconv.Itoa(pageSize))
	}

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

	output.FormatPrint(cmd, rsp.TraceId, rsp.Message, rsp.Data)
	return nil
}
