package record

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"tmeet/internal"
	"tmeet/internal/cmdutil"
	"tmeet/internal/cmdutil/apicmdctx"
	middleWare "tmeet/internal/cmdutil/middleware"
	"tmeet/internal/core/thttp"
	"tmeet/internal/exception"
	"tmeet/internal/output"
	restProxy "tmeet/internal/proxy/rest-proxy"
	"tmeet/internal/utils"

	"github.com/spf13/cobra"
)

// Share-set sub-action values.
const (
	// shareSetActionSharing is the default action and preserves the legacy
	// behaviour: one invocation may combine a permission-point update, batch
	// add of members and batch remove of members.
	shareSetActionSharing = "sharing"
	// shareSetActionParticipantPermission sets the viewing permission granted
	// to meeting participants and invitees. Only the recording creator can set it.
	shareSetActionParticipantPermission = "participant-permission"
)

// shareSetLongDoc is the --help long description of `tmeet record share-set`.
// The two actions are independent operations: their flags must not be mixed.
const shareSetLongDoc = "Commit recording sharing changes. Pick one of the two independent operations via --action.\n" +
	"\n" +
	"Actions:\n" +
	"  --action sharing (default)      - update sharing config and/or batch add/remove authorized members.\n" +
	"  --action participant-permission - set the viewing permission of meeting participants and invitees\n" +
	"                                    (only the recording creator may do this).\n" +
	"\n" +
	"Actions and their flags (required = <>, optional = []):\n" +
	"  --action sharing <--meeting-record-id> [--switch-state] [--permission-point-id] [--pwd]\n" +
	"                   [--expired-time-type] [--expired-time] [--download-enable]\n" +
	"                   [--view-minutes-enable] [--rooms-enable] [--add-members] [--remove-members]\n" +
	"  --action participant-permission <--meeting-record-id> <--participant-perm>\n" +
	"\n" +
	"--participant-perm tiers:\n" +
	"  1 - view the transcript of the attended period only\n" +
	"  2 - view the transcript and smart minutes of the whole meeting\n" +
	"  3 - require an explicit share via the authorized-member list or share link\n" +
	"  4 - view all recap content of the whole meeting\n" +
	"\n" +
	"The two actions are mutually exclusive: a flag owned by one action is rejected by the other.\n" +
	"For --action participant-permission, --meeting-record-id is the numeric uni_record_id."

// sharingOnlyFlags lists the flags that belong exclusively to --action sharing.
// Declaring them in one table keeps the two --action operations independent and
// keeps the rejection contract in sync with registerShareSetFlags.
var sharingOnlyFlags = []string{
	"switch-state", "permission-point-id", "pwd",
	"expired-time-type", "expired-time",
	"download-enable", "view-minutes-enable", "rooms-enable",
	"add-members", "remove-members",
}

// ShareSetOptions holds the union of options for `tmeet record share-set`.
// Which flags are used depends on --action; per-action semantics are enforced
// by each ActionHandler (FlagRules / Validate / Run).
//
// For the default `sharing` action, a single invocation may combine a
// permission-point update, batch add of members and batch remove of members;
// the backend dispatches to the relevant downstream APIs as needed.
type ShareSetOptions struct {
	tmeet *internal.Tmeet

	Action string // sharing (default) | participant-permission

	MeetingRecordID string // Meeting record ID (required by both actions)

	// permission config fields (--action sharing only)
	SwitchState       bool
	PermissionPointID int
	Pwd               string
	ExpiredTimeType   int
	ExpiredTime       string // ISO 8601, e.g. 2026-03-12T14:00+08:00; only when ExpiredTimeType == 4
	DownloadEnable    bool   // whether viewers can download; ignored when corp locks download
	ViewMinutesEnable bool   // whether viewers can view transcripts and minutes
	RoomsEnable       bool   // whether Rooms devices can view this recording

	// add_members (batch add grant users, --action sharing only)
	// format: "openid:permission" (permission optional, default 0; 0=view 1=manage)
	AddMembers []string

	// remove_members (batch remove grant users, --action sharing only)
	// format: "openid"
	RemoveMembers []string

	// ParticipantPerm is the participant/invitee viewing permission tier
	// (--action participant-permission only), see shareSetLongDoc for 1..4.
	ParticipantPerm int
}

