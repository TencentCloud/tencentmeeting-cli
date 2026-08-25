package record

import (
	"net/http"
	"tmeet/internal"
	"tmeet/internal/core/thttp"
	"tmeet/internal/output"
	restProxy "tmeet/internal/proxy/rest-proxy"

	"github.com/spf13/cobra"
)

// SettingsGetOptions holds options for querying recording sharing settings.
type SettingsGetOptions struct {
	tmeet           *internal.Tmeet
	MeetingRecordID string
}

func newSettingsGetCmd(tmeet *internal.Tmeet) *cobra.Command {
	opts := &SettingsGetOptions{tmeet: tmeet}
	cmd := &cobra.Command{
		Use:   "settings-get",
		Short: "get record sharing settings",
		RunE:  opts.Run,
	}
	cmd.Flags().StringVar(&opts.MeetingRecordID, "meeting-record-id", "", "meeting record id (required)")
	_ = cmd.MarkFlagRequired("meeting-record-id")
	return cmd
}

func (o *SettingsGetOptions) Run(cmd *cobra.Command, args []string) error {
	query := thttp.QueryParams{}
	query.Set("operator_id", o.tmeet.UserConfig.OpenId)
	query.Set("operator_id_type", "2")
	req := &thttp.Request{
		ApiURI:      "/v1/records/settings/{meeting_record_id}",
		PathParams:  thttp.PathParams{"meeting_record_id": o.MeetingRecordID},
		QueryParams: query,
	}
	rsp, err := restProxy.RequestProxy(cmd.Context(), http.MethodGet, o.tmeet, req)
	if err != nil {
		return err
	}
	output.FormatPrint(cmd, rsp.TraceId, rsp.Message, rsp.Data)
	return nil
}
