package record

import (
	"net/http"
	"tmeet/internal"
	"tmeet/internal/cmdutil"
	middleWare "tmeet/internal/cmdutil/middleware"
	"tmeet/internal/core/thttp"
	"tmeet/internal/output"
	restProxy "tmeet/internal/proxy/rest-proxy"
	"tmeet/internal/utils"

	"github.com/spf13/cobra"
)

// TranscriptsParagraphsOptions holds the options for transcript paragraphs.
type TranscriptsParagraphsOptions struct {
	tmeet        *internal.Tmeet
	RecordFileID string // Record file ID
	MeetingID    string // Meeting ID
}

// newTranscriptParagraphsCmd gets transcript paragraphs.
func newTranscriptParagraphsCmd(tmeet *internal.Tmeet) *cobra.Command {
	opts := &TranscriptsParagraphsOptions{tmeet: tmeet}
	cmd := &cobra.Command{
		Use:   "transcript-paragraphs",
		Short: "get transcript paragraphs",
		RunE: middleWare.Chain(
			opts.Run,
			middleWare.WithApiCmd(cmdutil.StaticApiCmd(cmdutil.ApiCmdRecordTranscriptParagraphs)),
			middleWare.WithCompact(tmeet),
		),
	}

	cmd.Flags().StringVar(&opts.RecordFileID, "record-file-id", "", "record file id (required)")
	cmd.Flags().StringVar(&opts.MeetingID, "meeting-id", "", "meeting id")

	// mark required flags
	_ = cmd.MarkFlagRequired("record-file-id")

	return cmd
}

func (o *TranscriptsParagraphsOptions) Run(cmd *cobra.Command, args []string) error {
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
