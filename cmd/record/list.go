package record

import (
	"net/http"
	"strconv"
	"tmeet/internal"
	"tmeet/internal/core/thttp"
	"tmeet/internal/exception"
	"tmeet/internal/log"
	restProxy "tmeet/internal/proxy/rest-proxy"
	"tmeet/internal/utils"

	"github.com/spf13/cobra"
)

// ListOptions holds the options for listing records.
type ListOptions struct {
	tmeet       *internal.Tmeet
	StartTime   string // Query start time, ISO 8601, e.g. 2026-03-12T14:00+08:00 (required)
	EndTime     string // Query end time, ISO 8601, e.g. 2026-03-12T14:00+08:00 (required)
	Page        int    // Page number, starting from 1
	PageSize    int    // Page size
	MeetingID   string // Meeting ID
	MeetingCode string // Meeting code
}

// newListCmd lists records.
func newListCmd(tmeet *internal.Tmeet) *cobra.Command {
	opts := &ListOptions{tmeet: tmeet}
	cmd := &cobra.Command{
		Use:   "list",
		Short: "list records",
		RunE: func(cmd *cobra.Command, args []string) error {
			return opts.Run(cmd, args)
		},
	}

	cmd.Flags().StringVar(&opts.StartTime, "start", "", "query start time (ISO 8601, e.g. 2026-03-12T14:00+08:00)")
	cmd.Flags().StringVar(&opts.EndTime, "end", "", "query end time (ISO 8601, e.g. 2026-03-12T14:00+08:00)")
	cmd.Flags().IntVar(&opts.Page, "page", 1, "page number, starting from 1")
	cmd.Flags().IntVar(&opts.PageSize, "page-size", 10, "page size")
	cmd.Flags().StringVar(&opts.MeetingID, "meeting-id", "", "meeting id, one of the following groups is required(--start + --end or --meeting-id or --meeting-code)")
	cmd.Flags().StringVar(&opts.MeetingCode, "meeting-code", "", "meeting code, one of the following groups is required(--start + --end or --meeting-id or --meeting-code)")

	return cmd
}

func (o *ListOptions) Run(cmd *cobra.Command, args []string) error {
	// Three groups of parameters, one is required:
	//   1. --start + --end   (time range)
	//   2. --meeting-id      (meeting id)
	//   3. --meeting-code    (meeting code)
	hasTimeRange := o.StartTime != "" && o.EndTime != ""
	hasMeetingID := o.MeetingID != ""
	hasMeetingCode := o.MeetingCode != ""

	if !hasTimeRange && !hasMeetingID && !hasMeetingCode {
		return exception.InvalidArgsError.With("one of the following groups is required:\n" +
			"  --start + --end   (time range)\n" +
			"  --meeting-id      (meeting id)\n" +
			"  --meeting-code    (meeting code)")
	}

	queryParams := thttp.QueryParams{}
	queryParams.Set("operator_id", o.tmeet.UserConfig.OpenId)
	queryParams.Set("operator_id_type", "2") // OpenId
	queryParams.Set("page_size", strconv.FormatInt(int64(o.PageSize), 10))
	queryParams.Set("page", strconv.FormatInt(int64(o.Page), 10))

	if o.MeetingID != "" {
		queryParams.Set("meeting_id", o.MeetingID)
	}
	if o.MeetingCode != "" {
		queryParams.Set("meeting_code", o.MeetingCode)
	}
	if o.StartTime != "" {
		startTime, err := utils.ISO8601ToTimeStamp(o.StartTime)
		if err != nil {
			return exception.InvalidArgsError.With("--start format error: %v", err)
		}
		queryParams.Set("start_time", strconv.FormatInt(startTime, 10))
	}
	if o.EndTime != "" {
		endTime, err := utils.ISO8601ToTimeStamp(o.EndTime)
		if err != nil {
			return exception.InvalidArgsError.With("--end format error: %v", err)
		}
		queryParams.Set("end_time", strconv.FormatInt(endTime, 10))
	}

	req := &thttp.Request{
		ApiURI:      "/v1/mcp/records",
		QueryParams: queryParams,
	}
	rsp, err := restProxy.RequestProxy(cmd.Context(), http.MethodGet, o.tmeet, req)
	if err != nil {
		return err
	}

	// Parse response, recursively convert timestamp fields to ISO8601 format.
	rsp.Data = string(utils.ConvertFields([]byte(rsp.Data), 10, map[string]utils.FieldConverter{
		"media_start_time":  utils.TimestampConverter,
		"record_start_time": utils.TimestampConverter,
		"record_end_time":   utils.TimestampConverter,
	}))
	log.Infof(cmd, restProxy.Print(cmd, rsp))
	return nil
}
