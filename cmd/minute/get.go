package minute

import (
	"encoding/json"
	"net/http"
	"strconv"
	"tmeet/internal"
	"tmeet/internal/cmdutil"
	middleWare "tmeet/internal/cmdutil/middleware"
	"tmeet/internal/core/thttp"
	"tmeet/internal/exception"
	"tmeet/internal/output"
	restProxy "tmeet/internal/proxy/rest-proxy"

	"github.com/spf13/cobra"
)

// GetOptions holds the options for getting minutes.
type GetOptions struct {
	tmeet         *internal.Tmeet
	MinuteID      string // minute unique identifier
	MeetingID     string // meeting ID (cycle-level), requires sub_meeting_id for instance
	MeetingCode   string // meeting code (9-12 digits), will be resolved to meeting ID
	SubMeetingID  string // sub-meeting ID (recurring meeting instance); omit for non-recurring
	Overview      bool   // whether to include meeting overview, default true
	SummaryPoints bool   // whether to include summary points, default true
	Todos         bool   // whether to include todos, default true
	ShortSummary  bool   // whether to fetch transient (rolling) minutes
	PageToken     string // page token for pagination
	PageSize      int    // page size
}

// newGetCmd queries Yuanbao minutes detail.
func newGetCmd(tmeet *internal.Tmeet) *cobra.Command {
	opts := &GetOptions{tmeet: tmeet}
	cmd := &cobra.Command{
		Use:   "get",
		Short: "get Yuanbao minutes by minute-id or meeting-id",
		RunE: middleWare.Chain(
			opts.Run,
			middleWare.WithApiCmd(cmdutil.FlagSwitchWithDefault(cmdutil.ApiCmdMinuteGet,
				cmdutil.FlagCase{
					When:   func(cmd *cobra.Command) bool { v, _ := cmd.Flags().GetBool("short-summary"); return v },
					ApiCmd: cmdutil.ApiCmdMinuteGetTransient,
				},
			)),
		),
	}

	cmd.Flags().StringVar(&opts.MinuteID, "minute-id", "",
		"minute unique identifier (required for transient minutes)")
	cmd.Flags().StringVar(&opts.MeetingID, "meeting-id", "",
		"meeting ID (cycle-level), used for stable minutes")
	cmd.Flags().StringVar(&opts.MeetingCode, "meeting-code", "",
		"meeting code (9-12 digits), will be resolved to meeting ID for stable minutes")
	cmd.Flags().StringVar(&opts.SubMeetingID, "sub-meeting-id", "",
		"sub-meeting ID (recurring meeting instance); omit for non-recurring meetings")
	cmd.Flags().BoolVar(&opts.Overview, "overview", true, "include meeting overview, default true")
	cmd.Flags().BoolVar(&opts.SummaryPoints, "summary-points", true, "include summary points, default true")
	cmd.Flags().BoolVar(&opts.Todos, "todos", true, "include todos, default true")
	cmd.Flags().BoolVar(&opts.ShortSummary, "short-summary", false,
		"fetch transient (rolling) minutes; requires --minute-id")
	cmd.Flags().StringVar(&opts.PageToken, "page-token", "", "page token for pagination")
	cmd.Flags().IntVar(&opts.PageSize, "page-size", 0, "page size (stable: default 10, max 30; transient: default 100, max 300)")

	return cmd
}

const (
	// pageSizeDefaultStable is the default page size for stable minute get.
	pageSizeDefaultStable = 10
	// pageSizeMaxStable is the max page size for stable minute get.
	pageSizeMaxStable = 30
	// pageSizeDefaultTransient is the default page size for transient minute get.
	pageSizeDefaultTransient = 100
	// pageSizeMaxTransient is the max page size for transient minute get.
	pageSizeMaxTransient = 300
)

// Run executes the get command.
func (o *GetOptions) Run(cmd *cobra.Command, args []string) error {
	if o.ShortSummary {
		return o.runTransient(cmd)
	}
	return o.runStable(cmd)
}

