package record

import (
	"net/http"
	"tmeet/internal"
	"tmeet/internal/core/thttp"
	"tmeet/internal/output"
	restProxy "tmeet/internal/proxy/rest-proxy"
	"tmeet/internal/utils"

	"github.com/spf13/cobra"
)

// TranscriptsGetOptions holds the options for transcript-related commands (details, paragraphs, search).
type TranscriptsGetOptions struct {
	tmeet        *internal.Tmeet
	RecordFileID string // Record file ID
	MeetingID    string // Meeting ID
	Pid          string // Starting paragraph ID for query
	Limit        string // Number of paragraphs to query
}

// newTranscriptGetCmd gets transcript details.
func newTranscriptGetCmd(tmeet *internal.Tmeet) *cobra.Command {
	opts := &TranscriptsGetOptions{tmeet: tmeet}
	cmd := &cobra.Command{
		Use:   "transcript-get",
		Short: "get transcript detail",
		RunE: func(cmd *cobra.Command, args []string) error {
			return opts.Run(cmd, args)
		},
	}

	cmd.Flags().StringVar(&opts.RecordFileID, "record-file-id", "", "record file id (required)")
	cmd.Flags().StringVar(&opts.MeetingID, "meeting-id", "", "meeting id")
	cmd.Flags().StringVar(&opts.Pid, "pid", "", "start paragraph id")
	cmd.Flags().StringVar(&opts.Limit, "limit", "", "number of paragraphs to query")

	_ = cmd.MarkFlagRequired("record-file-id")

	return cmd
}

func (o *TranscriptsGetOptions) Run(cmd *cobra.Command, args []string) error {
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

	// Parse response.
	rsp.Data = string(utils.ConvertFields([]byte(rsp.Data), 10, map[string]utils.FieldConverter{
		"start_time": utils.HHMMSSConverter,
		"end_time":   utils.HHMMSSConverter,
	}))
	output.FormatPrint(cmd, rsp.TraceId, rsp.Message, rsp.Data)
	return nil
}
