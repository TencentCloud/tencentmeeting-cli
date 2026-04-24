package meeting

import (
	"net/http"
	"strconv"
	"tmeet/internal"
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
}

// newListCmd lists meetings.
func newListCmd(tmeet *internal.Tmeet) *cobra.Command {
	opts := &ListOptions{tmeet: tmeet}
	cmd := &cobra.Command{
		Use:   "list",
		Short: "list meetings",
		RunE: func(cmd *cobra.Command, args []string) error {
			return opts.Run(cmd, args)
		},
	}

	// 填充参数
	cmd.Flags().StringVar(&opts.Pos, "start", "", "query start position (ISO 8601, e.g. 2026-03-12T15:00+08:00)")
	cmd.Flags().StringVar(&opts.Cursory, "end", "", "query end position (ISO 8601, e.g. 2026-03-12T15:00+08:00)")
	cmd.Flags().IntVar(&opts.IsShowAllSubMeetings, "show-all-sub", 0, "show all sub-meetings: 0-no, 1-yes")

	return cmd
}

func (o *ListOptions) Run(cmd *cobra.Command, args []string) error {
	queryParams := thttp.QueryParams{}
	queryParams.Set("userid", o.tmeet.UserConfig.OpenId)
	queryParams.Set("instanceid", "1") // PC, fixed value

	// Filter meeting_info_list by the requested start/end time range.
	var filterStart, filterEnd int64
	if o.Pos != "" {
		posTs, err := utils.ISO8601ToTimeStamp(o.Pos)
		if err != nil {
			return exception.InvalidArgsError.With("--start format error: %v", err)
		}
		filterStart = posTs
		queryParams.Set("pos", strconv.FormatInt(posTs, 10))
	}
	if o.Cursory != "" {
		cursoryTs, err := utils.ISO8601ToTimeStamp(o.Cursory)
		if err != nil {
			return exception.InvalidArgsError.With("--end format error: %v", err)
		}
		// Millisecond-level timestamp, needs conversion.
		filterEnd = cursoryTs
		cursoryMillis := cursoryTs * 1000
		queryParams.Set("cursory", strconv.FormatInt(cursoryMillis, 10))
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

	// Parse response, recursively convert timestamp fields to ISO8601 format.
	rsp.Data = string(utils.ConvertFields([]byte(rsp.Data), 10, map[string]utils.FieldConverter{
		"start_time":               utils.TimestampConverter,
		"end_time":                 utils.TimestampConverter,
		"meeting_info_list.status": utils.MeetingStatusConverter,
		"meeting_type":             utils.MeetingTypeConverter,
		"recurring_type":           utils.MeetingRecurringTypeConverter,
		"time_zone":                utils.Base64DecodeConverter,
		"until_date":               utils.TimestampConverter,
		"until_type":               utils.MeetingRecurringUntilTypeConverter,
	}))
	output.FormatPrint(cmd, rsp.TraceId, rsp.Message, rsp.Data)
	return nil
}