// runStable fetches stable (steady-state) minutes via meeting-id.
func (o *GetOptions) runStable(cmd *cobra.Command) error {
	// meeting-id, meeting-code or minute-id is required for stable minutes
	hasMinuteID := o.MinuteID != ""
	hasMeetingID := o.MeetingID != ""
	hasMeetingCode := o.MeetingCode != ""

	if !hasMinuteID && !hasMeetingID && !hasMeetingCode {
		return exception.InvalidArgsError.With("one of the following is required:\n" +
			"  --minute-id      (minute unique identifier)\n" +
			"  --meeting-id     (meeting ID)\n" +
			"  --meeting-code   (meeting code)")
	}

	// 如果传入的是 meeting-code，先通过 meeting code 查询会议详情获取 meeting_id
	if hasMeetingCode && !hasMeetingID {
		meetingID, err := o.resolveMeetingIDByCode(cmd)
		if err != nil {
			return err
		}
		o.MeetingID = meetingID
	}

	queryParams := thttp.QueryParams{}
	queryParams.Set("operator_id", o.tmeet.UserConfig.OpenId)
	queryParams.Set("operator_id_type", "2") // OpenId

	// identifier parameters
	if o.MinuteID != "" {
		queryParams.Set("minute_id", o.MinuteID)
	}
	if o.MeetingID != "" {
		queryParams.Set("meeting_id", o.MeetingID)
	}
	if o.SubMeetingID != "" {
		queryParams.Set("sub_meeting_id", o.SubMeetingID)
	}

	// content selection flags — short_summary is always false for stable minutes
	queryParams.Set("overview", strconv.FormatBool(o.Overview))
	queryParams.Set("summary_points", strconv.FormatBool(o.SummaryPoints))
	queryParams.Set("todos", strconv.FormatBool(o.Todos))
	queryParams.Set("short_summary", "false")

	// pagination
	pageSize := o.PageSize
	if pageSize == 0 {
		pageSize = pageSizeDefaultStable
	}
	pageSize, err := cmdutil.ClampingPageSize(cmd, pageSize, pageSizeMaxStable)
	if err != nil {
		return err
	}
	queryParams.Set("page_token", o.PageToken)
	queryParams.Set("page_size", strconv.Itoa(pageSize))

	req := &thttp.Request{
		ApiURI:      "/v1/mcp/asr/get-minutes-mcp",
		QueryParams: queryParams,
	}
	rsp, err := restProxy.RequestProxy(cmd.Context(), http.MethodGet, o.tmeet, req)
	if err != nil {
		return err
	}

	output.FormatPrint(cmd, rsp.TraceId, rsp.Message, rsp.Data,
		output.WithTotalCountLogic())
	return nil
}

// transientMinutesRsp is the response structure for transient minutes pagination.
type transientMinutesRsp struct {
	MinuteID      string                   `json:"minute_id,omitempty"`
	Items         []map[string]interface{} `json:"items,omitempty"`
	TotalCount    int                      `json:"total_count,omitempty"`
	HasMore       bool                     `json:"has_more,omitempty"`
	NextPageToken string                   `json:"next_page_token,omitempty"`
}

// runTransient fetches transient (rolling) minutes via minute-id.
// 默认获取全量数据，循环分页拉取直到 has_more 为 false。
func (o *GetOptions) runTransient(cmd *cobra.Command) error {
	// minute-id is required for transient minutes
	if o.MinuteID == "" {
		return exception.InvalidArgsError.With("--minute-id is required when --short-summary is set")
	}

	pageSize := o.PageSize
	if pageSize == 0 {
		pageSize = pageSizeDefaultTransient
	}
	pageSize, err := cmdutil.ClampingPageSize(cmd, pageSize, pageSizeMaxTransient)
	if err != nil {
		return err
	}

	return o.fetchAllTransient(cmd, pageSize)
}

