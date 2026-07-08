package meeting

import (
	"net/http"
	"strconv"
	"strings"
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

// Invitees mutation strategies for `meeting update --invitees-type`.
// The CLI string maps to the integer enum `invitees_operate_type` expected
// by the server:
//

// add     -> 1  add invitees
// remove  -> 2  remove invitees
// replace -> 3  replace all invitees
const (
	inviteesTypeReplace = "replace"
	inviteesTypeAdd     = "add"
	inviteesTypeRemove  = "remove"

	inviteesOperateTypeAdd     = 1
	inviteesOperateTypeRemove  = 2
	inviteesOperateTypeReplace = 3
)

// inviteesOperateTypeMap maps the user-facing strategy string to the server
// enum value for `invitees_operate_type`.
var inviteesOperateTypeMap = map[string]int{
	inviteesTypeReplace: inviteesOperateTypeReplace,
	inviteesTypeAdd:     inviteesOperateTypeAdd,
	inviteesTypeRemove:  inviteesOperateTypeRemove,
}

// UpdateOptions holds the options for updating a meeting.
type UpdateOptions struct {
	tmeet             *internal.Tmeet
	MeetingID         string   // Meeting ID
	Subject           string   // Meeting subject
	StartTime         string   // Meeting start time, ISO 8601, e.g. 2026-03-12T15:00+08:00
	EndTime           string   // Meeting end time, ISO 8601, e.g. 2026-03-12T15:00+08:00
	Password          string   // Meeting password (4-6 digits)
	Timezone          string   // Timezone, e.g. Asia/Shanghai
	MeetingType       int      // Meeting type. 0: normal meeting, 1: recurring meeting
	OnlyUserJoinType  int      // Member join restriction. 1: all members, 2: invited only, 3: internal only
	AutoInWaitingRoom bool     // Whether to enable waiting room
	RecurringType     int      // Recurring meeting config (required when meetingType=1). Recurrence type, default 0. 0: daily, 1: weekdays, 2: weekly, 3: biweekly, 4: monthly
	UntilType         int      // Recurring meeting config (required when meetingType=1). End type, default 0. 0: end by date, 1: end by count
	UntilCount        int      // Recurring meeting config (required when meetingType=1). Max occurrences. Daily/weekday/weekly max 500; biweekly/monthly max 50. Default 7.
	UntilDate         string   // Recurring meeting config (required when meetingType=1). End date
	SubMeetingID      string   // Recurring meeting config (optional, when meetingType=1). Sub-meeting ID: modify a single sub-meeting's time only; mutually exclusive with recurring-rule fields (recurring-type/until-type/until-count/until-date). If empty, the whole recurring meeting is updated.
	Invitees          []string // Invited participants (openid list).
	InviteesType      string   // Invitees mutation strategy: replace / add / remove.
	WaterMarkType     int      // Text watermark, 0=single row, 1=double row, 2=off
	AudioWatermark    bool     // Audio watermark, true=on, false=off
	AutoRecordType    string   // Auto record when host joins, none=off, local=local recording, cloud=cloud recording
	AutoASR           bool     // Auto speech recognition, true=on, false=off
}

// newUpdateCmd updates a meeting.
func newUpdateCmd(tmeet *internal.Tmeet) *cobra.Command {
	opts := &UpdateOptions{tmeet: tmeet}
	cmd := &cobra.Command{
		Use:   "update",
		Short: "update meeting",
		RunE: middleWare.Chain(
			opts.Run,
			middleWare.WithApiCmd(cmdutil.StaticApiCmd(cmdutil.ApiCmdMeetingUpdate)),
		),
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
	cmd.Flags().IntVar(&opts.RecurringType, "recurring-type", 0, "recurring type (0-daily, 1-weekdays, 2-weekly, 3-biweekly, 4-monthly)")
	cmd.Flags().IntVar(&opts.UntilType, "until-type", 0, "until type (0-date, 1-count)")
	cmd.Flags().IntVar(&opts.UntilCount, "until-count", 7, "until count")
	cmd.Flags().StringVar(&opts.UntilDate, "until-date", "", "until date e.g. 2026-03-12T15:00+08:00)")
	cmd.Flags().StringVar(&opts.SubMeetingID, "sub-meeting-id", "",
		"sub-meeting ID: modify a single sub-meeting's time only (recurring meeting). "+
			"mutually exclusive with --recurring-type / --until-type / --until-count / --until-date. "+
			"if not set, the whole recurring meeting is updated")
	cmd.Flags().StringSliceVar(&opts.Invitees, "invitees", nil,
		"invitee openid list, comma-separated or repeat the flag (used together with --invitees-type)")
	cmd.Flags().StringVar(&opts.InviteesType, "invitees-type", "",
		"invitees mutation strategy: replace | add | remove (required when --invitees is set)")
	cmd.Flags().IntVar(&opts.WaterMarkType, "water-mark-type", 2, "text watermark: 0=single row, 1=double row, 2=off")
	cmd.Flags().BoolVar(&opts.AudioWatermark, "audio-watermark", false, "audio watermark")
	cmd.Flags().StringVar(&opts.AutoRecordType, "auto-record-type", "none", "auto record when host joins: none=off, local=local recording, cloud=cloud recording")
	cmd.Flags().BoolVar(&opts.AutoASR, "auto-asr", false, "auto speech recognition")

	// Set required parameters.
	_ = cmd.MarkFlagRequired("meeting-id")

	return cmd
}

// Hints implements the Hints.HintProvider interface and returns hints for meeting settings.
// Lock detection is solely driven by the corp_lock_mask bitmask in the response.
func (o *UpdateOptions) Hints(responseData string) []string {
	return cmdutil.GenerateMeetingSettingsHints(responseData)
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
		recurringRule, err := o.buildRecurringRule(cmd)
		if err != nil {
			return err
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
	if len(settings) > 0 {
		params["settings"] = settings
	}

	// Resolve --invitees / --invitees-type and inject into request body.
	// The strategy is forwarded as `invitees_operate_type` and the userid
	// list is sent verbatim; the server applies the mutation.
	if err := o.applyInvitees(cmd, params); err != nil {
		return err
	}

	req := &thttp.Request{
		ApiURI: "/v1/meetings/" + o.MeetingID,
		Body:   params,
	}
	rsp, err := restProxy.RequestProxy(cmd.Context(), http.MethodPut, o.tmeet, req)
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

// buildRecurringRule assembles the `recurring_rule` payload for a recurring
// meeting update. When --sub-meeting-id is provided, only that single
// sub-meeting's time is modified and it must not be combined with the
// recurring-rule fields (--recurring-type / --until-type / --until-count /
// --until-date); otherwise the whole recurring meeting is updated using the
// recurring-rule fields.
func (o *UpdateOptions) buildRecurringRule(cmd *cobra.Command) (map[string]interface{}, error) {
	if o.SubMeetingID != "" {
		conflicting := []string{"recurring-type", "until-type", "until-count", "until-date"}
		for _, name := range conflicting {
			if cmd.Flags().Changed(name) {
				return nil, exception.InvalidArgsError.With(
					"--sub-meeting-id cannot be used together with --%s: "+
						"either update a single sub-meeting's time, or update the recurring rule", name)
			}
		}
		return map[string]interface{}{"sub_meeting_id": o.SubMeetingID}, nil
	}

	recurringRule := make(map[string]interface{}, 4)
	recurringRule["until_type"] = o.UntilType
	recurringRule["until_count"] = o.UntilCount
	recurringRule["recurring_type"] = o.RecurringType
	if o.UntilDate != "" {
		untilDate, err := utils.ISO8601ToTimeStamp(o.UntilDate)
		if err != nil {
			return nil, exception.InvalidArgsError.With("--until-date format error: %v", err)
		}
		recurringRule["until_date"] = untilDate
	}
	return recurringRule, nil
}

// applyInvitees validates --invitees / --invitees-type and writes the
// resulting `invitees` and `invitees_operate_type` fields into params when
// the user opts in. The two flags must be set together; the userid list is
// only normalized (trim, dedup, drop empties) and forwarded as-is — the
// server is responsible for merging/removing against the existing list.
func (o *UpdateOptions) applyInvitees(cmd *cobra.Command, params map[string]interface{}) error {
	inviteesSet := cmd.Flags().Changed("invitees")
	typeSet := cmd.Flags().Changed("invitees-type")
	if !inviteesSet && !typeSet {
		return nil
	}
	if !typeSet {
		return exception.InvalidArgsError.With(
			"--invitees-type is required when --invitees is set (replace | add | remove)")
	}
	if !inviteesSet {
		return exception.InvalidArgsError.With(
			"--invitees is required when --invitees-type is set")
	}

	strategy := strings.ToLower(strings.TrimSpace(o.InviteesType))
	operateType, ok := inviteesOperateTypeMap[strategy]
	if !ok {
		return exception.InvalidArgsError.With(
			"--invitees-type must be one of: replace, add, remove (got %q)", o.InviteesType)
	}

	inviteesList, err := cmdutil.PackageApiInviteesUsers("--invitees", o.Invitees)
	if err != nil {
		return err
	}

	params["invitees"] = inviteesList
	params["invitees_operate_type"] = operateType
	return nil
}
