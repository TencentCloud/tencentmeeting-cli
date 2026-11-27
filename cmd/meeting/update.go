package meeting

import (
	"encoding/json"
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
	"tmeet/internal/utils/enumerate"

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
	tmeet *internal.Tmeet

	// MeetingID is the meeting ID (required).
	MeetingID string

	// Subject is the meeting subject.
	Subject string

	// StartTime is the meeting start time in ISO 8601 format,
	// e.g. 2026-03-12T15:00+08:00.
	StartTime string

	// EndTime is the meeting end time in ISO 8601 format,
	// e.g. 2026-03-12T15:00+08:00.
	EndTime string

	// Password is the meeting password (4-6 digits).
	Password string

	// Timezone follows the Oracle-TimeZone standard, e.g. Asia/Shanghai.
	Timezone string

	// MeetingType is the meeting type.
	// 0: normal meeting, 1: recurring meeting.
	MeetingType int

	// OnlyUserJoinType is the member join restriction.
	// 1: all members, 2: invited only, 3: internal only.
	OnlyUserJoinType int

	// AutoInWaitingRoom controls whether the waiting room is enabled.
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

	// SubMeetingID is the recurring meeting sub-meeting ID (optional, when
	// meetingType=1). When set, only that single sub-meeting's time is
	// modified; it is mutually exclusive with the recurring-rule fields
	// (recurring-type / until-type / until-count / until-date). If empty,
	// the whole recurring meeting is updated.
	SubMeetingID string

	// Invitees is the list of invited participants' openids.
	Invitees []string

	// InviteesType is the invitees mutation strategy:
	// replace / add / remove.
	InviteesType string

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

	// Description is the replacement description text; an explicit empty
	// string clears existing text.
	Description string

	// AttachmentsAdd lists local file paths to append as new attachments.
	AttachmentsAdd []string

	// AttachmentsRemove lists file IDs to remove from the existing
	// attachment list.
	AttachmentsRemove []string

	// AttachmentsClear removes ALL attachments; mutually exclusive with
	// AttachmentsAdd / AttachmentsRemove.
	AttachmentsClear bool
}

// newUpdateCmd updates a meeting.
func newUpdateCmd(tmeet *internal.Tmeet) *cobra.Command {
	opts := &UpdateOptions{tmeet: tmeet, AutoRecordType: string(enumerate.AutoRecordTypeNone)}
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
	cmd.Flags().IntVar(&opts.OnlyUserJoinType, "join-type", 0,
		"join restriction: 1-all members, 2-invited only, 3-internal only")
	cmd.Flags().BoolVar(&opts.AutoInWaitingRoom, "waiting-room", false, "enable waiting room")
	cmd.Flags().IntVar(&opts.RecurringType, "recurring-type", 0,
		"recurring type (0-daily, 1-weekdays, 2-weekly, 3-biweekly, 4-monthly)")
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
	cmd.Flags().Var(&cmdutil.EnumValue{Value: &opts.AutoRecordType, Allowed: enumerate.AutoRecordTypeValues()},
		"auto-record-type", "auto record when host joins: none=off, cloud=cloud recording")
	cmd.Flags().BoolVar(&opts.AutoASR, "auto-asr", false, "auto speech recognition")
	cmd.Flags().BoolVar(&opts.AllowAfterMeetingView, "allow-after-meeting-view", false,
		"allow participants to view the transcript after the meeting (true=allow, false=deny); omitted = not sent to API")
	cmd.Flags().StringVar(&opts.Description, "description", "",
		"replacement description text (<=2000 UTF-8 chars); pass empty string to clear text")
	cmd.Flags().StringArrayVar(&opts.AttachmentsAdd, "attachment-add", nil,
		"local attachment path to append; repeat the flag for multiple files")
	cmd.Flags().StringSliceVar(&opts.AttachmentsRemove, "attachment-remove", nil,
		"file_id list to remove from the current attachments (repeat or comma-separated)")
	cmd.Flags().BoolVar(&opts.AttachmentsClear, "attachment-clear", false,
		"clear ALL attachments; mutually exclusive with --attachment-add / --attachment-remove")

	// Set required parameters.
	_ = cmd.MarkFlagRequired("meeting-id")

	return cmd
}