// fetchAllTransient fetches all pages of transient minutes and merges them.
func (o *GetOptions) fetchAllTransient(cmd *cobra.Command, pageSize int) error {
	const maxPages = 100
	var allItems []map[string]interface{}
	var minuteID string
	var lastTraceId string
	pageToken := o.PageToken

	for i := 0; i < maxPages; i++ {
		rsp, err := o.fetchTransientPage(cmd, pageSize, pageToken)
		if err != nil {
			return err
		}
		lastTraceId = rsp.TraceId

		// Parse response to check has_more and collect items
		var parsed transientMinutesRsp
		if err := json.Unmarshal([]byte(rsp.Data), &parsed); err != nil {
			// If parsing fails, just output the raw response
			output.FormatPrint(cmd, rsp.TraceId, rsp.Message, rsp.Data,
				output.WithTotalCountLogic())
			return nil
		}

		if minuteID == "" {
			minuteID = parsed.MinuteID
		}
		allItems = append(allItems, parsed.Items...)

		if !parsed.HasMore || parsed.NextPageToken == "" {
			break
		}
		pageToken = parsed.NextPageToken
	}

	// Build merged response
	merged := map[string]interface{}{
		"minute_id":   minuteID,
		"items":       allItems,
		"total_count": len(allItems),
		"has_more":    false,
	}
	mergedData, _ := json.Marshal(merged)

	output.FormatPrint(cmd, lastTraceId, restProxy.Success, string(mergedData),
		output.WithTotalCountLogic())
	return nil
}

// fetchTransientPage fetches a single page of transient minutes.
func (o *GetOptions) fetchTransientPage(cmd *cobra.Command, pageSize int, pageToken string) (*restProxy.ProxyRsp, error) {
	queryParams := thttp.QueryParams{}
	queryParams.Set("operator_id", o.tmeet.UserConfig.OpenId)
	queryParams.Set("operator_id_type", "2") // OpenId
	queryParams.Set("minute_id", o.MinuteID)
	queryParams.Set("pagination_type", "2") // pageToken mode
	queryParams.Set("page_size", strconv.Itoa(pageSize))
	if pageToken != "" {
		queryParams.Set("page_token", pageToken)
	}

	req := &thttp.Request{
		ApiURI:      "/v1/mcp/asr/get-transient-minutes-mcp",
		QueryParams: queryParams,
	}
	return restProxy.RequestProxy(cmd.Context(), http.MethodGet, o.tmeet, req)
}

// meetingInfoListRsp is the response structure for meeting query by code.
type meetingInfoListRsp struct {
	MeetingInfoList []struct {
		MeetingID string `json:"meeting_id"`
	} `json:"meeting_info_list"`
}

// resolveMeetingIDByCode queries meeting details by meeting code and extracts the meeting_id.
func (o *GetOptions) resolveMeetingIDByCode(cmd *cobra.Command) (string, error) {
	queryParams := thttp.QueryParams{}
	queryParams.Set("userid", o.tmeet.UserConfig.OpenId)
	queryParams.Set("instanceid", "1") // PC, fixed value
	queryParams.Set("meeting_code", cmdutil.FormatMeetingCode(o.MeetingCode))

	req := &thttp.Request{
		ApiURI:      "/v1/meetings",
		QueryParams: queryParams,
	}
	rsp, err := restProxy.RequestProxy(cmd.Context(), http.MethodGet, o.tmeet, req)
	if err != nil {
		return "", err
	}

	var parsed meetingInfoListRsp
	if err := json.Unmarshal([]byte(rsp.Data), &parsed); err != nil {
		return "", exception.InvalidArgsError.With("failed to parse meeting info from meeting-code response")
	}
	if len(parsed.MeetingInfoList) == 0 {
		return "", exception.InvalidArgsError.With("no meeting found for meeting-code: " + o.MeetingCode)
	}
	return parsed.MeetingInfoList[0].MeetingID, nil
}
