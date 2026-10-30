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

// buildTranscriptHandler declares the `--action transcript` branch.
//
// This branch fans out into three sub-forms selected by which exclusive flag
// is present:
//
//	--share-url  present  -> transcript(url):     list-by-share-url (POST, paginated)
//	--text       present  -> transcript(search):  in-file search    (GET)
//	otherwise             -> transcript(details): transcript details (GET, with --pid / --limit)
//
// Flag contract is declared via FlagRules (see ActionHandler.FlagRules):
// one unconditional rule rejects --lang / --pwd for all transcript forms,
// and three mutually-exclusive conditional rules reject the flags that
// don't belong to the chosen sub-form. The three When conditions partition
// the valid --share-url / --text combinations and are therefore mutually
// exclusive by construction.
//
// Validate only enforces the required / mutually-exclusive combinations of
// --record-file-id, --share-url and --text.
//
// All transcript-related changes should stay in this file.
func (o *ContentOptions) buildTranscriptHandler() *cmdutil.ActionHandler {
	return &cmdutil.ActionHandler{
		ResolveApiCmd: func(cmd *cobra.Command) string {
			if o.ShareURL != "" {
				return cmdutil.ApiCmdRecordTranscriptListByUrl
			}
			if o.Text != "" {
				return cmdutil.ApiCmdRecordTranscriptSearch
			}
			return cmdutil.ApiCmdRecordTranscriptGet
		},
		FlagRules: []cmdutil.FlagRule{
			// Action-level: never accepted by any transcript form.
			{Reject: []string{"lang", "pwd"}},
			// Form-level: transcript(url) accepts --share-url, --page-token,
			// --page-size.
			{
				When:   func(cmd *cobra.Command) bool { return o.ShareURL != "" },
				Form:   "transcript(url)",
				Reject: []string{"meeting-id", "pid", "limit"},
			},
			// Form-level: transcript(search) accepts --record-file-id,
			// --text, --meeting-id.
			{
				When:   func(cmd *cobra.Command) bool { return o.ShareURL == "" && o.Text != "" },
				Form:   "transcript(search)",
				Reject: []string{"pid", "limit", "page-token", "page-size"},
			},
			// Form-level: transcript(details) accepts --record-file-id,
			// --meeting-id, --pid, --limit.
			{
				When:   func(cmd *cobra.Command) bool { return o.ShareURL == "" && o.Text == "" },
				Form:   "transcript(details)",
				Reject: []string{"page-token", "page-size"},
			},
		},
		Validate: func(cmd *cobra.Command) error {
			if o.RecordFileID == "" && o.ShareURL == "" {
				return exception.InvalidArgsError.With(
					"--action transcript requires one of --record-file-id or --share-url")
			}
			if o.ShareURL != "" && o.Text != "" {
				return exception.InvalidArgsError.With(
					"--share-url and --text must not be used together for --action transcript")
			}
			if o.RecordFileID != "" && o.ShareURL != "" {
				return exception.InvalidArgsError.With(
					"--record-file-id and --share-url must not be used together for --action transcript")
			}
			return nil
		},
		Run: func(cmd *cobra.Command) error {
			switch {
			case o.ShareURL != "":
				return o.runTranscriptListByUrl(cmd)
			case o.Text != "":
				return o.runTranscriptSearch(cmd)
			default:
				return o.runTranscriptGet(cmd)
			}
		},
	}
}

// runTranscriptGet corresponds to legacy `transcript-get`.
func (o *ContentOptions) runTranscriptGet(cmd *cobra.Command) error {
	queryParams := thttp.QueryParams{}
	queryParams.Set("operator_id", o.tmeet.UserConfig.OpenId)
	queryParams.Set("operator_id_type", "2") // OpenId
	queryParams.Set("record_file_id", o.RecordFileID)
	if o.MeetingID != "" {
		queryParams.Set("meeting_id", o.MeetingID)
	}
	if o.Pid != "" {
		queryParams.Set("pid", o.Pid)
	}
	if o.Limit != "" {
		queryParams.Set("limit", o.Limit)
	}

	req := &thttp.Request{
		ApiURI:      "/v1/records/mcp/transcripts/details",
		QueryParams: queryParams,
	}
	rsp, err := restProxy.RequestProxy(cmd.Context(), http.MethodGet, o.tmeet, req)
	if err != nil {
		return err
	}

	convertMap := map[string]utils.FieldConverter{
		"start_time": utils.HHMMSSConverter,
		"end_time":   utils.HHMMSSConverter,
	}
	output.FormatPrint(cmd, rsp.TraceId, rsp.Message, rsp.Data,
		output.WithCompact(middleWare.GetCompactFields(cmd.Context())),
		output.WithConvert(convertMap))
	return nil
}

// runTranscriptListByUrl corresponds to legacy `transcript-list`.
func (o *ContentOptions) runTranscriptListByUrl(cmd *cobra.Command) error {
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

	req := &thttp.Request{
		ApiURI: "/v1/mcp/records/transcripts/list-by-url",
		Body:   body,
	}
	rsp, err := restProxy.RequestProxy(cmd.Context(), http.MethodPost, o.tmeet, req)
	if err != nil {
		return err
	}

	convertMap := map[string]utils.FieldConverter{
		"transcripts_type": utils.TranscriptTypeConverter,
		"audio_detect":     utils.RecordAudioDetectConverter,
		"start_time":       utils.HHMMSSConverter,
		"end_time":         utils.HHMMSSConverter,
	}
	output.FormatPrint(cmd, rsp.TraceId, rsp.Message, rsp.Data,
		output.WithCompact(middleWare.GetCompactFields(cmd.Context())),
		output.WithConvert(convertMap))
	return nil
}

// runTranscriptSearch corresponds to legacy `transcript-search`.
func (o *ContentOptions) runTranscriptSearch(cmd *cobra.Command) error {
	queryParams := thttp.QueryParams{}
	queryParams.Set("operator_id", o.tmeet.UserConfig.OpenId)
	queryParams.Set("operator_id_type", "2") // OpenId
	queryParams.Set("record_file_id", o.RecordFileID)
	queryParams.Set("text", o.Text)
	if o.MeetingID != "" {
		queryParams.Set("meeting_id", o.MeetingID)
	}

	req := &thttp.Request{
		ApiURI:      "/v1/records/mcp/transcripts/search",
		QueryParams: queryParams,
	}
	rsp, err := restProxy.RequestProxy(cmd.Context(), http.MethodGet, o.tmeet, req)
	if err != nil {
		return err
	}

	convertMap := map[string]utils.FieldConverter{
		"start_time": utils.HHMMSSConverter,
	}
	output.FormatPrint(cmd, rsp.TraceId, rsp.Message, rsp.Data,
		output.WithCompact(middleWare.GetCompactFields(cmd.Context())),
		output.WithConvert(convertMap))
	return nil
}
