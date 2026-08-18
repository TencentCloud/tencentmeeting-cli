package meeting

import (
	"encoding/json"
	"fmt"
	"net/http"

	"tmeet/internal"
	"tmeet/internal/cmdutil"
	middleWare "tmeet/internal/cmdutil/middleware"
	"tmeet/internal/config"
	"tmeet/internal/core/thttp"
	"tmeet/internal/exception"
	"tmeet/internal/log"
	"tmeet/internal/output"
	restProxy "tmeet/internal/proxy/rest-proxy"

	"github.com/spf13/cobra"
)

// Fixed parameters used when the agent joins a meeting on behalf of the master account.
const (
	// agentOperatorIdTypeOpenId is the operator_id_type value for OpenId-typed operator_id.
	agentOperatorIdTypeOpenId = 2
	// agentJoinInstanceId is the terminal device type for the agent join call,
	// fixed to 57 to indicate a robot client joining the meeting.
	agentJoinInstanceId = 57
	// agentJoinRobotUserType marks the joining client as a CLI robot.
	agentJoinRobotUserType = 11
	// agentAsrInstanceId is the terminal device type for the ASR control call, fixed to PC for CLI.
	agentAsrInstanceId = 1
	// agentJoinType is the join type, fixed to 1 indicating join by meeting code.
	agentJoinType = 1
)

// JoinAsAgentOptions holds the options for the `meeting join-as-agent` command.
//
// The command lets the agent (sub-account) join a meeting to listen on behalf
// of the master account, then automatically turns on real-time transcription
// so that the user can review the meeting content afterwards.
type JoinAsAgentOptions struct {
	tmeet           *internal.Tmeet
	MeetingCode     string
	AgentId         string
	Password        string
	RequireRealtime bool
}

// newJoinAsAgentCmd builds the `meeting join-as-agent` command.
func newJoinAsAgentCmd(tmeet *internal.Tmeet) *cobra.Command {
	opts := &JoinAsAgentOptions{tmeet: tmeet}
	cmd := &cobra.Command{
		Use:   "join-as-agent",
		Short: "Let the agent join a meeting to listen and auto-enable real-time transcription",
		Long: "Let the agent (sub-account) join the specified meeting on behalf of the master account " +
			"and automatically turn on real-time transcription, so that the user can review the meeting " +
			"content afterwards when unable to attend in person.",
		RunE: middleWare.Chain(
			opts.Run,
			middleWare.WithApiCmd(cmdutil.StaticApiCmd(cmdutil.ApiCmdMeetingJoinAsAgent)),
		),
	}

	// fill flags
	cmd.Flags().StringVar(&opts.MeetingCode, "meeting-code", "", "meeting code (required)")
	cmd.Flags().StringVar(&opts.AgentId, "agent-id", "", "agent ID (required)")
	cmd.Flags().StringVar(&opts.Password, "password", "", "meeting password, usually 4-6 digits (optional)")
	cmd.Flags().BoolVar(&opts.RequireRealtime, "require-realtime-asr", false, "if true and ASR fails, bot will auto-leave the meeting with reason asr-denied")

	// mark required
	_ = cmd.MarkFlagRequired("meeting-code")
	_ = cmd.MarkFlagRequired("agent-id")

	return cmd
}

