package record

import (
	"net/http"
	"strconv"

	"tmeet/internal/cmdutil"
	middleWare "tmeet/internal/cmdutil/middleware"
	"tmeet/internal/core/thttp"
	"tmeet/internal/exception"
	"tmeet/internal/output"
	restProxy "tmeet/internal/proxy/rest-proxy"

	"github.com/spf13/cobra"
)

// buildAddressHandler declares the `--action address` branch, corresponding
// to legacy `record address`. Returns record file download / playback URLs
// for a given meeting_record_id.
func (o *AccessOptions) buildAddressHandler() *cmdutil.ActionHandler {
	return &cmdutil.ActionHandler{
		ApiCmd: cmdutil.ApiCmdRecordAddress,
		FlagRules: []cmdutil.FlagRule{
			{Reject: []string{"record-file-id"}},
		},
		Validate: func(cmd *cobra.Command) error {
			if o.MeetingRecordID == "" {
				return exception.InvalidArgsError.With("--action address requires --meeting-record-id")
			}
			return nil
		},
		Run: o.runAddress,
	}
}

// runAddress corresponds to legacy `address`.
func (o *AccessOptions) runAddress(cmd *cobra.Command) error {
	queryParams := thttp.QueryParams{}
	queryParams.Set("meeting_record_id", o.MeetingRecordID)
	queryParams.Set("operator_id", o.tmeet.UserConfig.OpenId)
	queryParams.Set("operator_id_type", "2") // OpenId

	// Only --page-token is supported (legacy --page is dropped).
	queryParams.Set("page_type", strconv.Itoa(cmdutil.PageTypeToken))
	queryParams.Set("page_token", o.PageToken)

	pageSize, err := cmdutil.ClampingPageSize(cmd, o.PageSize, cmdutil.PageSizeMaxRecords)
	if err != nil {
		return err
	}
	queryParams.Set("page_size", strconv.Itoa(pageSize))

	req := &thttp.Request{
		ApiURI:      "/v1/mcp/addresses",
		QueryParams: queryParams,
	}
	rsp, err := restProxy.RequestProxy(cmd.Context(), http.MethodGet, o.tmeet, req)
	if err != nil {
		return err
	}
	output.FormatPrint(cmd, rsp.TraceId, rsp.Message, rsp.Data,
		output.WithCompact(middleWare.GetCompactFields(cmd.Context())))
	return nil
}
