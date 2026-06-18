package meeting

import (
	"net/http"
	"strconv"
	"tmeet/internal"
	"tmeet/internal/cmdutil"
	middleWare "tmeet/internal/cmdutil/middleware"
	"tmeet/internal/core/thttp"
	"tmeet/internal/exception"
	"tmeet/internal/output"
	restProxy "tmeet/internal/proxy/rest-proxy"
	"tmeet/internal/utils"

	"github.com/spf13/cobra"
)

// ListOptions holds the options for listing meetings.
type ListOptions struct {
	tmeet                *internal.Tmeet
	Pos                  string // Query start position, default 0. ISO 8601 start time for paginated meeting list, e.g. 2026-03-12T15:00+08:00
	Cursory              string // Query end position. ISO 8601 end time for paginated meeting list, e.g. 2026-03-12T15:00+08:00
	IsShowAllSubMeetings int    // Whether to show all sub-meetings. 0: no, 1: yes, default 0
	PageToken            string // Page token for pagination
	PageSize             int    // Page size for pagination, max 20
}

// newListCmd lists meetings.
func newListCmd(tmeet *internal.Tmeet) *cobra.Command {
	opts := &ListOptions{tmeet: tmeet}
	cmd := &cobra.Command{
		Use:   "list",
		Short: "list pending/in-progress meetings",
		RunE: middleWare.Chain(
			opts.Run,
			middleWare.WithApiCmd(cmdutil.StaticApiCmd(cmdutil.ApiCmdMeetingList)),
			middleWare.WithCompact(tmeet),
		),
	}

	// 填充参数
	cmd.Flags().StringVar(&opts.Pos, "start", "", "query start position (ISO 8601, e.g. 2026-03-12T15:00+08:00)")
	cmd.Flags().StringVar(&opts.Cursory, "end", "", "query end position (ISO 8601, e.g. 2026-03-12T15:00+08:00)")
	cmd.Flags().IntVar(&opts.IsShowAllSubMeetings, "show-all-sub", 0, "show all sub-meetings: 0-no, 1-yes")
	cmd.Flags().StringVar(&opts.PageToken, "page-token", "", "page token for pagination")
	cmd.Flags().IntVar(&opts.PageSize, "page-size", 20, "page size for pagination, max 20, default 20")

	return cmd
}

func (o *ListOptions) Run(cmd *cobra.Command, args []string) error {
	queryParams := thttp.QueryParams{}
	queryParams.Set("userid", o.tmeet.UserConfig.OpenId)
	queryParams.Set("instanceid", "1") // PC, fixed value

	// page handler
	// special page-size max，not 30
	pageSize, err := cmdutil.ClampingPageSize(cmd, o.PageSize, 20)
	if err != nil {
		return err
	}
	queryParams.Set("page_size", strconv.Itoa(pageSize))
	queryParams.Set("page_token", o.PageToken)
	queryParams.Set("page_type", strconv.Itoa(cmdutil.PageTypeToken))

	// Filter meeting_info_list by the requested start/end time range.
	var filterStart, filterEnd int64
	if o.Pos != "" {
		posTs, err := utils.ISO8601ToTimeStamp(o.Pos)
		if err != nil {
			return exception.InvalidArgsError.With("--start format error: %v", err)
		}
		filterStart = posTs
		if o.PageToken == "" {
			queryParams.Set("pos", strconv.FormatInt(posTs, 10))
		}
	}
	if o.Cursory != "" {
		cursoryTs, err := utils.ISO8601ToTimeStamp(o.Cursory)
		if err != nil {
			return exception.InvalidArgsError.With("--end format error: %v", err)
		}
		// Millisecond-level timestamp, needs conversion.
		filterEnd = cursoryTs
	}
	if o.IsShowAllSubMeetings != 0 {
		queryParams.Set("is_show_all_sub_meetings", strconv.Itoa(o.IsShowAllSubMeetings))
	}

	req := &thttp.Request{
		ApiURI:      "/v1/meetings",
		QueryParams: queryParams,
	}
	rsp, err := restProxy.RequestProxy(cmd.Context(), http.MethodGet, o.tmeet, req)
	if err != nil {
		return err
	}

	if filterStart > 0 || filterEnd > 0 {
		rsp.Data = string(utils.FilterMeetingsByTimeRange(
			[]byte(rsp.Data), "meeting_info_list", "start_time", "end_time", filterStart, filterEnd))
	}

	convertMap := map[string]utils.FieldConverter{
		"start_time":               utils.TimestampConverter,
		"end_time":                 utils.TimestampConverter,
		"meeting_info_list.status": utils.MeetingStatusConverter,
		"meeting_type":             utils.MeetingTypeConverter,
		"recurring_type":           utils.MeetingRecurringTypeConverter,
		"until_date":               utils.TimestampConverter,
		"until_type":               utils.MeetingRecurringUntilTypeConverter,
		"join_meeting_role":        utils.MeetingUserJoinRoleConverter,
		"only_user_join_type":      utils.MeetingJoinTypeConverter,
		"is_show_all_sub_meetings": utils.ShowAllSubMeetingsConverter,
	}
	output.FormatPrint(cmd, rsp.TraceId, rsp.Message, rsp.Data,
		output.WithCompact(middleWare.GetCompactFields(cmd.Context())),
		output.WithConvert(convertMap))
	return nil
}
