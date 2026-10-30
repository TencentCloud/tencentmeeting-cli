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

// buildMinutesHandler declares the `--action minutes` branch.
//
// This branch has two sub-forms selected implicitly by --share-url:
//   - present -> minutes(url): smart-minutes-list (list-by-share-url, POST)
//   - absent  -> minutes(file): smart-minutes (single-file, GET)
//
// Flag contract is declared via FlagRules (see ActionHandler.FlagRules):
// one unconditional rule rejects the flags that no minutes form accepts
// (--meeting-id / --pid / --limit / --text), and one conditional rule
// further rejects the list-by-url pagination flags under minutes(file).
// Validate only enforces the required / mutually-exclusive combinations of
// --record-file-id and --share-url.
//
// All minutes-related changes should stay in this file; the parent command
// shell only references buildMinutesHandler.
func (o *ContentOptions) buildMinutesHandler() *cmdutil.ActionHandler {
	return &cmdutil.ActionHandler{
		ResolveApiCmd: func(cmd *cobra.Command) string {
			if o.ShareURL != "" {
				return cmdutil.ApiCmdRecordSmartMinutesListByUrl
			}
			return cmdutil.ApiCmdRecordSmartMinutes
		},
		FlagRules: []cmdutil.FlagRule{
			// Action-level: never accepted by any minutes form.
			{Reject: []string{"meeting-id", "pid", "limit", "text"}},
			// Form-level: minutes(file) rejects the list-by-url pagination
			// flags. minutes(url) itself needs no extra rejections.
			{
				When:   func(cmd *cobra.Command) bool { return o.ShareURL == "" },
				Form:   "minutes(file)",
				Reject: []string{"page-token", "page-size"},
			},
		},
		Validate: func(cmd *cobra.Command) error {
			if o.RecordFileID == "" && o.ShareURL == "" {
				return exception.InvalidArgsError.With(
					"--action minutes requires one of --record-file-id or --share-url")
			}
			if o.RecordFileID != "" && o.ShareURL != "" {
				return exception.InvalidArgsError.With(
					"--record-file-id and --share-url must not be used together for --action minutes")
			}
			return nil
		},
		Run: func(cmd *cobra.Command) error {
			if o.ShareURL != "" {
				return o.runMinutesListByUrl(cmd)
			}
			return o.runMinutesGet(cmd)
		},
	}
}

// runMinutesGet corresponds to legacy `smart-minutes` (single file, GET).
func (o *ContentOptions) runMinutesGet(cmd *cobra.Command) error {
	lang := o.Lang
	if lang == "" {
		lang = "default"
	}
	queryParams := thttp.QueryParams{}
	queryParams.Set("operator_id", o.tmeet.UserConfig.OpenId)
	queryParams.Set("operator_id_type", "2") // OpenId
	queryParams.Set("lang", lang)
	if o.Pwd != "" {
		queryParams.Set("pwd", o.Pwd)
	}

	req := &thttp.Request{
		ApiURI:      "/v1/smart/minutes/{record_file_id}",
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

// runMinutesListByUrl corresponds to legacy `smart-minutes-list` (list by share URL, POST).
func (o *ContentOptions) runMinutesListByUrl(cmd *cobra.Command) error {
	pageSize, err := cmdutil.ClampingPageSize(cmd, o.PageSize, 5)
	if err != nil {
		return err
	}

	body := map[string]interface{}{
		"operator_id":      o.tmeet.UserConfig.OpenId,
		"operator_id_type": 2, // OpenId
		"share_url":        o.ShareURL,
		"page_size":        pageSize,
	}
	if o.PageToken != "" {
		body["page_token"] = o.PageToken
	}
	if o.Lang != "" {
		body["lang"] = o.Lang
	}
	if o.Pwd != "" {
		body["pwd"] = o.Pwd
	}

	req := &thttp.Request{
		ApiURI: "/v1/mcp/smart/minutes/list-by-url",
		Body:   body,
	}
	rsp, err := restProxy.RequestProxy(cmd.Context(), http.MethodPost, o.tmeet, req)
	if err != nil {
		return err
	}

	convertMap := map[string]utils.FieldConverter{
		"minute_type": utils.SmartMinutesMinuteTypeConverter,
		"text_type":   utils.SmartMinutesTextTypeConverter,
		"industry":    utils.SmartMinutesIndustryConverter,
		"llm":         utils.SmartMinutesLLMConverter,
	}
	output.FormatPrint(cmd, rsp.TraceId, rsp.Message, rsp.Data,
		output.WithCompact(middleWare.GetCompactFields(cmd.Context())),
		output.WithConvert(convertMap))
	return nil
}
