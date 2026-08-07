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

// Character length limits for search fields.
const (
	maxQueryLen = 50
)

// SearchOptions is the options for search command.
type SearchOptions struct {
	tmeet       *internal.Tmeet
	Query       string // search keyword
	QueryField  string // search field for Query
	MeetingCode string // meeting code
	From        string // start time, ISO 8601, e.g. 2026-03-12T15:00+08:00
	To          string // end time, ISO 8601, e.g. 2026-03-12T15:00+08:00
	PageSize    int    // Page size
	PageToken   string // Page token for pagination
}

// newSearchCmd searches meetings.
func newSearchCmd(tmeet *internal.Tmeet) *cobra.Command {
	opts := &SearchOptions{tmeet: tmeet}
	cmd := &cobra.Command{
		Use:   "search",
		Short: "search meetings by keyword, meeting code, time range or status",
		RunE: middleWare.Chain(
			opts.Run,
			middleWare.WithApiCmd(cmdutil.StaticApiCmd(cmdutil.ApiCmdMeetingSearch)),
			middleWare.WithCompact(tmeet),
		),
	}

	cmd.Flags().StringVar(&opts.Query, "query", "", "search keyword")
	cmd.Flags().StringVar(&opts.QueryField, "query-field", "all", `search field for --query (e.g. 
		subject: meeting subject; 
		creator: creator's nickname/remark name,
		note: user's note on the meeting
		all: search all fields)`)
	cmd.Flags().StringVar(&opts.MeetingCode, "meeting-code", "", "filter by meeting code, exact match (digits only, no dashes)")
	cmd.Flags().StringVar(&opts.From, "start", "", "lower bound of search time window (ISO 8601). Matches if meeting's scheduled start time OR actual start time OR user's join time falls within the window (ISO 8601, e.g. 2026-03-12T15:00+08:00)")
	cmd.Flags().StringVar(&opts.To, "end", "", "upper bound of search time window (ISO 8601). Same semantics as above (ISO 8601, e.g. 2026-03-12T15:00+08:00)")
	cmd.Flags().IntVar(&opts.PageSize, "page-size", 30, "page size, default 30, max 30")
	cmd.Flags().StringVar(&opts.PageToken, "page-token", "", "page token for pagination")

	return cmd
}

// Run executes the search command.
func (o *SearchOptions) Run(cmd *cobra.Command, args []string) error {
	queryParams := thttp.QueryParams{}
	queryParams.Set("operator_id", o.tmeet.UserConfig.OpenId)
	queryParams.Set("operator_id_type", "2") // OpenId

	// page handler
	pageSize, err := cmdutil.ClampingPageSize(cmd, o.PageSize, cmdutil.PageSizeMaxMeetings)
	if err != nil {
		return err
	}
	queryParams.Set("page_token", o.PageToken)
	queryParams.Set("page_size", strconv.Itoa(pageSize))

	// other params
	if o.Query != "" {
		if err := utils.CharacterLimit("--query", o.Query, maxQueryLen); err != nil {
			return err
		}
		queryParams.Set("q", o.Query)
		queryParams.Set("q_fields", o.QueryField)
	}
	if o.MeetingCode != "" {
		queryParams.Set("meeting_code", cmdutil.FormatMeetingCode(o.MeetingCode))
	}

	// time range
	var start, end int64
	if o.From != "" {
		start, err = utils.ISO8601ToTimeStamp(o.From)
		if err != nil {
			return exception.InvalidArgsError.With("--start format error: %v", err)
		}
		queryParams.Set("from", o.From)
	}
	if o.To != "" {
		end, err = utils.ISO8601ToTimeStamp(o.To)
		if err != nil {
			return exception.InvalidArgsError.With("--end format error: %v", err)
		}
		queryParams.Set("to", o.To)
	}
	if start > 0 && end > 0 && start >= end {
		return exception.InvalidArgsError.With("--start must be earlier than --end")
	}

	req := &thttp.Request{
		ApiURI:      "/v1/meetings/search-meetings",
		QueryParams: queryParams,
	}
	rsp, err := restProxy.RequestProxy(cmd.Context(), http.MethodGet, o.tmeet, req)
	if err != nil {
		return err
	}

	// Enrich each meeting with its recording basic info.
	rsp.Data = string(enrichMeetingsWithRecords(cmd.Context(), o.tmeet, []byte(rsp.Data), "meetings", true))

	convertMap := map[string]utils.FieldConverter{
		"meetings.meeting_type":    utils.MeetingTypeConverter,
		"meetings.status":          utils.MeetingStatusInSearchConverter,
		"media_start_time":         utils.TimestampConverter,       // recording start time
		"duration":                 utils.DurationSecondsConverter, // recording duration (seconds -> HH:MM:SS)
		"meetings.records.subject": utils.Base64DecodeConverter,    // recording subject (base64 -> plain text)
	}
	output.FormatPrint(cmd, rsp.TraceId, rsp.Message, rsp.Data,
		output.WithCompact(compactFieldsWithRecords(cmd.Context())),
		output.WithConvert(convertMap),
		output.WithTotalCountLogic())
	return nil
}
