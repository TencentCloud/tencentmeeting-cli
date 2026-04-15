package meeting

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

// ListEndedOptions holds the options for listing ended meetings.
type ListEndedOptions struct {
	tmeet     *internal.Tmeet
	StartTime string // Query start time, ISO 8601, e.g. 2026-03-12T15:00+08:00
	EndTime   string // Query end time, ISO 8601, e.g. 2026-03-12T15:00+08:00
	Page      int    // Page number, starting from 1
	PageSize  int    // Page size
}

// newListEndedCmd lists ended meetings.
func newListEndedCmd(tmeet *internal.Tmeet) *cobra.Command {
	opts := &ListEndedOptions{tmeet: tmeet}
	cmd := &cobra.Command{
		Use:   "list-ended",
		Short: "list ended meetings",
		RunE: func(cmd *cobra.Command, args []string) error {
			return opts.Run(cmd, args)
		},
	}

	cmd.Flags().StringVar(&opts.StartTime, "start", "", "query start time (ISO 8601, e.g. 2026-03-12T15:00+08:00)")
	cmd.Flags().StringVar(&opts.EndTime, "end", "", "query end time (ISO 8601, e.g. 2026-03-12T15:00+08:00)")
	cmd.Flags().IntVar(&opts.Page, "page", 1, "page number, starting from 1")
	cmd.Flags().IntVar(&opts.PageSize, "page-size", 10, "page size, default 10, max 20")

	return cmd
}

func (o *ListEndedOptions) Run(cmd *cobra.Command, args []string) error {
	queryParams := thttp.QueryParams{}
	queryParams.Set("page", strconv.Itoa(o.Page))
	queryParams.Set("page_size", strconv.Itoa(o.PageSize))

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

	// 解析响应，递归转换时间戳字段为 ISO8601 格式
	rsp.Data = string(utils.ConvertFields([]byte(rsp.Data), 10, map[string]utils.FieldConverter{
		"start_time":   utils.TimestampConverter,
		"end_time":     utils.TimestampConverter,
		"meeting_type": utils.MeetingTypeConverter,
	}))
	log.FormatPrint(cmd, rsp.TraceId, rsp.Message, rsp.Data)
	return nil
}
