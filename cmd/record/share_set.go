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
	"tmeet/internal/output"
	restProxy "tmeet/internal/proxy/rest-proxy"
	"tmeet/internal/utils"

	"github.com/spf13/cobra"
)

// ShareSetOptions holds the options for committing sharing changes. A single
// invocation may combine a permission-point update, batch add of members and
// batch remove of members; the backend dispatches to the relevant downstream
// APIs as needed.
type ShareSetOptions struct {
	tmeet *internal.Tmeet

	MeetingRecordID string // Meeting record ID (required)

	// permission config fields
	SwitchState       bool
	PermissionPointID int
	Pwd               string
	ExpiredTimeType   int
	ExpiredTime       string // ISO 8601, e.g. 2026-03-12T14:00+08:00; only when ExpiredTimeType == 4
	DownloadEnable    bool   // whether viewers can download; ignored when corp locks download
	ViewMinutesEnable bool   // whether viewers can view transcripts and minutes
	RoomsEnable       bool   // whether Rooms devices can view this recording

	// add_members (batch add grant users)
	// format: "openid:permission" (permission optional, default 0; 0=view 1=manage)
	AddMembers []string

	// remove_members (batch remove grant users)
	// format: "openid"
	RemoveMembers []string
}

// newShareSetCmd commits sharing settings and member changes in one call.
func newShareSetCmd(tmeet *internal.Tmeet) *cobra.Command {
	opts := &ShareSetOptions{tmeet: tmeet}
	cmd := &cobra.Command{
		Use:   "share-set",
		Short: "set record sharing config and member changes (write operation, confirm before executing)",
		RunE: middleWare.Chain(
			opts.Run,
			middleWare.WithApiCmd(cmdutil.StaticApiCmd(cmdutil.ApiCmdRecordModifySharingConfig)),
		),
	}

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
		"whether viewers can view transcripts and smart minutes")
	cmd.Flags().BoolVar(&opts.RoomsEnable, "rooms-enable", false,
		"whether Rooms devices can view this recording")

	// add_members flags
	cmd.Flags().StringArrayVar(&opts.AddMembers, "add-members", nil,
		"members to add, format: \"openid:permission\" (permission: 0=view 1=manage, default 0), repeatable; "+
			"e.g. --add-members openid_a:0 --add-members openid_b:1")

	// remove_members flags
	cmd.Flags().StringArrayVar(&opts.RemoveMembers, "remove-members", nil,
		"members to remove, format: \"openid\", repeatable; e.g. --remove-members openid_a --remove-members openid_b")

	_ = cmd.MarkFlagRequired("meeting-record-id")

	return cmd
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

// Run executes the share-set command, dispatching to the downstream APIs
// that are relevant for this invocation, then aggregates the results so that
// partial success / partial failure is reported clearly.
func (o *ShareSetOptions) Run(cmd *cobra.Command, args []string) error {
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
