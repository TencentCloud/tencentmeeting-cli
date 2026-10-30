package record

import (
	"net/http"

	"tmeet/internal/cmdutil"
	middleWare "tmeet/internal/cmdutil/middleware"
	"tmeet/internal/core/thttp"
	"tmeet/internal/exception"
	"tmeet/internal/output"
	restProxy "tmeet/internal/proxy/rest-proxy"

	"github.com/spf13/cobra"
)

// buildTodosHandler declares the `--action todos` branch, corresponding to
// the legacy `todos` command. Returns structured smart todos (executor list
// with openId, deduplicated), typically used to fetch todos without the
// heavier full smart-minutes payload.
func (o *ContentOptions) buildTodosHandler() *cmdutil.ActionHandler {
	return &cmdutil.ActionHandler{
		ApiCmd: cmdutil.ApiCmdRecordSmartTodos,
		FlagRules: []cmdutil.FlagRule{
			{Reject: []string{
				"share-url", "meeting-id", "pid", "limit", "text",
				"page-token", "page-size",
			}},
		},
		Validate: func(cmd *cobra.Command) error {
			if o.RecordFileID == "" {
				return exception.InvalidArgsError.With("--action todos requires --record-file-id")
			}
			return nil
		},
		Run: o.runTodos,
	}
}

// runTodos corresponds to legacy `todos`.
func (o *ContentOptions) runTodos(cmd *cobra.Command) error {
	lang := o.Lang
	if lang == "" {
		lang = "default"
	}

	queryParams := thttp.QueryParams{}
	queryParams.Set("operator_id", o.tmeet.UserConfig.OpenId)
	queryParams.Set("operator_id_type", "2") // OpenId
	queryParams.Set("record_file_id", o.RecordFileID)
	queryParams.Set("lang", lang)
	if o.Pwd != "" {
		queryParams.Set("pwd", o.Pwd)
	}

	req := &thttp.Request{
		ApiURI:      "/v1/mcp/smart/todos/{record_file_id}",
		QueryParams: queryParams,
		PathParams: thttp.PathParams{
			"record_file_id": o.RecordFileID,
		},
	}
	rsp, err := restProxy.RequestProxy(cmd.Context(), http.MethodGet, o.tmeet, req)
	if err != nil {
		return err
	}
	output.FormatPrint(cmd, rsp.TraceId, rsp.Message, rsp.Data,
		output.WithCompact(middleWare.GetCompactFields(cmd.Context())))
	return nil
}