// Run executes the join-as-agent command: agent joins the meeting, then turns on ASR.
//
// Output strategy:
//   - join failure: return the error directly, no output is produced;
//   - join + ASR succeed: emit a single response whose trace_id is the
//     comma-joined "<joinTrace>,<asrTrace>" and whose data only contains
//     meeting_id from the join response;
//   - join succeeds but ASR fails:
//   - if --require-realtime-asr is true: auto-leave the meeting and return error;
//   - otherwise (default): emit a single response with only the join trace_id, the
//     meeting_id data, and a hint telling the user that ASR could not be enabled.
func (o *JoinAsAgentOptions) Run(cmd *cobra.Command, args []string) error {
	if o.MeetingCode == "" {
		return exception.InvalidArgsError.With("--meeting-code is required")
	}

	// Validate --agent-id against the locally stored agent configuration.
	agentCfg, err := config.GetAgentAccountConfig()
	if err != nil {
		return err
	}
	if agentCfg == nil || agentCfg.AgentOpenId != o.AgentId {
		return exception.AgentNotFoundError
	}

	// Step 1: agent joins the meeting.
	//
	// operator_id / operator_id_type still carry the master-account OpenId; the
	// agent identity is conveyed by RequestProxyWithAgent via the
	// Tmeet-Agent-ID / Tmeet-Agent-AK headers.
	joinBody := map[string]interface{}{
		"operator_id":      o.tmeet.UserConfig.OpenId,
		"operator_id_type": agentOperatorIdTypeOpenId,
		"instanceid":       agentJoinInstanceId,
		"robot_user_type":  agentJoinRobotUserType,
		"join_type":        agentJoinType,
	}
	if o.Password != "" {
		joinBody["password"] = o.Password
	}
	joinReq := &thttp.Request{
		ApiURI: "/v1/meetings/{meeting_code}/join",
		Body:   joinBody,
		PathParams: thttp.PathParams{
			"meeting_code": cmdutil.FormatMeetingCode(o.MeetingCode),
		},
	}
	joinRsp, err := restProxy.RequestProxyWithAgent(cmd.Context(), http.MethodPost, o.tmeet, joinReq)
	if err != nil {
		// Join failed: surface the error directly so the command exits non-zero.
		return err
	}

	// Extract meeting_id and room_id from the join response for subsequent calls.
	meetingId := extractMeetingId(joinRsp.Data)
	roomId := extractRoomId(joinRsp.Data)

	// Step 2: turn on real-time transcription right after the join succeeds.
	//
	// Before enabling ASR, check whether the meeting already has real-time
	// transcription turned on. If it is already enabled, skip the enableAsr call.
	// ASR failures are handled based on --require-realtime-asr flag:
	// - true: auto-leave the meeting and return error (asr-denied)
	// - false: only log and hint, command still succeeds
	var asrTraceId string
	var asrOK bool

	if o.isAsrEnabled(cmd, meetingId, roomId) {
		// ASR is already enabled, no need to call enableAsr.
		log.Infof(cmd.Context(), "real-time transcription is already enabled for meeting %s, skip enableAsr", o.MeetingCode)
		asrOK = true
	} else {
		asrTraceId, asrOK = o.enableAsr(cmd, meetingId)
	}

	// If ASR failed and --require-realtime-asr is true, auto-leave and return error.
	if !asrOK && o.RequireRealtime {
		o.autoLeave(cmd, meetingId)
		return exception.AsrDeniedError
	}

	// Build the merged output: trace_id is "<joinTrace>[,<asrTrace>]",
	// data keeps only meeting_id from the join response, and a hint is added
	// when ASR failed.
	traceId := joinRsp.TraceId
	if asrOK && asrTraceId != "" {
		traceId = joinRsp.TraceId + "," + asrTraceId
	}

	mergedData := buildAgentMeetingIdOnlyData(joinRsp.Data)

	var printOpts []output.Option
	if !asrOK {
		printOpts = append(printOpts, output.WithHints(func(string) []string {
			return []string{"joined the meeting successfully, but failed to enable real-time transcription"}
		}))
	}

	output.FormatPrint(cmd, traceId, joinRsp.Message, mergedData, printOpts...)

	return nil
}

// isAsrEnabled queries the current real-time transcription state of the meeting.
// It returns true if subtitle_state == 1 (enabled), false otherwise.
//
// Failures are swallowed (only logged) and treated as "not enabled" so that the
// caller will proceed to call enableAsr.
func (o *JoinAsAgentOptions) isAsrEnabled(cmd *cobra.Command, meetingId string, roomId string) bool {
	ctx := cmd.Context()

	queryReq := &thttp.Request{
		ApiURI: "/v1/asr/query-in-meeting-subtitle-state",
		QueryParams: thttp.QueryParams{
			"operator_id_type": []string{fmt.Sprint(agentOperatorIdTypeOpenId)},
			"operator_id":      []string{o.tmeet.UserConfig.OpenId},
			"meeting_id":       []string{meetingId},
			"room_id":          []string{roomId},
		},
	}

	rsp, err := restProxy.RequestProxyWithAgent(ctx, http.MethodGet, o.tmeet, queryReq)
	if err != nil {
		log.Errorf(ctx, "query in-meeting subtitle state failed: %v", err)
		return false
	}

	var result struct {
		SubtitleState int `json:"subtitle_state"`
	}
	if err = json.Unmarshal([]byte(rsp.Data), &result); err != nil {
		log.Errorf(ctx, "decode subtitle state response failed: %v", err)
		return false
	}

	return result.SubtitleState == 1
}

