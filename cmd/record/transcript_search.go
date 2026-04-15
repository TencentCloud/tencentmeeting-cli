package record

import (
	"net/http"
	"tmeet/internal"
	"tmeet/internal/core/thttp"
	"tmeet/internal/log"
	restProxy "tmeet/internal/proxy/rest-proxy"
	"tmeet/internal/utils"

	"github.com/spf13/cobra"
)

// TranscriptsSearchOptions holds the options for transcript search.
type TranscriptsSearchOptions struct {
	tmeet        *internal.Tmeet
	RecordFileID string // Record file ID
	MeetingID    string // Meeting ID
	Text         string // Search text
}

// newTranscriptSearchCmd searches transcript content.
func newTranscriptSearchCmd(tmeet *internal.Tmeet) *cobra.Command {
	opts := &TranscriptsSearchOptions{tmeet: tmeet}
	cmd := &cobra.Command{
		Use:   "transcript-search",
		Short: "search transcript content",
		RunE: func(cmd *cobra.Command, args []string) error {
			return opts.Run(cmd, args)
		},
	}

	cmd.Flags().StringVar(&opts.RecordFileID, "record-file-id", "", "record file id (required)")
	cmd.Flags().StringVar(&opts.MeetingID, "meeting-id", "", "meeting id")
	cmd.Flags().StringVar(&opts.Text, "text", "", "search text (required)")

	_ = cmd.MarkFlagRequired("record-file-id")
	_ = cmd.MarkFlagRequired("text")

	return cmd
}

func (o *TranscriptsSearchOptions) Run(cmd *cobra.Command, args []string) error {
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

	// Parse response.
	rsp.Data = string(utils.ConvertFields([]byte(rsp.Data), 10, map[string]utils.FieldConverter{
		"start_time": utils.HHMMSSConverter,
	}))
	log.FormatPrint(cmd, rsp.TraceId, rsp.Message, rsp.Data)
	return nil
}
