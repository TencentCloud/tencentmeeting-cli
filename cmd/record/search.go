package record

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

// SearchOptions holds the options for searching records.
type SearchOptions struct {
	tmeet       *internal.Tmeet
	Query       string // search keyword
	QueryField  string // search field for Query
	FileType    string // file type: video / audio / transcript / upload / external / all (default all)
	MeetingID   string // meeting id
	MeetingCode string // meeting code
	From        string // start time, ISO 8601, e.g. 2026-03-12T15:00+08:00
	To          string // end time, ISO 8601, e.g. 2026-03-12T15:00+08:00
	PageSize    int    // page size
	PageToken   string // page token for pagination
}

// newSearchCmd searches records.
func newSearchCmd(tmeet *internal.Tmeet) *cobra.Command {
	opts := &SearchOptions{tmeet: tmeet}
	cmd := &cobra.Command{
		Use:   "search",
		Short: "search records by keyword, meeting code, meeting id or time range",
		RunE: middleWare.Chain(
			opts.Run,
			middleWare.WithApiCmd(cmdutil.StaticApiCmd(cmdutil.ApiCmdRecordSearch)),
			middleWare.WithCompact(tmeet),
		),
	}

	cmd.Flags().StringVar(&opts.Query, "query", "", "search keyword")
	cmd.Flags().StringVar(&opts.QueryField, "query-field", "all", `search field for --query (e.g.
		subject: record subject;
		creator: nickname/remark name of the meeting creator;
		transcript_content: original transcript content within the file;
		smart_minutes: smart minutes content within the file (summary + todos);
		timeline: timeline content within the file;
		all: search all fields)`)
	cmd.Flags().StringVar(&opts.From, "start", "", "query start time (ISO 8601, e.g. 2026-03-12T14:00+08:00)")
	cmd.Flags().StringVar(&opts.To, "end", "", "query end time (ISO 8601, e.g. 2026-03-12T14:00+08:00)")
	cmd.Flags().StringVar(&opts.FileType, "file-type", "all", "file type: video / audio / transcript / upload / external / all (default all)")
	cmd.Flags().StringVar(&opts.MeetingID, "meeting-id", "", "filter by meeting id")
	cmd.Flags().StringVar(&opts.MeetingCode, "meeting-code", "", "filter by meeting code, exact match (digits only, no dashes)")
	cmd.Flags().IntVar(&opts.PageSize, "page-size", 30, "page size, default 30, max 30")
	cmd.Flags().StringVar(&opts.PageToken, "page-token", "", "page token for pagination")

	return cmd
}

// Run executes the search command.
func (o *SearchOptions) Run(cmd *cobra.Command, args []string) error {
	queryParams := thttp.QueryParams{}
	queryParams.Set("operator_id", o.tmeet.UserConfig.OpenId)
	queryParams.Set("operator_id_type", "2") // OpenId
	queryParams.Set("file_type", o.FileType)

	// page handler
	pageSize, err := cmdutil.ClampingPageSize(cmd, o.PageSize, cmdutil.PageSizeMaxRecords)
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
	if o.MeetingID != "" {
		queryParams.Set("meeting_id", o.MeetingID)
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
		ApiURI:      "/v1/mcp/records/search",
		QueryParams: queryParams,
	}
	rsp, err := restProxy.RequestProxy(cmd.Context(), http.MethodGet, o.tmeet, req)
	if err != nil {
		return err
	}

	output.FormatPrint(cmd, rsp.TraceId, rsp.Message, rsp.Data,
		output.WithCompact(middleWare.GetCompactFields(cmd.Context())),
		output.WithTotalCountLogic())
	return nil
}
