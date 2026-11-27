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
	"tmeet/internal/utils/enumerate"

	"github.com/spf13/cobra"
)

// CreateOptions holds the options for creating a meeting.
type CreateOptions struct {
	tmeet *internal.Tmeet

	// Subject is the meeting subject (required).
	Subject string

	// StartTime is the meeting start time in ISO 8601 format,
	// e.g. 2026-03-12T14:00+08:00 (required).
	StartTime string

	// EndTime is the meeting end time in ISO 8601 format,
	// e.g. 2026-03-12T15:00+08:00 (required).
	EndTime string

	// Password is the meeting password (4-6 digits), optional.
	Password string

	// Timezone follows the Oracle-TimeZone standard, e.g. Asia/Shanghai.
	Timezone string

	// MeetingType is the meeting type, default 0.
	// 0: normal meeting, 1: recurring meeting.
	MeetingType int

	// OnlyUserJoinType is the member join restriction.
	// 1: all members, 2: invited only, 3: internal only.
	OnlyUserJoinType int

	// AutoInWaitingRoom controls whether the waiting room is enabled,
	// default false.
	AutoInWaitingRoom bool

	// RecurringType is required when meetingType=1. Recurrence type, default 0.
	// 0: daily, 1: weekdays, 2: weekly, 3: biweekly, 4: monthly.
	RecurringType int

	// UntilType is required when meetingType=1. End type, default 0.
	// 0: end by date, 1: end by count.
	UntilType int

	// UntilCount is required when meetingType=1. Max occurrences.
	// Daily/weekday/weekly max 500; biweekly/monthly max 50. Default 7.
	UntilCount int

	// UntilDate is required when meetingType=1. End date in ISO 8601,
	// e.g. 2026-03-12T15:00+08:00.
	UntilDate string

	// Invitees is the list of invited participants' openids. Up to 100 users.
	Invitees []string

	// WaterMarkType is the text watermark setting.
	// 0: single row, 1: double row, 2: off.
	WaterMarkType int

	// AudioWatermark toggles the audio watermark. true=on, false=off.
	AudioWatermark bool

	// AutoRecordType is the auto-record policy when the host joins.
	// none=off, cloud=cloud recording.
	AutoRecordType string

	// AutoASR toggles the auto speech recognition. true=on, false=off.
	AutoASR bool

	// AllowAfterMeetingView allows participants to view the transcript
	// after the meeting. true=allow, false=deny.
	AllowAfterMeetingView bool

	// Description is the meeting description text (<=2000 UTF-8 chars).
	// Only sent when the flag is explicitly set.
	Description string

	// Attachments is the list of local attachment paths
	// (<=20 items, aggregate <=100MB).
	Attachments []string
}

// newCreateCmd creates a meeting.
func newCreateCmd(tmeet *internal.Tmeet) *cobra.Command {
	opts := &CreateOptions{tmeet: tmeet, AutoRecordType: string(enumerate.AutoRecordTypeNone)}
	cmd := &cobra.Command{
		Use:   "create",
		Short: "create meeting",
		RunE: middleWare.Chain(
			opts.Run,
			middleWare.WithApiCmd(cmdutil.StaticApiCmd(cmdutil.ApiCmdMeetingCreate)),
		),
	}

	// Fill in parameters.
	cmd.Flags().StringVar(&opts.Subject, "subject", "", "meeting subject (required)")
	cmd.Flags().StringVar(&opts.StartTime, "start", "",
		"meeting start time (ISO 8601, e.g. 2026-03-12T14:00+08:00) (required)")
	cmd.Flags().StringVar(&opts.EndTime, "end", "",
		"meeting end time (ISO 8601, e.g. 2026-03-12T15:00+08:00) (required)")
	cmd.Flags().StringVar(&opts.Password, "password", "", "meeting password (4-6 digits)")
	cmd.Flags().StringVar(&opts.Timezone, "timezone", "", "timezone (e.g. Asia/Shanghai)")
	cmd.Flags().IntVar(&opts.MeetingType, "meeting-type", 0, "meeting type: 0-normal, 1-recurring")
	cmd.Flags().IntVar(&opts.OnlyUserJoinType, "join-type", 0,
		"join restriction: 1-all members, 2-invited only, 3-internal only")
	cmd.Flags().BoolVar(&opts.AutoInWaitingRoom, "waiting-room", false, "enable waiting room")
	cmd.Flags().IntVar(&opts.RecurringType, "recurring-type", 0,
		"recurring type (meeting-type=1): "+
			"0-daily, 1-weekdays, 2-weekly, 3-biweekly, 4-monthly")
	cmd.Flags().IntVar(&opts.UntilType, "until-type", 0,
		"recurring end type (meeting-type=1): 0-by date, 1-by count")
	cmd.Flags().IntVar(&opts.UntilCount, "until-count", 7,
		"recurring count (meeting-type=1, max: "+
			"500 for daily/weekdays/weekly, 50 for biweekly/monthly)")
	cmd.Flags().StringVar(&opts.UntilDate, "until-date", "",
		"recurring end date (meeting-type=1) e.g. 2026-03-12T15:00+08:00)")
	cmd.Flags().StringSliceVar(&opts.Invitees, "invitees", nil,
		"invited participants' openid list, comma-separated or repeat the flag (max 100, e.g. --invitees open_id1,open_id2)")
	cmd.Flags().IntVar(&opts.WaterMarkType, "water-mark-type", 2, "text watermark: 0=single row, 1=double row, 2=off")
	cmd.Flags().BoolVar(&opts.AudioWatermark, "audio-watermark", false, "audio watermark")
	cmd.Flags().Var(&cmdutil.EnumValue{Value: &opts.AutoRecordType, Allowed: enumerate.AutoRecordTypeValues()},
		"auto-record-type", "auto record when host joins: none=off, cloud=cloud recording")
	cmd.Flags().BoolVar(&opts.AutoASR, "auto-asr", false, "auto speech")
	cmd.Flags().BoolVar(&opts.AllowAfterMeetingView, "allow-after-meeting-view", false,
		"allow participants to view the transcript after the meeting (true=allow, false=deny); omitted = not sent to API")
	cmd.Flags().StringVar(&opts.Description, "description", "",
		"meeting description text (<=2000 UTF-8 chars)")
	cmd.Flags().StringArrayVar(&opts.Attachments, "attachment", nil,
		"local attachment path; repeat the flag for multiple files "+
			"(max 20, aggregate 100MB)")

	_ = cmd.MarkFlagRequired("subject")
	_ = cmd.MarkFlagRequired("start")
	_ = cmd.MarkFlagRequired("end")
	return cmd
}