// newShareSetCmd builds the `tmeet record share-set` command. It is a command
// shell: it wires the union of flags, assembles the ActionRegistry and hands
// dispatch off to cmdutil.ActionRegistry.Dispatch, following the same pattern
// as `record content` / `record access` / `record permission-apply`.
//
// Unlike those read commands, no WithCompact middleware is mounted here: the
// `sharing` action renders a CLI-aggregated payload (results/succeeded/failed)
// rather than a single downstream response, so a server-side compact schema
// does not apply to it. WithApiCmd still resolves per action so that the
// participant-permission request carries the correct Tmeet-Cli-Name header.
func newShareSetCmd(tmeet *internal.Tmeet) *cobra.Command {
	opts := &ShareSetOptions{tmeet: tmeet}
	registry := newShareSetRegistry(opts)

	cmd := &cobra.Command{
		Use: "share-set",
		Short: "set sharing config/members or participant/invitee viewing permission " +
			"(--action sharing|participant-permission; write operation, confirm before executing)",
		Long: shareSetLongDoc,
		RunE: middleWare.Chain(
			func(cmd *cobra.Command, args []string) error {
				return registry.Dispatch(cmd, opts.Action, "action")
			},
			middleWare.WithApiCmd(registry.BuildApiCmdResolver("action")),
		),
	}

	registerShareSetFlags(cmd, opts)
	return cmd
}

// newShareSetRegistry assembles the two independent --action handlers. Keeping
// this as a separate function makes the action-to-ApiCmd mapping directly
// testable without going through the network-calling Run stage.
func newShareSetRegistry(opts *ShareSetOptions) cmdutil.ActionRegistry {
	return cmdutil.ActionRegistry{
		shareSetActionSharing:               opts.buildSharingHandler(),
		shareSetActionParticipantPermission: opts.buildParticipantPermissionHandler(),
	}
}

// registerShareSetFlags declares the union of flags consumed by either action.
// Per-action rules on which flags are required / forbidden are enforced by each
// handler's Validate + FlagRules.
func registerShareSetFlags(cmd *cobra.Command, opts *ShareSetOptions) {
	// --action defaults to `sharing` so all existing invocations keep working.
	cmdutil.SingleStringVar(cmd.Flags(), &opts.Action, "action", shareSetActionSharing,
		"operation: sharing (default) | participant-permission")

	cmd.Flags().StringVar(&opts.MeetingRecordID, "meeting-record-id", "", "meeting record id (required)")

	// sharing config flags (aligned with record-SetRecordPermissionConfigMcp)
	cmd.Flags().BoolVar(&opts.SwitchState, "switch-state", false,
		"sharing master switch; pass --switch-state=false to disable sharing (must not combine with --permission-point-id)")
	cmd.Flags().IntVar(&opts.PermissionPointID, "permission-point-id", 0,
		"sharing mode id: 1-public view, 2-public with password, 3-public with approval, "+
			"4-corp view, 5-corp with password, 6-corp with approval")
	cmd.Flags().StringVar(&opts.Pwd, "pwd", "",
		"access password (4~10 digits or letters); required when the chosen sharing mode requires a password")
	cmd.Flags().IntVar(&opts.ExpiredTimeType, "expired-time-type", 0,
		"share link expiration type: 0-forever, 1-7days, 2-15days, 3-30days, 4-custom (requires --expired-time)")
	cmd.Flags().StringVar(&opts.ExpiredTime, "expired-time", "",
		"custom expiration time (ISO 8601, e.g. 2026-12-31T23:59+08:00); only effective when --expired-time-type=4")
	cmd.Flags().BoolVar(&opts.DownloadEnable, "download-enable", false,
		"whether viewers can download the recording; ignored by backend when the enterprise locks download")
	cmd.Flags().BoolVar(&opts.ViewMinutesEnable, "view-minutes-enable", false,
		"allow members to view the recording transcript")
	cmd.Flags().BoolVar(&opts.RoomsEnable, "rooms-enable", false,
		"whether Rooms devices can view this recording")

	// add_members flags
	cmd.Flags().StringArrayVar(&opts.AddMembers, "add-members", nil,
		"members to add, format: \"openid:permission\" (permission: 0=view 1=manage, default 0), repeatable; "+
			"e.g. --add-members openid_a:0 --add-members openid_b:1")

	// remove_members flags
	cmd.Flags().StringArrayVar(&opts.RemoveMembers, "remove-members", nil,
		"members to remove, format: \"openid\", repeatable; e.g. --remove-members openid_a --remove-members openid_b")

	// participant-permission flags
	cmd.Flags().IntVar(&opts.ParticipantPerm, "participant-perm", 0,
		"participant/invitee viewing permission tier; "+
			"required by and only valid with --action participant-permission. Values:\n"+
			"  1 - transcript of the attended period only\n"+
			"  2 - whole-meeting transcript and smart minutes\n"+
			"  3 - require an explicit share (authorized-member list or share link)\n"+
			"  4 - all whole-meeting recap content\n"+
			"  5 - whole-meeting smart minutes only (without transcript)")

	_ = cmd.MarkFlagRequired("meeting-record-id")
}

