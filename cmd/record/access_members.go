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

// buildMembersHandler declares the `--action members` branch, corresponding
// to legacy `share-list --list-type users`. Returns the list of members
// authorized to view the recording (paginated by --page-token).
func (o *AccessOptions) buildMembersHandler() *cmdutil.ActionHandler {
	return &cmdutil.ActionHandler{
		ApiCmd: cmdutil.ApiCmdRecordGetGrantedUserList,
		FlagRules: []cmdutil.FlagRule{
			{Reject: []string{"record-file-id", "page-size"}},
		},
		Validate: func(cmd *cobra.Command) error {
			if o.MeetingRecordID == "" {
				return exception.InvalidArgsError.With("--action members requires --meeting-record-id")
			}
			return nil
		},
		Run: o.runMembers,
	}
}

// runMembers corresponds to legacy `share-list --list-type users`.
func (o *AccessOptions) runMembers(cmd *cobra.Command) error {
	queryParams := thttp.QueryParams{}
	queryParams.Set("operator_id", o.tmeet.UserConfig.OpenId)
	queryParams.Set("operator_id_type", "2")
	if o.PageToken != "" {
		queryParams.Set("page_token", o.PageToken)
	}

	req := &thttp.Request{
		ApiURI:      "/v1/mcp/records/access/{meeting_record_id}",
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
			"permission": utils.RecordMemberPermissionConverter,
		}),
		output.WithTotalCountLogic())
	return nil
}
