package record

import (
	"net/http"

	"tmeet/internal"
	"tmeet/internal/cmdutil"
	middleWare "tmeet/internal/cmdutil/middleware"
	"tmeet/internal/core/thttp"
	"tmeet/internal/exception"
	"tmeet/internal/output"
	restProxy "tmeet/internal/proxy/rest-proxy"

	"github.com/spf13/cobra"
)

// Permission-apply sub-action values.
const (
	permissionApplyActionPrepare = "prepare"
	permissionApplyActionCommit  = "commit"
)

// PermissionApplyOptions holds the options for `tmeet record permission-apply`.
// The command defaults to previewing the application (--action prepare) and
// only actually submits when --action commit is passed.
type PermissionApplyOptions struct {
	tmeet *internal.Tmeet

	Action string // prepare (default) | commit

	MeetingRecordID string
	MeetingID       string
}

// newPermissionApplyCmd builds the `tmeet record permission-apply` command,
// which merges the following legacy commands via --action:
//   - permission-apply-prepare -> --action prepare (default)
//   - permission-apply-commit  -> --action commit  (write op, confirm before executing)
func newPermissionApplyCmd(tmeet *internal.Tmeet) *cobra.Command {
	opts := &PermissionApplyOptions{tmeet: tmeet}
	cmd := &cobra.Command{
		Use: "permission-apply",
		Short: "apply for record permission (default --action prepare previews; " +
			"--action commit submits, write operation, confirm before executing)",
		Long: "Apply for record permission. Both actions share the same flags.\n" +
			"\n" +
			"Actions:\n" +
			"  --action prepare (default) - preview the application (read-only)\n" +
			"  --action commit            - actually submit (write operation, confirm before executing)\n" +
			"\n" +
			"Required flags: <--meeting-record-id>\n" +
			"Optional flags: [--meeting-id]",
		RunE: middleWare.Chain(
			opts.Run,
			middleWare.WithApiCmd(cmdutil.ResolverFunc(opts.resolveApiCmd)),
			middleWare.WithCompact(tmeet),
		),
	}

	cmdutil.SingleStringVar(cmd.Flags(), &opts.Action, "action", permissionApplyActionPrepare,
		"action: prepare (preview, default) | commit (submit; write operation)")
	cmd.Flags().StringVar(&opts.MeetingRecordID, "meeting-record-id", "",
		"meeting record id (required)")
	cmd.Flags().StringVar(&opts.MeetingID, "meeting-id", "", "meeting id")

	_ = cmd.MarkFlagRequired("meeting-record-id")

	return cmd
}

// resolveApiCmd routes the ApiCmd according to --action.
func (o *PermissionApplyOptions) resolveApiCmd(cmd *cobra.Command) string {
	switch o.Action {
	case permissionApplyActionPrepare:
		return cmdutil.ApiCmdRecordPermissionApplyPrepare
	case permissionApplyActionCommit:
		return cmdutil.ApiCmdRecordPermissionApplyCommit
	}
	return ""
}

// Run dispatches to the prepare / commit sub-runner.
func (o *PermissionApplyOptions) Run(cmd *cobra.Command, args []string) error {
	switch o.Action {
	case permissionApplyActionPrepare:
		return o.run(cmd, "/v1/records/mcp/permission-apply/prepare")
	case permissionApplyActionCommit:
		return o.run(cmd, "/v1/records/mcp/permission-apply/commit")
	default:
		return exception.InvalidArgsError.With(
			"invalid --action %q, expected one of: prepare, commit", o.Action)
	}
}

// run issues the underlying request for the given URI (prepare / commit share
// exactly the same request shape).
func (o *PermissionApplyOptions) run(cmd *cobra.Command, apiURI string) error {
	queryParams := thttp.QueryParams{}
	queryParams.Set("operator_id", o.tmeet.UserConfig.OpenId)
	queryParams.Set("operator_id_type", "2") // OpenId
	queryParams.Set("meeting_record_id", o.MeetingRecordID)
	if o.MeetingID != "" {
		queryParams.Set("meeting_id", o.MeetingID)
	}

	req := &thttp.Request{
		ApiURI:      apiURI,
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
