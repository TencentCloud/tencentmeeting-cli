package report

import (
	"net/http"
	"strconv"
	"tmeet/internal"
	"tmeet/internal/core/thttp"
	"tmeet/internal/output"
	restProxy "tmeet/internal/proxy/rest-proxy"
	"tmeet/internal/utils"

	"github.com/spf13/cobra"
)

// WaitingRoomOptions holds the options for waiting room.
type WaitingRoomOptions struct {
	tmeet     *internal.Tmeet
	MeetingId string // Meeting ID
	PageSize  int    // Page size, default 20
	Page      int    // Page number, default 1
}

// newWaitingRoomCmd gets the waiting room members list.
func newWaitingRoomCmd(tmeet *internal.Tmeet) *cobra.Command {
	opts := &WaitingRoomOptions{tmeet: tmeet}
	cmd := &cobra.Command{
		Use:   "waiting-room-log",
		Short: "get waiting room members",
		RunE: func(cmd *cobra.Command, args []string) error {
			return opts.Run(cmd, args)
		},
	}

	cmd.Flags().StringVar(&opts.MeetingId, "meeting-id", "", "meeting id (required)")
	cmd.Flags().IntVar(&opts.PageSize, "page-size", 20, "page size, default 20, max 50")
	cmd.Flags().IntVar(&opts.Page, "page", 1, "page number, default 1")

	_ = cmd.MarkFlagRequired("meeting-id")

	return cmd
}

func (o *WaitingRoomOptions) Run(cmd *cobra.Command, args []string) error {
	queryParams := thttp.QueryParams{}
	queryParams.Set("operator_id", o.tmeet.UserConfig.OpenId)
	queryParams.Set("operator_id_type", "2") // OpenId
	queryParams.Set("page", strconv.Itoa(o.Page))
	queryParams.Set("page_size", strconv.Itoa(o.PageSize))

	req := &thttp.Request{
		ApiURI:      "/v1/meeting/{meeting_id}/waiting-room",
		PathParams:  thttp.PathParams{"meeting_id": o.MeetingId},
		QueryParams: queryParams,
	}
	rsp, err := restProxy.RequestProxy(cmd.Context(), http.MethodGet, o.tmeet, req)
	if err != nil {
		return err
	}

	// 解析响应，递归转换时间戳字段为 ISO8601 格式
	rsp.Data = string(utils.ConvertFields([]byte(rsp.Data), 10, map[string]utils.FieldConverter{
		"join_time":           utils.TimestampConverter,
		"left_time":           utils.TimestampConverter,
		"schedule_start_time": utils.TimestampConverter,
		"schedule_end_time":   utils.TimestampConverter,
		"instanceid":          utils.InstanceIdConverter,
		"user_name":           utils.Base64DecodeConverter,
		"subject":             utils.Base64DecodeConverter,
	}))
	output.FormatPrint(cmd, rsp.TraceId, rsp.Message, rsp.Data)
	return nil
}
