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

// ListEndedOptions holds the options for listing ended meetings.
type ListEndedOptions struct {
	tmeet     *internal.Tmeet
	StartTime string // Query start time, ISO 8601, e.g. 2026-03-12T15:00+08:00
	EndTime   string // Query end time, ISO 8601, e.g. 2026-03-12T15:00+08:00
	Page      int    // Page number, starting from 1, deprecated use PageToken instead
	PageSize  int    // Page size
	PageToken string // Page token for pagination
}

// newListEndedCmd lists ended meetings.
func newListEndedCmd(tmeet *internal.Tmeet) *cobra.Command {
	opts := &ListEndedOptions{tmeet: tmeet}
	cmd := &cobra.Command{
		Use:   "list-ended",
		Short: "list ended meetings",
		RunE: middleWare.Chain(
			opts.Run,
			middleWare.WithApiCmd(cmdutil.StaticApiCmd(cmdutil.ApiCmdMeetingListEnded)),
			middleWare.WithCompact(tmeet),
		),
	}

	cmd.Flags().StringVar(&opts.StartTime, "start", "", "query start time (ISO 8601, e.g. 2026-03-12T15:00+08:00)")
	cmd.Flags().StringVar(&opts.EndTime, "end", "", "query end time (ISO 8601, e.g. 2026-03-12T15:00+08:00)")
	cmd.Flags().IntVar(&opts.Page, "page", 0, "page number, starting from 1, (deprecated, use --page-token instead)")
	cmd.Flags().IntVar(&opts.PageSize, "page-size", 30, "page size, default 30, max 30")
	cmd.Flags().StringVar(&opts.PageToken, "page-token", "", "page token for pagination")

	// mark deprecated
	_ = cmd.Flags().MarkDeprecated("page", "use --page-token instead")
	return cmd
}

func (o *ListEndedOptions) Run(cmd *cobra.Command, args []string) error {
	queryParams := thttp.QueryParams{}

	// pagination compatibility
	pageValue, pageType := cmdutil.ChoosePageOrToken(o.Page, o.PageToken)
	queryParams.Set("page_type", strconv.Itoa(pageType))
	if pageType == cmdutil.PageTypeOld {
		queryParams.Set("page", pageValue)
	} else {
		queryParams.Set("page_token", pageValue)
	}
	pageSize, err := cmdutil.ClampingPageSize(cmd, o.PageSize, cmdutil.PageSizeMaxMeetings)
	if err != nil {
		return err
	}
	queryParams.Set("page_size", strconv.Itoa(pageSize))

	if o.StartTime != "" {
		startTs, err := utils.ISO8601ToTimeStamp(o.StartTime)
		if err != nil {
			return exception.InvalidArgsError.With("--start format error: %v", err)
		}
		queryParams.Set("start_time", strconv.FormatInt(startTs, 10))
	}
	if o.EndTime != "" {
		endTs, err := utils.ISO8601ToTimeStamp(o.EndTime)
		if err != nil {
			return exception.InvalidArgsError.With("--end format error: %v", err)
		}
		queryParams.Set("end_time", strconv.FormatInt(endTs, 10))
	}

	req := &thttp.Request{
		ApiURI:      "/v1/history/meetings/{userid}",
		QueryParams: queryParams,
		PathParams:  thttp.PathParams{"userid": o.tmeet.UserConfig.OpenId},
	}
	rsp, err := restProxy.RequestProxy(cmd.Context(), http.MethodGet, o.tmeet, req)
	if err != nil {
		return err
	}

	convertMap := map[string]utils.FieldConverter{
		"start_time":   utils.TimestampConverter,
		"end_time":     utils.TimestampConverter,
		"meeting_type": utils.MeetingTypeConverter,
	}
	output.FormatPrint(cmd, rsp.TraceId, rsp.Message, rsp.Data,
		output.WithCompact(middleWare.GetCompactFields(cmd.Context())),
		output.WithConvert(convertMap))
	return nil
}