// buildSharingHandler declares the `--action sharing` branch (default). It
// keeps the legacy aggregated write behaviour and rejects the flag owned by
// the participant-permission action so the two operations stay independent.
func (o *ShareSetOptions) buildSharingHandler() *cmdutil.ActionHandler {
	return &cmdutil.ActionHandler{
		ApiCmd: cmdutil.ApiCmdRecordModifySharingConfig,
		FlagRules: []cmdutil.FlagRule{
			{Reject: []string{"participant-perm"}},
		},
		Run: o.runSharing,
	}
}

// buildParticipantPermissionHandler
func (o *ShareSetOptions) buildParticipantPermissionHandler() *cmdutil.ActionHandler {
	return &cmdutil.ActionHandler{
		ApiCmd: cmdutil.ApiCmdRecordSetParticipantPermission,
		FlagRules: []cmdutil.FlagRule{
			{Reject: sharingOnlyFlags},
		},
		Validate: func(cmd *cobra.Command) error {
			if !cmd.Flags().Changed("participant-perm") {
				return exception.InvalidArgsError.With(
					"--action participant-permission requires --participant-perm")
			}
			if o.ParticipantPerm < 1 || o.ParticipantPerm > 5 {
				return exception.InvalidArgsError.With(
					"invalid --participant-perm %d, expected one of: 1, 2, 3, 4, 5", o.ParticipantPerm)
			}
			return nil
		},
		Run: o.runParticipantPermission,
	}
}

// runParticipantPermission viewing permission of meeting participants and invitees.
func (o *ShareSetOptions) runParticipantPermission(cmd *cobra.Command) error {
	body := map[string]interface{}{
		"operator_id":      o.tmeet.UserConfig.OpenId,
		"operator_id_type": 2, // OpenId
		"participant_perm": o.ParticipantPerm,
	}
	req := &thttp.Request{
		ApiURI: "/v1/mcp/records/participant-permission/{meeting_record_id}",
		Body:   body,
		PathParams: thttp.PathParams{
			"meeting_record_id": o.MeetingRecordID,
		},
	}
	rsp, err := restProxy.RequestProxy(cmd.Context(), http.MethodPost, o.tmeet, req)
	if err != nil {
		return err
	}
	output.FormatPrint(cmd, rsp.TraceId, rsp.Message, rsp.Data,
		output.WithCompact(middleWare.GetCompactFields(cmd.Context())))
	return nil
}

// commitResult records the outcome of a single downstream sub-operation.
type commitResult struct {
	Op      string `json:"op"`
	OK      bool   `json:"ok"`
	Message string `json:"message,omitempty"`
	TraceId string `json:"trace_id,omitempty"`
	Data    string `json:"data,omitempty"`
	Err     string `json:"error,omitempty"`
}

