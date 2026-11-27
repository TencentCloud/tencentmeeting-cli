package record

import (
	"net/http"

	"tmeet/internal/cmdutil"
	middleWare "tmeet/internal/cmdutil/middleware"
	"tmeet/internal/core/thttp"
	"tmeet/internal/exception"
	"tmeet/internal/output"
	restProxy "tmeet/internal/proxy/rest-proxy"
	"tmeet/internal/utils"

	"github.com/spf13/cobra"
)

// buildParticipantPermissionHandler declares the `--action participant-permission`
// It queries the viewing permission granted to meeting participants and invitees.
// Only the recording creator may query it; that is enforced by the backend and
// surfaced in --help and the skill reference.
func (o *AccessOptions) buildParticipantPermissionHandler() *cmdutil.ActionHandler {
	return &cmdutil.ActionHandler{
		ApiCmd: cmdutil.ApiCmdRecordGetParticipantPermission,
		FlagRules: []cmdutil.FlagRule{
			{Reject: []string{"record-file-id", "page-token", "page-size"}},
		},
		Validate: func(cmd *cobra.Command) error {
			if o.MeetingRecordID == "" {
				return exception.InvalidArgsError.With(
					"--action participant-permission requires --meeting-record-id")
			}
			return nil
		},
		Run: o.runParticipantPermission,
	}
}

// runParticipantPermission
func (o *AccessOptions) runParticipantPermission(cmd *cobra.Command) error {
	queryParams := thttp.QueryParams{}
	queryParams.Set("operator_id", o.tmeet.UserConfig.OpenId)
	queryParams.Set("operator_id_type", "2") // OpenId

	req := &thttp.Request{
		ApiURI:      "/v1/mcp/records/participant-permission/{meeting_record_id}",
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
		output.WithConvert(map[string]utils.FieldConverter{
			"participant_perm": utils.RecordParticipantPermConverter,
		}))
	return nil
}
