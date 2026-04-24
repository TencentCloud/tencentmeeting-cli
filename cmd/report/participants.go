package report

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

// ParticipantsOptions holds the options for participants.
type ParticipantsOptions struct {
	tmeet        *internal.Tmeet
	MeetingId    string // Meeting ID
	SubMeetingId string // Sub-meeting ID for recurring meetings
	Pos          int    // Query start position for paginated retrieval, default 0
	Size         int    // Number of participants per page, max 100.
	StartTime    string // Query start time, ISO 8601, e.g. 2026-03-12T14:00+08:00
	EndTime      string // Query end time, ISO 8601, e.g. 2026-03-12T14:00+08:00
}

// newParticipantsCmd gets the participants list.
func newParticipantsCmd(tmeet *internal.Tmeet) *cobra.Command {
	opts := &ParticipantsOptions{tmeet: tmeet}
	cmd := &cobra.Command{
		Use:   "participants",
		Short: "get meeting participants",
		RunE: func(cmd *cobra.Command, args []string) error {
			return opts.Run(cmd, args)
		},
	}

	cmd.Flags().StringVar(&opts.MeetingId, "meeting-id", "", "meeting id (required)")
	cmd.Flags().StringVar(&opts.SubMeetingId, "sub-meeting-id", "", "sub meeting id for recurring meeting")
	cmd.Flags().IntVar(&opts.Pos, "pos", 0, "query start position, default 0")
	cmd.Flags().IntVar(&opts.Size, "size", 20, "number of participants per page, max 100")
	cmd.Flags().StringVar(&opts.StartTime, "start", "", "query start time (ISO 8601, e.g. 2026-03-12T14:00+08:00)")
	cmd.Flags().StringVar(&opts.EndTime, "end", "", "query end time (ISO 8601, e.g. 2026-03-12T14:00+08:00)")

	_ = cmd.MarkFlagRequired("meeting-id")

	return cmd
}

func (o *ParticipantsOptions) Run(cmd *cobra.Command, args []string) error {
	queryParams := thttp.QueryParams{}
	queryParams.Set("operator_id", o.tmeet.UserConfig.OpenId)
	queryParams.Set("operator_id_type", "2") // OpenId
	queryParams.Set("pos", strconv.Itoa(o.Pos))
	queryParams.Set("size", strconv.Itoa(o.Size))

	if o.SubMeetingId != "" {
		queryParams.Set("sub_meeting_id", o.SubMeetingId)
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
		ApiURI:      "/v1/meetings/{meeting_id}/participants",
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
		"user_role":           utils.MeetingUserRoleConverter,
	}))
	output.FormatPrint(cmd, rsp.TraceId, rsp.Message, rsp.Data)
	return nil
}