// Hints implements the hints.HintProvider interface. Lock detection comes from
// the update response.
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

	// Resolve --invitees / --invitees-type and inject into request body.
	// The strategy is forwarded as `invitees_operate_type` and the userid
	// list is sent verbatim; the server applies the mutation.
	if err := o.applyInvitees(cmd, params); err != nil {
		return err
	}

	// Resolve description / attachment flags and inject the merged
	// `description` payload. Empty text is sent only when --description is
	// explicitly set (see applyDescription).
	if err := o.applyDescription(cmd, params); err != nil {
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

// applyDescription merges --description / --attachment-* flags into a single
// `description` payload. The logic mirrors applyInvitees' Changed()-driven
// style, but attachments are integrated locally instead of leaving the merge
// to the server (open-platform ModifyMeeting replaces the whole attachments
// array). The resulting matrix:
//
//   - none of the flags set  -> no-op.
//   - --attachment-clear + add/remove -> rejected (mutually exclusive).
//   - --description (even empty)     -> writes `text` (empty string clears).
//   - --attachment-clear             -> writes `attachments: []`.
//   - --attachment-add/remove        -> GET current attachments, merge
//     locally (filter out --attachment-remove first, then append the newly
//     uploaded ones), and send the merged list.
func (o *UpdateOptions) applyDescription(cmd *cobra.Command, params map[string]interface{}) error {
	descChanged := cmd.Flags().Changed("description")
	addChanged := cmd.Flags().Changed("attachment-add")
	rmChanged := cmd.Flags().Changed("attachment-remove")
	clrChanged := cmd.Flags().Changed("attachment-clear") && o.AttachmentsClear

	if !descChanged && !addChanged && !rmChanged && !clrChanged {
		return nil
	}
	if clrChanged && (addChanged || rmChanged) {
		return exception.InvalidArgsError.With(
			"--attachment-clear is mutually exclusive with --attachment-add / --attachment-remove")
	}
	if descChanged {
		if err := ValidateDescription(o.Description); err != nil {
			return err
		}
	}

	desc := map[string]interface{}{}
	if descChanged {
		desc["text"] = o.Description // empty string == clear text
	}

	switch {
	case clrChanged:
		desc["attachments"] = []submitAttachmentRef{}
	case addChanged || rmChanged:
		existing, err := o.fetchExistingAttachments(cmd)
		if err != nil {
			return err
		}
		merged := existing
		if rmChanged {
			merged, err = filterOutAttachments(existing, o.AttachmentsRemove)
			if err != nil {
				return err
			}
		}
		if addChanged && len(o.AttachmentsAdd) > 0 {
			uploaded, err := UploadLocalAttachments(cmd.Context(), o.tmeet,
				o.AttachmentsAdd, len(merged), sumAttachmentSize(merged))
			if err != nil {
				return err
			}
			merged = append(merged, uploaded...)
		}
		// Project to the minimal submit DTO: the server only needs
		// file_id + file_name on description.attachments[]; local-only
		// fields (FileSize, FileSuffix) must not leak into the request.
		desc["attachments"] = toSubmitAttachmentRefs(merged)
	}

	params["description"] = desc
	return nil
}

// fetchExistingAttachments queries the current meeting detail and returns the
// `description.attachments` list. A nil slice is returned when no description
// / attachments exist yet, which is a valid state for add/remove-on-empty.
func (o *UpdateOptions) fetchExistingAttachments(cmd *cobra.Command) ([]AttachmentRef, error) {
	pathParams := thttp.PathParams{}
	pathParams.Set("meetingId", o.MeetingID)
	queryParams := thttp.QueryParams{}
	queryParams.Set("instanceid", "1") // PC
	queryParams.Set("operator_id", o.tmeet.UserConfig.OpenId)
	queryParams.Set("operator_id_type", "2") // openId

	req := &thttp.Request{
		ApiURI:      "/v1/meetings/{meetingId}",
		PathParams:  pathParams,
		QueryParams: queryParams,
	}
	rsp, err := restProxy.RequestProxy(cmd.Context(), http.MethodGet, o.tmeet, req)
	if err != nil {
		return nil, err
	}

	// Decode only the fields we need; unknown fields are ignored.
	var detail struct {
		MeetingInfoList []struct {
			Description struct {
				Attachments []AttachmentRef `json:"attachments"`
			} `json:"description"`
		} `json:"meeting_info_list"`
	}
	if err = json.Unmarshal([]byte(rsp.Data), &detail); err != nil {
		return nil, exception.InvalidArgsError.With("parse meeting detail: %v", err)
	}
	if len(detail.MeetingInfoList) == 0 {
		return nil, nil
	}
	return detail.MeetingInfoList[0].Description.Attachments, nil
}

// filterOutAttachments removes entries whose FileID appears in the removeIDs
// list. It fails fast when any removeID is NOT found in `existing`, so the
// caller never silently drops a bad request.
func filterOutAttachments(existing []AttachmentRef, removeIDs []string) ([]AttachmentRef, error) {
	if len(removeIDs) == 0 {
		return existing, nil
	}
	index := make(map[string]struct{}, len(existing))
	for _, a := range existing {
		index[a.FileID] = struct{}{}
	}
	toRemove := make(map[string]struct{}, len(removeIDs))
	for _, id := range removeIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := index[id]; !ok {
			return nil, exception.InvalidArgsError.With(
				"--attachment-remove: file_id %q is not an attachment of this meeting", id)
		}
		toRemove[id] = struct{}{}
	}
	out := make([]AttachmentRef, 0, len(existing))
	for _, a := range existing {
		if _, drop := toRemove[a.FileID]; drop {
			continue
		}
		out = append(out, a)
	}
	return out, nil
}

// sumAttachmentSize returns the aggregate byte count of attachments whose
// FileSize field is a valid integer string. Missing / malformed FileSize
// values contribute zero \u2014 this is safe because the server authoritatively
// enforces the per-meeting cap; the local sum exists only to short-circuit
// oversized add operations before any network call.
func sumAttachmentSize(refs []AttachmentRef) int64 {
	var total int64
	for _, a := range refs {
		if a.FileSize == 0 {
			continue
		}
		total += a.FileSize
	}
	return total
}