// runSharing executes the legacy `sharing` action, dispatching to the
// downstream APIs that are relevant for this invocation, then aggregates the
// results so that partial success / partial failure is reported clearly.
func (o *ShareSetOptions) runSharing(cmd *cobra.Command) error {
	results := make([]commitResult, 0, 3)

	if o.hasPermissionConfig(cmd) {
		results = append(results, o.commitPermissionConfig(cmd))
	}
	if len(o.AddMembers) > 0 {
		results = append(results, o.commitAddMembers(cmd))
	}
	if len(o.RemoveMembers) > 0 {
		results = append(results, o.commitRemoveMembers(cmd))
	}

	if len(results) == 0 {
		return fmt.Errorf("nothing to commit: provide at least one of sharing-config flags, --add-members or --remove-members")
	}

	summary := map[string]interface{}{
		"results": results,
	}
	var failed int
	for _, r := range results {
		if !r.OK {
			failed++
		}
	}
	summary["succeeded"] = len(results) - failed
	summary["failed"] = failed

	b, err := json.Marshal(summary)
	if err != nil {
		b = []byte("{}")
	}

	// Use the trace id of the first failed result (or first result) as the
	// top-level trace id for reference.
	traceID := ""
	message := "success"
	if failed > 0 {
		message = fmt.Sprintf("%d of %d sub-operation(s) failed", failed, len(results))
		for _, r := range results {
			if !r.OK {
				traceID = r.TraceId
				break
			}
		}
	} else if len(results) > 0 {
		traceID = results[0].TraceId
	}

	output.FormatPrint(cmd, traceID, message, string(b),
		output.WithCompact(middleWare.GetCompactFields(cmd.Context())))

	if failed > 0 {
		return fmt.Errorf("%s", message)
	}
	return nil
}

// hasPermissionConfig reports whether the user explicitly set any permission-config flag.
func (o *ShareSetOptions) hasPermissionConfig(cmd *cobra.Command) bool {
	for _, f := range []string{
		"switch-state", "permission-point-id", "pwd",
		"expired-time-type", "expired-time",
		"download-enable", "view-minutes-enable", "rooms-enable",
	} {
		if cmd.Flags().Changed(f) {
			return true
		}
	}
	return false
}

// commitPermissionConfig calls record-SetRecordPermissionConfigMcp with only
// the flags the user explicitly set.
func (o *ShareSetOptions) commitPermissionConfig(cmd *cobra.Command) commitResult {
	// Mutex check: switch_state=false must not be combined with permission_point_id.
	if cmd.Flags().Changed("switch-state") && !o.SwitchState && cmd.Flags().Changed("permission-point-id") {
		return commitResult{
			Op: "settings", OK: false,
			Err: "--switch-state=false must not be combined with --permission-point-id",
		}
	}

	body := map[string]interface{}{
		"operator_id":      o.tmeet.UserConfig.OpenId,
		"operator_id_type": 2,
	}
	if cmd.Flags().Changed("switch-state") {
		body["switch_state"] = o.SwitchState
	}
	if cmd.Flags().Changed("permission-point-id") {
		body["permission_point_id"] = o.PermissionPointID
	}
	if cmd.Flags().Changed("pwd") {
		body["pwd"] = o.Pwd
	}
	if cmd.Flags().Changed("expired-time-type") {
		body["expired_time_type"] = o.ExpiredTimeType
	}
	if cmd.Flags().Changed("expired-time") {
		expireTs, err := utils.ISO8601ToTimeStamp(o.ExpiredTime)
		if err != nil {
			return commitResult{Op: "settings", OK: false, Err: fmt.Sprintf("--expired-time format error: %v", err)}
		}
		body["expired_time"] = expireTs
	}
	if cmd.Flags().Changed("download-enable") {
		body["download_enable"] = o.DownloadEnable
	}
	if cmd.Flags().Changed("view-minutes-enable") {
		body["view_minutes_enable"] = o.ViewMinutesEnable
	}
	if cmd.Flags().Changed("rooms-enable") {
		body["rooms_enable"] = o.RoomsEnable
	}

	req := &thttp.Request{
		ApiURI: "/v1/mcp/records/permission/{meeting_record_id}",
		Body:   body,
		PathParams: thttp.PathParams{
			"meeting_record_id": o.MeetingRecordID,
		},
	}
	return o.doSub(cmd, "settings", cmdutil.ApiCmdRecordModifySharingConfig, http.MethodPost, req)
}

