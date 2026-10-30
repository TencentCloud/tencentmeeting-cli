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

// buildTimelineHandler declares the `--action timeline` branch, corresponding
// to legacy `recording-timeline`. Returns the smart timeline of a record file
// (chapter-like structure with status / start_time).
func (o *ContentOptions) buildTimelineHandler() *cmdutil.ActionHandler {
	return &cmdutil.ActionHandler{
		ApiCmd: cmdutil.ApiCmdRecordRecordingTimeline,
		FlagRules: []cmdutil.FlagRule{
			{Reject: []string{
				"share-url", "pwd", "meeting-id", "pid", "limit", "text",
				"page-token", "page-size",
			}},
		},
		Validate: func(cmd *cobra.Command) error {
			if o.RecordFileID == "" {
				return exception.InvalidArgsError.With("--action timeline requires --record-file-id")
			}
			return nil
		},
		Run: o.runTimeline,
	}
}

// runTimeline corresponds to legacy `recording-timeline`.
func (o *ContentOptions) runTimeline(cmd *cobra.Command) error {
	queryParams := thttp.QueryParams{}
	queryParams.Set("operator_id", o.tmeet.UserConfig.OpenId)
	queryParams.Set("operator_id_type", "2") // OpenId
	queryParams.Set("record_file_id", o.RecordFileID)
	if o.Lang != "" {
		queryParams.Set("lang", o.Lang)
	}

	req := &thttp.Request{
		ApiURI:      "/v1/mcp/asr/get-recording-timeline-mcp",
		QueryParams: queryParams,
	}
	rsp, err := restProxy.RequestProxy(cmd.Context(), http.MethodGet, o.tmeet, req)
	if err != nil {
		return err
	}

	convertMap := map[string]utils.FieldConverter{
		"status":     utils.RecordingTimelineStatusConverter,
		"start_time": utils.DurationSecondsConverter,
	}
	output.FormatPrint(cmd, rsp.TraceId, rsp.Message, rsp.Data,
		output.WithCompact(middleWare.GetCompactFields(cmd.Context())),
		output.WithConvert(convertMap))
	return nil
}
