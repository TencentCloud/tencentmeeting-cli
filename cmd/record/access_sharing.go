package record

import (
	"encoding/json"
	"net/http"
	"tmeet/internal/utils"

	"tmeet/internal/cmdutil"
	middleWare "tmeet/internal/cmdutil/middleware"
	"tmeet/internal/core/thttp"
	"tmeet/internal/exception"
	"tmeet/internal/output"
	restProxy "tmeet/internal/proxy/rest-proxy"

	"github.com/spf13/cobra"
)

// buildSharingHandler declares the `--action sharing` branch, corresponding
// to legacy `share-list --list-type config`. Returns the sharing settings
// (switches / password / expiration / download / etc.) of a recording.
func (o *AccessOptions) buildSharingHandler() *cmdutil.ActionHandler {
	return &cmdutil.ActionHandler{
		ApiCmd: cmdutil.ApiCmdRecordQuerySharingConfig,
		FlagRules: []cmdutil.FlagRule{
			{Reject: []string{"record-file-id", "page-token", "page-size"}},
		},
		Validate: func(cmd *cobra.Command) error {
			if o.MeetingRecordID == "" {
				return exception.InvalidArgsError.With("--action sharing requires --meeting-record-id")
			}
			return nil
		},
		Run: o.runSharing,
	}
}

// runSharing queries the record permission-point configuration
func (o *AccessOptions) runSharing(cmd *cobra.Command) error {
	queryParams := thttp.QueryParams{}
	queryParams.Set("operator_id", o.tmeet.UserConfig.OpenId)
	queryParams.Set("operator_id_type", "2")

	req := &thttp.Request{
		ApiURI:      "/v1/mcp/records/permission/{meeting_record_id}",
		QueryParams: queryParams,
		PathParams: thttp.PathParams{
			"meeting_record_id": o.MeetingRecordID,
		},
	}
	rsp, err := restProxy.RequestProxy(cmd.Context(), http.MethodGet, o.tmeet, req)
	if err != nil {
		return err
	}
	output.FormatPrint(cmd, rsp.TraceId, rsp.Message, rsp.Data,
		output.WithCompact(middleWare.GetCompactFields(cmd.Context())),
		output.WithFnFilterFunc(sharingFilterFields),
		output.WithConvert(map[string]utils.FieldConverter{
			"permission_point_id": utils.RecordPermissionPointConverter,
			"expired_time_type":   utils.RecordExpiredTimeTypeConverter,
			"corp_share_scope":    utils.RecordCorpShareScopeConverter,
			"expired_time":        utils.TimestampConverter,
		}))
	return nil
}

// sharingFilterFields decides which response fields should be stripped before
// rendering. When current.switch_state is false, expired_time_type is
// meaningless because sharing is disabled; strip it to avoid confusing the
// caller.
func sharingFilterFields(data string) []string {
	var root struct {
		Current struct {
			SwitchState *bool `json:"switch_state"`
		} `json:"current"`
	}
	if err := json.Unmarshal([]byte(data), &root); err != nil {
		return nil
	}
	if root.Current.SwitchState != nil && !*root.Current.SwitchState {
		return []string{"expired_time_type", "expired_time"}
	}
	return nil
}
