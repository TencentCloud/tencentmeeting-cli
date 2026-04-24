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

// UpdateOptions holds the options for updating a meeting.
type UpdateOptions struct {
	tmeet             *internal.Tmeet
	MeetingID         string // Meeting ID
	Subject           string // Meeting subject
	StartTime         string // Meeting start time, ISO 8601, e.g. 2026-03-12T15:00+08:00
	EndTime           string // Meeting end time, ISO 8601, e.g. 2026-03-12T15:00+08:00
	Password          string // Meeting password (4-6 digits)
	Timezone          string // Timezone, e.g. Asia/Shanghai
	MeetingType       int    // Meeting type. 0: normal meeting, 1: recurring meeting
	OnlyUserJoinType  int    // Member join restriction. 1: all members, 2: invited only, 3: internal only
	AutoInWaitingRoom bool   // Whether to enable waiting room
	RecurringType     int    // Recurring meeting config (required when meetingType=1). Recurrence type, default 0. 0: daily, 1: weekdays, 2: weekly, 3: biweekly, 4: monthly
	UntilType         int    // Recurring meeting config (required when meetingType=1). End type, default 0. 0: end by date, 1: end by count
	UntilCount        int    // Recurring meeting config (required when meetingType=1). Max occurrences. Daily/weekday/weekly max 500; biweekly/monthly max 50. Default 7.
	UntilDate         string // Recurring meeting config (required when meetingType=1). End date
}

// newUpdateCmd updates a meeting.
func newUpdateCmd(tmeet *internal.Tmeet) *cobra.Command {
	opts := &UpdateOptions{tmeet: tmeet}
	cmd := &cobra.Command{
		Use:   "update",
		Short: "update meeting",
		RunE: func(cmd *cobra.Command, args []string) error {
			return opts.Run(cmd, args)
		},
	}

	// 填充参数
	cmd.Flags().StringVar(&opts.MeetingID, "meeting-id", "", "meeting ID (required)")
	cmd.Flags().StringVar(&opts.Subject, "subject", "", "meeting subject")
	cmd.Flags().StringVar(&opts.StartTime, "start", "", "meeting start time (ISO 8601, e.g. 2026-03-12T14:00+08:00)")
	cmd.Flags().StringVar(&opts.EndTime, "end", "", "meeting end time (ISO 8601, e.g. 2026-03-12T14:00+08:00)")
	cmd.Flags().StringVar(&opts.Password, "password", "", "meeting password (4-6 digits)")
	cmd.Flags().StringVar(&opts.Timezone, "timezone", "", "timezone (e.g. Asia/Shanghai)")
	cmd.Flags().IntVar(&opts.MeetingType, "meeting-type", 0, "meeting type: 0-normal, 1-recurring")
	cmd.Flags().IntVar(&opts.OnlyUserJoinType, "join-type", 0, "join restriction: 1-all members, 2-invited only, 3-internal only")
	cmd.Flags().BoolVar(&opts.AutoInWaitingRoom, "waiting-room", false, "enable waiting room")
	cmd.Flags().IntVar(&opts.RecurringType, "recurring-type", 0, "recurring type (0-daily, 1-weekday, 2-weekly, 3-biweekly, 4-monthly)")
	cmd.Flags().IntVar(&opts.UntilType, "until-type", 0, "until type (0-date, 1-count)")
	cmd.Flags().IntVar(&opts.UntilCount, "until-count", 7, "until count")
	cmd.Flags().StringVar(&opts.UntilDate, "until-date", "", "until date e.g. 2026-03-12T15:00+08:00)")

	// Set required parameters.
	_ = cmd.MarkFlagRequired("meeting-id")

	return cmd
}

func (o *UpdateOptions) Run(cmd *cobra.Command, args []string) error {
	params := map[string]interface{}{
		"userid":     o.tmeet.UserConfig.OpenId,
		"instanceid": 1, // PC, fixed value
	}

	// Optional fields: only set when user provides them.
	if cmd.Flags().Changed("meeting-type") {
		params["meeting_type"] = o.MeetingType
	}
	if o.Subject != "" {
		params["subject"] = o.Subject
	}
	if o.Password != "" {
		params["password"] = o.Password
	}
	if o.Timezone != "" {
		params["timezone"] = o.Timezone
	}
	if o.StartTime != "" {
		startTime, err := utils.ISO8601ToTimeStamp(o.StartTime)
		if err != nil {
			return exception.InvalidArgsError.With("--start format error: %v", err)
		}
		params["start_time"] = strconv.FormatInt(startTime, 10)
	}
	if o.EndTime != "" {
		endTime, err := utils.ISO8601ToTimeStamp(o.EndTime)
		if err != nil {
			return exception.InvalidArgsError.With("--end format error: %v", err)
		}
		params["end_time"] = strconv.FormatInt(endTime, 10)
	}
	if o.MeetingType == 1 {
		// Recurring meeting, add recurring parameters.
		recurringRule := make(map[string]interface{}, 3)
		recurringRule["until_type"] = o.UntilType
		recurringRule["until_count"] = o.UntilCount
		recurringRule["recurring_type"] = o.RecurringType
		if o.UntilDate != "" {
			untilDate, err := utils.ISO8601ToTimeStamp(o.UntilDate)
			if err != nil {
				return exception.InvalidArgsError.With("--until-date format error: %v", err)
			}
			recurringRule["until_date"] = untilDate
		}
		params["recurring_rule"] = recurringRule
	}
	// setting parameters
	settings := make(map[string]interface{}, 0)
	if o.OnlyUserJoinType > 0 {
		settings["only_user_join_type"] = o.OnlyUserJoinType
	}
	if o.AutoInWaitingRoom {
		settings["auto_in_waiting_room"] = o.AutoInWaitingRoom
	}
	if len(settings) > 0 {
		params["settings"] = settings
	}

	req := &thttp.Request{
		ApiURI: "/v1/meetings/" + o.MeetingID,
		Body:   params,
	}
	rsp, err := restProxy.RequestProxy(cmd.Context(), http.MethodPut, o.tmeet, req)
	if err != nil {
		return err
	}

	// 解析响应，递归转换时间戳字段为 ISO8601 格式
	rsp.Data = string(utils.ConvertFields([]byte(rsp.Data), 10, map[string]utils.FieldConverter{
		"start_time": utils.TimestampConverter,
		"end_time":   utils.TimestampConverter,
	}))
	output.FormatPrint(cmd, rsp.TraceId, rsp.Message, rsp.Data)
	return nil
}
