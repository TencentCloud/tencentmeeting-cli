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

// PermissionApplyPrepareOptions holds the options for previewing a record permission application.
type PermissionApplyPrepareOptions struct {
	tmeet           *internal.Tmeet
	MeetingID       string // Meeting ID
	MeetingRecordID string // Meeting record ID
}

// newPermissionApplyPrepareCmd previews the record permission application before commit.
func newPermissionApplyPrepareCmd(tmeet *internal.Tmeet) *cobra.Command {
	opts := &PermissionApplyPrepareOptions{tmeet: tmeet}
	cmd := &cobra.Command{
		Use:   "permission-apply-prepare",
		Short: "preview record permission application before commit",
		RunE: middleWare.Chain(
			opts.Run,
			middleWare.WithApiCmd(cmdutil.StaticApiCmd(cmdutil.ApiCmdRecordPermissionApplyPrepare)),
			middleWare.WithCompact(tmeet),
		),
	}

	cmd.Flags().StringVar(&opts.MeetingRecordID, "meeting-record-id", "", "meeting record id (required)")
	cmd.Flags().StringVar(&opts.MeetingID, "meeting-id", "", "meeting id")

	_ = cmd.MarkFlagRequired("meeting-record-id")

	return cmd
}

// Run run cmd
func (o *PermissionApplyPrepareOptions) Run(cmd *cobra.Command, args []string) error {
	queryParams := thttp.QueryParams{}
	queryParams.Set("operator_id", o.tmeet.UserConfig.OpenId)
	queryParams.Set("operator_id_type", "2") // OpenId
	queryParams.Set("meeting_record_id", o.MeetingRecordID)
	if o.MeetingID != "" {
		queryParams.Set("meeting_id", o.MeetingID)
	}

	req := &thttp.Request{
		ApiURI:      "/v1/records/mcp/permission-apply/prepare",
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
