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

// buildParagraphsHandler declares the `--action paragraphs` branch,
// corresponding to legacy `transcript-paragraphs`. Returns transcript
// paragraph list with speaker / start_time / end_time fields.
func (o *ContentOptions) buildParagraphsHandler() *cmdutil.ActionHandler {
	return &cmdutil.ActionHandler{
		ApiCmd: cmdutil.ApiCmdRecordTranscriptParagraphs,
		FlagRules: []cmdutil.FlagRule{
			{Reject: []string{
				"share-url", "lang", "pwd", "pid", "limit", "text",
				"page-token", "page-size",
			}},
		},
		Validate: func(cmd *cobra.Command) error {
			if o.RecordFileID == "" {
				return exception.InvalidArgsError.With("--action paragraphs requires --record-file-id")
			}
			return nil
		},
		Run: o.runParagraphs,
	}
}

// runParagraphs corresponds to legacy `transcript-paragraphs`.
func (o *ContentOptions) runParagraphs(cmd *cobra.Command) error {
	queryParams := thttp.QueryParams{}
	queryParams.Set("operator_id", o.tmeet.UserConfig.OpenId)
	queryParams.Set("operator_id_type", "2") // OpenId
	queryParams.Set("record_file_id", o.RecordFileID)
	if o.MeetingID != "" {
		queryParams.Set("meeting_id", o.MeetingID)
	}

	req := &thttp.Request{
		ApiURI:      "/v1/records/mcp/transcripts/paragraphs",
		QueryParams: queryParams,
	}
	rsp, err := restProxy.RequestProxy(cmd.Context(), http.MethodGet, o.tmeet, req)
	if err != nil {
		return err
	}

	convertMap := map[string]utils.FieldConverter{
		"start_time":   utils.HHMMSSConverter,
		"end_time":     utils.HHMMSSConverter,
		"audio_detect": utils.RecordAudioDetectConverter,
	}
	output.FormatPrint(cmd, rsp.TraceId, rsp.Message, rsp.Data,
		output.WithCompact(middleWare.GetCompactFields(cmd.Context())),
		output.WithConvert(convertMap))
	return nil
}
