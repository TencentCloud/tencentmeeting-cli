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

// CreateOptions holds the options for creating a meeting.
type CreateOptions struct {
	tmeet             *internal.Tmeet
	Subject           string // Meeting subject (required)
	StartTime         string // Meeting start time, ISO 8601, e.g. 2026-03-12T14:00+08:00 (required)
	EndTime           string // Meeting end time, ISO 8601, e.g. 2026-03-12T15:00+08:00 (required)
	Password          string // Meeting password (4-6 digits), optional
	Timezone          string // Timezone, see Oracle-TimeZone standard, e.g. Asia/Shanghai
	MeetingType       int    // Meeting type, default 0. 0: normal meeting, 1: recurring meeting
	OnlyUserJoinType  int    // Member join restriction. 1: all members, 2: invited only, 3: internal only
	AutoInWaitingRoom bool   // Whether to enable waiting room, default false.
	RecurringType     int    // Recurring meeting config (required when meetingType=1). Recurrence type, default 0. 0: daily, 1: weekdays, 2: weekly, 3: biweekly, 4: monthly
	UntilType         int    // Recurring meeting config (required when meetingType=1). End type, default 0. 0: end by date, 1: end by count
	UntilCount        int    // Recurring meeting config (required when meetingType=1). Max occurrences. Daily/weekday/weekly max 500; biweekly/monthly max 500. Default 7.
	UntilDate         string // Recurring meeting config (required when meetingType=1). End date, ISO 8601, e.g. 2026-03-12T15:00+08:00
}

// newCreateCmd creates a meeting.
func newCreateCmd(tmeet *internal.Tmeet) *cobra.Command {
	opts := &CreateOptions{tmeet: tmeet}
	cmd := &cobra.Command{
		Use:   "create",
		Short: "create meeting",
		RunE: func(cmd *cobra.Command, args []string) error {
			return opts.Run(cmd, args)
		},
	}

	// Fill in parameters.
	cmd.Flags().StringVar(&opts.Subject, "subject", "", "meeting subject (required)")
	cmd.Flags().StringVar(&opts.StartTime, "start", "", "meeting start time (ISO 8601, e.g. 2026-03-12T14:00+08:00) (required)")
	cmd.Flags().StringVar(&opts.EndTime, "end", "", "meeting end time (ISO 8601, e.g. 2026-03-12T15:00+08:00) (required)")
	cmd.Flags().StringVar(&opts.Password, "password", "", "meeting password (4-6 digits)")
	cmd.Flags().StringVar(&opts.Timezone, "timezone", "", "timezone (e.g. Asia/Shanghai)")
	cmd.Flags().IntVar(&opts.MeetingType, "meeting-type", 0, "meeting type: 0-normal, 1-recurring")
	cmd.Flags().IntVar(&opts.OnlyUserJoinType, "join-type", 0, "join restriction: 1-all members, 2-invited only, 3-internal only")
	cmd.Flags().BoolVar(&opts.AutoInWaitingRoom, "waiting-room", false, "enable waiting room")
	cmd.Flags().IntVar(&opts.RecurringType, "recurring-type", 0, "recurring type (meeting-type=1): 0-daily, 1-weekdays, 2-weekly, 3-biweekly, 4-monthly, 5-custom")
	cmd.Flags().IntVar(&opts.UntilType, "until-type", 0, "recurring end type (meeting-type=1): 0-by date, 1-by count")
	cmd.Flags().IntVar(&opts.UntilCount, "until-count", 7, "recurring count (meeting-type=1, max: 500 for daily/weekdays/weekly, 50 for biweekly/monthly)")
	cmd.Flags().StringVar(&opts.UntilDate, "until-date", "", "recurring end date (meeting-type=1) e.g. 2026-03-12T15:00+08:00)")

	_ = cmd.MarkFlagRequired("subject")
	_ = cmd.MarkFlagRequired("start")
	_ = cmd.MarkFlagRequired("end")
	return cmd
}

func (o *CreateOptions) Run(cmd *cobra.Command, args []string) error {
	// Time conversion.
	startTime, err := utils.ISO8601ToTimeStamp(o.StartTime)
	if err != nil {
		return exception.InvalidArgsError.With("--start format error: %v", err)
	}
	endTime, err := utils.ISO8601ToTimeStamp(o.EndTime)
	if err != nil {
		return exception.InvalidArgsError.With("--end format error: %v", err)
	}

	// Execute create meeting.
	params := map[string]interface{}{
		"subject":      o.Subject,
		"start_time":   strconv.FormatInt(startTime, 10),
		"end_time":     strconv.FormatInt(endTime, 10),
		"password":     o.Password,
		"timezone":     o.Timezone,
		"meeting_type": o.MeetingType,
		"type":         0, // scheduled meeting, fixed value
		"instanceid":   1, // PC, fixed value
		"userid":       o.tmeet.UserConfig.OpenId,
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
				log.Errorf(cmd, "--until-date format error: %v", err)
				return nil
			}
			recurringRule["until_date"] = untilDate
		}
		params["recurring_rule"] = recurringRule
	}
	// setting参数
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
		ApiURI: "/v1/meetings",
		Body:   params,
	}
	rsp, err := restProxy.RequestProxy(cmd.Context(), http.MethodPost, o.tmeet, req)
	if err != nil {
		return err
	}

	// Parse response, recursively convert timestamp fields to ISO8601 format.
	rsp.Data = string(utils.ConvertFields([]byte(rsp.Data), 10, map[string]utils.FieldConverter{
		"start_time": utils.TimestampConverter,
		"end_time":   utils.TimestampConverter,
	}))
	log.Infof(cmd, restProxy.Print(cmd, rsp))
	return nil
}
