package report

import (
	"net/http"
	"tmeet/internal"
	"tmeet/internal/cmdutil"
	middleWare "tmeet/internal/cmdutil/middleware"
	"tmeet/internal/core/thttp"
	"tmeet/internal/exception"
	"tmeet/internal/output"
	restProxy "tmeet/internal/proxy/rest-proxy"
	"tmeet/internal/utils"

	"github.com/spf13/cobra"
)

// ParticipantsExportOptions holds the options for participants export.
type ParticipantsExportOptions struct {
	tmeet        *internal.Tmeet
	MeetingId    string // Meeting ID
	SubMeetingId string // Sub-meeting ID for recurring meetings
	StartTime    string // Query start time, ISO 8601
	EndTime      string // Query end time, ISO 8601
	FileType     string // Export file format: xlsx or json
}

// newParticipantsExportCmd exports the participants list.
func newParticipantsExportCmd(tmeet *internal.Tmeet) *cobra.Command {
	opts := &ParticipantsExportOptions{tmeet: tmeet}
	cmd := &cobra.Command{
		Use:   "participants-export",
		Short: "export meeting participants list",
		RunE: middleWare.Chain(
			opts.Run,
			middleWare.WithApiCmd(cmdutil.StaticApiCmd(cmdutil.ApiCmdReportParticipantsExport)),
		),
	}

	cmd.Flags().StringVar(&opts.MeetingId, "meeting-id", "", "meeting id (required)")
	cmd.Flags().StringVar(&opts.SubMeetingId, "sub-meeting-id", "", "sub meeting id for recurring meeting")
	cmd.Flags().StringVar(&opts.StartTime, "start", "", "query start time (ISO 8601, e.g. 2026-05-22T14:00+08:00)")
	cmd.Flags().StringVar(&opts.EndTime, "end", "", "query end time (ISO 8601, e.g. 2026-05-22T14:00+08:00)")
	cmd.Flags().StringVar(&opts.FileType, "file-type", "xlsx", "export file format: xlsx (default) or json")

	// mark required flags
	_ = cmd.MarkFlagRequired("meeting-id")

	return cmd
}

func (o *ParticipantsExportOptions) Run(cmd *cobra.Command, args []string) error {
	params := map[string]interface{}{
		"operator_id":      o.tmeet.UserConfig.OpenId,
		"operator_id_type": 2, // OpenId
		"meeting_id":       o.MeetingId,
	}

	if o.FileType != "" {
		if o.FileType != "xlsx" && o.FileType != "json" {
			return exception.InvalidArgsError.With("--file-type must be xlsx or json")
		}
		params["file_type"] = o.FileType
	}

	if o.SubMeetingId != "" {
		params["sub_meeting_id"] = o.SubMeetingId
	}

	if o.StartTime != "" {
		startTime, err := utils.ISO8601ToTimeStamp(o.StartTime)
		if err != nil {
			return exception.InvalidArgsError.With("--start format error: %v", err)
		}
		params["start_time"] = startTime
	}

	if o.EndTime != "" {
		endTime, err := utils.ISO8601ToTimeStamp(o.EndTime)
		if err != nil {
			return exception.InvalidArgsError.With("--end format error: %v", err)
		}
		params["end_time"] = endTime
	}

	req := &thttp.Request{
		ApiURI: "/v1/meetings/export-participants-list",
		Body:   params,
	}
	rsp, err := restProxy.RequestProxy(cmd.Context(), http.MethodPost, o.tmeet, req)
	if err != nil {
		return err
	}

	output.FormatPrint(cmd, rsp.TraceId, rsp.Message, rsp.Data)
	return nil
}
