package record

import (
	"net/http"
	"tmeet/internal"
	"tmeet/internal/cmdutil"
	middleWare "tmeet/internal/cmdutil/middleware"
	"tmeet/internal/core/thttp"
	"tmeet/internal/output"
	restProxy "tmeet/internal/proxy/rest-proxy"

	"github.com/spf13/cobra"
)

// SmartMinutesOptions holds the options for smart minutes.
type SmartMinutesOptions struct {
	tmeet        *internal.Tmeet
	RecordFileId string // Record file ID
	Lang         string // Language for translation: default (no translation), zh (Simplified Chinese), en (English), ja (Japanese)
	Pwd          string // Record file access password
}

// newSmartMinutesCmd gets smart minutes.
func newSmartMinutesCmd(tmeet *internal.Tmeet) *cobra.Command {
	opts := &SmartMinutesOptions{tmeet: tmeet}
	cmd := &cobra.Command{
		Use:   "smart-minutes",
		Short: "get smart minutes from record",
		RunE: middleWare.Chain(
			opts.Run,
			middleWare.WithApiCmd(cmdutil.StaticApiCmd(cmdutil.ApiCmdRecordSmartMinutes)),
			middleWare.WithCompact(tmeet),
		),
	}

	cmd.Flags().StringVar(&opts.RecordFileId, "record-file-id", "", "record file id (required)")
	cmd.Flags().StringVar(&opts.Lang, "lang", "default", "language: default, zh, en, ja")
	cmd.Flags().StringVar(&opts.Pwd, "pwd", "", "record file access password")

	_ = cmd.MarkFlagRequired("record-file-id")

	return cmd
}

func (o *SmartMinutesOptions) Run(cmd *cobra.Command, args []string) error {
	queryParams := thttp.QueryParams{}
	queryParams.Set("operator_id", o.tmeet.UserConfig.OpenId)
	queryParams.Set("operator_id_type", "2") // OpenId
	queryParams.Set("lang", o.Lang)
	if o.Pwd != "" {
		queryParams.Set("pwd", o.Pwd)
	}

	req := &thttp.Request{
		ApiURI:      "/v1/smart/minutes/{record_file_id}",
		QueryParams: queryParams,
		PathParams: thttp.PathParams{
			"record_file_id": o.RecordFileId,
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