// extractMeetingId extracts meeting_id from the join response data.
// Returns empty string if extraction fails.
func extractMeetingId(raw string) string {
	var data struct {
		MeetingId string `json:"meeting_id"`
	}
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		return ""
	}
	return data.MeetingId
}

// extractRoomId extracts room_id from the join response data.
// The join response contains a nested structure: media_platform_info_meeting.room_id.
// Returns empty string if extraction fails.
func extractRoomId(raw string) string {
	var data struct {
		MediaPlatformMeetingInfo struct {
			RoomId string `json:"room_id"`
		} `json:"media_platform_info_meeting"`
	}
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		return ""
	}
	return data.MediaPlatformMeetingInfo.RoomId
}

// buildAgentMeetingIdOnlyData extracts the meeting_id field from the raw join
// response data and returns it as a JSON string suitable for output.FormatPrint.
//
// If the raw payload cannot be parsed or does not contain meeting_id, an empty
// JSON object ("{}") is returned so the output remains a valid object rather
// than leaking unrelated fields.
func buildAgentMeetingIdOnlyData(raw string) string {
	src := make(map[string]interface{})
	if err := json.Unmarshal([]byte(raw), &src); err != nil {
		return "{}"
	}
	out := map[string]interface{}{}
	if v, ok := src["meeting_id"]; ok {
		out["meeting_id"] = v
	}
	return marshalOrEmpty(out)
}

// marshalOrEmpty marshals the given map to JSON string, returning "{}" on error.
func marshalOrEmpty(m map[string]interface{}) string {
	b, err := json.Marshal(m)
	if err != nil {
		return "{}"
	}
	return string(b)
}

// enableAsr turns on real-time transcription for the meeting under the agent
// identity. It returns the ASR trace id and whether the call succeeded.
//
// Failures are intentionally swallowed (only logged) so that a successful
// join is preserved; the caller is responsible for surfacing the failure to
// the user (for example via a hint in the merged output or auto-leave).
func (o *JoinAsAgentOptions) enableAsr(cmd *cobra.Command, meetingId string) (string, bool) {
	ctx := cmd.Context()

	asrBody := map[string]interface{}{
		"operator_id":      o.tmeet.UserConfig.OpenId,
		"operator_id_type": agentOperatorIdTypeOpenId,
		"instance_id":      agentAsrInstanceId,
		"is_open":          true,
	}
	asrReq := &thttp.Request{
		ApiURI: "/v1/real-control/meetings/{meeting_id}/asr",
		Body:   asrBody,
		PathParams: thttp.PathParams{
			"meeting_id": meetingId,
		},
	}

	asrRsp, err := restProxy.RequestProxyWithAgent(ctx, http.MethodPut, o.tmeet, asrReq)
	if err != nil {
		// Do not surface the error to the user: the join already succeeded,
		// and an ASR failure should not turn the whole command into a failure.
		// Just record it in the log for later troubleshooting.
		log.Errorf(ctx, "enable real-time transcription failed: %v", err)
		return "", false
	}

	return asrRsp.TraceId, true
}

// autoLeave makes the agent leave the meeting after ASR failure when
// --require-realtime-asr is true. Errors are only logged, not surfaced.
func (o *JoinAsAgentOptions) autoLeave(cmd *cobra.Command, meetingId string) {
	ctx := cmd.Context()

	body := map[string]interface{}{
		"operator_id":      o.tmeet.UserConfig.OpenId,
		"operator_id_type": agentOperatorIdTypeOpenId,
		"instanceid":       agentJoinInstanceId,
		"robot_user_type":  agentJoinRobotUserType,
		"leave_type":       0,
		"reason_code":      1,
	}
	req := &thttp.Request{
		ApiURI: "/v1/meetings/{meeting_id}/leave",
		Body:   body,
		PathParams: thttp.PathParams{
			"meeting_id": meetingId,
		},
	}

	_, err := restProxy.RequestProxyWithAgent(ctx, http.MethodPost, o.tmeet, req)
	if err != nil {
		log.Errorf(ctx, "auto-leave meeting after ASR failure failed: %v", err)
	}
}
