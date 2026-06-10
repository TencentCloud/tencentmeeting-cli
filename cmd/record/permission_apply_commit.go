package record

import (
	"net/http"

	"tmeet/internal"
	"tmeet/internal/cmdutil"
	middleWare "tmeet/internal/cmdutil/middleware"
	"tmeet/internal/core/thttp"
	"tmeet/internal/output"
	restProxy "tmeet/internal/proxy/rest-proxy"

	"github.com/spf13/cobra"
)

// PermissionApplyCommitOptions holds the options for committing a record permission application.
type PermissionApplyCommitOptions struct {
	tmeet           *internal.Tmeet
	MeetingID       string // Meeting ID
	MeetingRecordID string // Meeting record ID
}

// newPermissionApplyCommitCmd commits the record permission application after user confirmation.
func newPermissionApplyCommitCmd(tmeet *internal.Tmeet) *cobra.Command {
	opts := &PermissionApplyCommitOptions{tmeet: tmeet}
	cmd := &cobra.Command{
		Use:   "permission-apply-commit",
		Short: "commit record permission application after user confirmation",
		RunE: middleWare.Chain(
			opts.Run,
			middleWare.WithApiCmd(cmdutil.StaticApiCmd(cmdutil.ApiCmdRecordPermissionApplyCommit)),
			middleWare.WithCompact(tmeet),
		),
	}

	cmd.Flags().StringVar(&opts.MeetingRecordID, "meeting-record-id", "", "meeting record id (required)")
	cmd.Flags().StringVar(&opts.MeetingID, "meeting-id", "", "meeting id")

	_ = cmd.MarkFlagRequired("meeting-record-id")

	return cmd
}

// Run run cmd
func (o *PermissionApplyCommitOptions) Run(cmd *cobra.Command, args []string) error {
	queryParams := thttp.QueryParams{}
	queryParams.Set("operator_id", o.tmeet.UserConfig.OpenId)
	queryParams.Set("operator_id_type", "2") // OpenId
	queryParams.Set("meeting_record_id", o.MeetingRecordID)
	if o.MeetingID != "" {
		queryParams.Set("meeting_id", o.MeetingID)
	}

	req := &thttp.Request{
		ApiURI:      "/v1/records/mcp/permission-apply/commit",
		QueryParams: queryParams,
	}
	rsp, err := restProxy.RequestProxy(cmd.Context(), http.MethodGet, o.tmeet, req)
	if err != nil {
		return err
	}

	output.FormatPrint(cmd, rsp.TraceId, rsp.Message, rsp.Data,
		output.WithCompact(middleWare.GetCompactFields(cmd.Context())))
	return nil
}