// commitAddMembers builds the access_members payload and calls the batch add grant users API.
func (o *ShareSetOptions) commitAddMembers(cmd *cobra.Command) commitResult {
	members, err := buildAddMembers(o.AddMembers)
	if err != nil {
		return commitResult{Op: "add", OK: false, Err: err.Error()}
	}
	body := map[string]interface{}{
		"operator_id":      o.tmeet.UserConfig.OpenId,
		"operator_id_type": 2,
		"access_members":   members,
	}
	req := &thttp.Request{
		ApiURI: "/v1/mcp/records/access/{meeting_record_id}",
		Body:   body,
		PathParams: thttp.PathParams{
			"meeting_record_id": o.MeetingRecordID,
		},
	}
	return o.doSub(cmd, "add", cmdutil.ApiCmdRecordBatchAddGrantUsers, http.MethodPost, req)
}

// commitRemoveMembers builds the access_members payload and calls the batch remove grant users API.
func (o *ShareSetOptions) commitRemoveMembers(cmd *cobra.Command) commitResult {
	members, err := buildRemoveMembers(o.RemoveMembers)
	if err != nil {
		return commitResult{Op: "remove", OK: false, Err: err.Error()}
	}
	body := map[string]interface{}{
		"operator_id":      o.tmeet.UserConfig.OpenId,
		"operator_id_type": 2,
		"access_members":   members,
	}
	req := &thttp.Request{
		ApiURI: "/v1/mcp/records/access/remove/{meeting_record_id}",
		Body:   body,
		PathParams: thttp.PathParams{
			"meeting_record_id": o.MeetingRecordID,
		},
	}
	return o.doSub(cmd, "remove", cmdutil.ApiCmdRecordBatchRemGrantUsers, http.MethodPost, req)
}

// doSub issues a single downstream request with the correct apiCmd injected
// into the context and records the outcome.
func (o *ShareSetOptions) doSub(cmd *cobra.Command, op, apiCmd, method string, req *thttp.Request) commitResult {
	ctx := apicmdctx.Inject(cmd.Context(), apiCmd)
	rsp, err := restProxy.RequestProxy(ctx, method, o.tmeet, req)
	if err != nil {
		return commitResult{Op: op, OK: false, Err: err.Error()}
	}
	return commitResult{
		Op:      op,
		OK:      true,
		Message: rsp.Message,
		TraceId: rsp.TraceId,
		Data:    strings.TrimSpace(rsp.Data),
	}
}

// buildAddMembers parses --add-members flag values (format: "openid:permission") into
// access_members objects. permission is optional and defaults to 0 (view).
func buildAddMembers(entries []string) ([]map[string]interface{}, error) {
	if len(entries) == 0 {
		return nil, fmt.Errorf("at least one --add-members is required")
	}
	members := make([]map[string]interface{}, 0, len(entries))
	for i, entry := range entries {
		if entry == "" {
			return nil, fmt.Errorf("--add-members[%d] must not be empty", i)
		}
		id, perm, err := parseIDPermission(entry)
		if err != nil {
			return nil, fmt.Errorf("--add-members[%d] %w", i, err)
		}
		members = append(members, map[string]interface{}{
			"to_operator_id":      id,
			"to_operator_id_type": 2, // OpenId
			"permission":          perm,
		})
	}
	return members, nil
}

// buildRemoveMembers parses --remove-members flag values (format: "openid") into
// access_members objects.
func buildRemoveMembers(entries []string) ([]map[string]interface{}, error) {
	if len(entries) == 0 {
		return nil, fmt.Errorf("at least one --remove-members is required")
	}
	members := make([]map[string]interface{}, 0, len(entries))
	for i, id := range entries {
		if id == "" {
			return nil, fmt.Errorf("--remove-members[%d] must not be empty", i)
		}
		members = append(members, map[string]interface{}{
			"to_operator_id":      id,
			"to_operator_id_type": 2, // OpenId
		})
	}
	return members, nil
}

// parseIDPermission parses a "openid:permission" string.
// permission part is optional and defaults to 0.
func parseIDPermission(entry string) (id string, permission int, err error) {
	parts := strings.SplitN(entry, ":", 2)
	id = parts[0]
	if id == "" {
		return "", 0, fmt.Errorf("openid must not be empty")
	}
	if len(parts) == 1 {
		return id, 0, nil
	}
	_, err = fmt.Sscanf(parts[1], "%d", &permission)
	if err != nil {
		return "", 0, fmt.Errorf("invalid permission %q, expected integer (0=view 1=manage)", parts[1])
	}
	return id, permission, nil
}
