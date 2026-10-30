package record

import (
	"net/http"
	"tmeet/internal/utils"

	"tmeet/internal/cmdutil"
	middleWare "tmeet/internal/cmdutil/middleware"
	"tmeet/internal/core/thttp"
	"tmeet/internal/exception"
	"tmeet/internal/output"
	restProxy "tmeet/internal/proxy/rest-proxy"

	"github.com/spf13/cobra"
)

// buildSettingsHandler declares the `--action settings` branch, corresponding
// to legacy `smart-config-get`. Returns the smart recording config (subtitle
// switch / AI switch / support flag) of a record file.
func (o *AccessOptions) buildSettingsHandler() *cmdutil.ActionHandler {
	return &cmdutil.ActionHandler{
		ApiCmd: cmdutil.ApiCmdRecordQuerySmartConfig,
		FlagRules: []cmdutil.FlagRule{
			{Reject: []string{"meeting-record-id", "page-token", "page-size"}},
		},
		Validate: func(cmd *cobra.Command) error {
			if o.RecordFileID == "" {
				return exception.InvalidArgsError.With("--action settings requires --record-file-id")
			}
			return nil
		},
		Run: o.runSettings,
	}
}

// runSettings corresponds to legacy `smart-config-get`.
func (o *AccessOptions) runSettings(cmd *cobra.Command) error {
	queryParams := thttp.QueryParams{}
	queryParams.Set("operator_id", o.tmeet.UserConfig.OpenId)
	queryParams.Set("operator_id_type", "2") // OpenId
	queryParams.Set("record_file_id", o.RecordFileID)

	req := &thttp.Request{
		ApiURI:      "/v1/mcp/records/smart-config",
		QueryParams: queryParams,
	}
	rsp, err := restProxy.RequestProxy(cmd.Context(), http.MethodGet, o.tmeet, req)
	if err != nil {
		return err
	}
	output.FormatPrint(cmd, rsp.TraceId, rsp.Message, rsp.Data,
		output.WithCompact(middleWare.GetCompactFields(cmd.Context())),
		output.WithConvert(map[string]utils.FieldConverter{
			"enable_ai_switch": utils.RecordSettingTypeConverter,
			"subtitle_switch":  utils.RecordSettingTypeConverter,
		}))
	return nil
}