// Hints implements the hints.HintProvider interface. Lock detection comes from
// the create response.
func (o *CreateOptions) Hints(responseData string) []string {
	return cmdutil.GenerateMeetingSettingsHints(responseData)
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
				output.PrintErrorf(cmd, "--until-date format error: %v", err)
				return nil
			}
			recurringRule["until_date"] = untilDate
		}
		params["recurring_rule"] = recurringRule
	}
	// setting param
	settings := make(map[string]interface{}, 0)
	if cmd.Flags().Changed("join-type") {
		settings["only_user_join_type"] = o.OnlyUserJoinType
	}
	if cmd.Flags().Changed("waiting-room") {
		settings["auto_in_waiting_room"] = o.AutoInWaitingRoom
	}
	if cmd.Flags().Changed("water-mark-type") {
		if o.WaterMarkType != 2 {
			settings["water_mark_type"] = o.WaterMarkType
		}
		settings["allow_screen_shared_watermark"] = o.WaterMarkType != 2
	}
	if cmd.Flags().Changed("audio-watermark") {
		settings["audio_watermark"] = o.AudioWatermark
	}
	if cmd.Flags().Changed("auto-record-type") {
		settings["auto_record_type"] = o.AutoRecordType
	}
	if cmd.Flags().Changed("auto-asr") {
		settings["auto_asr"] = o.AutoASR
	}
	if cmd.Flags().Changed("allow-after-meeting-view") {
		settings["allow_after_meeting_view"] = o.AllowAfterMeetingView
	}
	if len(settings) > 0 {
		params["settings"] = settings
	}

	// Invitees: validated, deduped and capped (max 100) by the shared helper.
	if len(o.Invitees) > 0 {
		invitees, err := cmdutil.PackageApiInviteesUsers("--invitees", o.Invitees)
		if err != nil {
			return err
		}
		params["invitees"] = invitees
	}

	// Description & attachments: pre-validate text length, upload all local
	// files first, and only then attach the composed `description` object.
	// Any upload failure aborts the request before the create call is
	// issued, so the server never sees a partially-uploaded meeting.
	if cmd.Flags().Changed("description") || cmd.Flags().Changed("attachment") {
		desc, err := o.buildDescription(cmd)
		if err != nil {
			return err
		}
		params["description"] = desc
	}

	req := &thttp.Request{
		ApiURI: "/v1/meetings",
		Body:   params,
	}
	rsp, err := restProxy.RequestProxy(cmd.Context(), http.MethodPost, o.tmeet, req)
	if err != nil {
		return err
	}

	convertMap := map[string]utils.FieldConverter{
		"start_time":          utils.TimestampConverter,
		"end_time":            utils.TimestampConverter,
		"only_user_join_type": utils.MeetingJoinTypeConverter,
	}

	output.FormatPrint(cmd, rsp.TraceId, rsp.Message, rsp.Data,
		output.WithConvert(convertMap),
		output.WithHints(o.Hints))
	return nil
}

// buildDescription composes the `description` payload for create: validates
// the text length, uploads any local attachments, and returns the merged
// object. existingCount/existingBytes are 0 for create — only update needs
// the merge-against-existing semantics.
func (o *CreateOptions) buildDescription(cmd *cobra.Command) (map[string]interface{}, error) {
	desc := map[string]interface{}{}
	if cmd.Flags().Changed("description") {
		if err := ValidateDescription(o.Description); err != nil {
			return nil, err
		}
		desc["text"] = o.Description
	}
	if cmd.Flags().Changed("attachment") && len(o.Attachments) > 0 {
		refs, err := UploadLocalAttachments(cmd.Context(), o.tmeet, o.Attachments, 0, 0)
		if err != nil {
			return nil, err
		}
		// Project to the minimal submit DTO: the server only needs
		// file_id + file_name on description.attachments[].
		desc["attachments"] = toSubmitAttachmentRefs(refs)
	}
	return desc, nil
}
