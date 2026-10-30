package record

import (
	"fmt"
	"net/http"

	"tmeet/internal"
	"tmeet/internal/cmdutil"
	middleWare "tmeet/internal/cmdutil/middleware"
	"tmeet/internal/core/thttp"
	"tmeet/internal/output"
	restProxy "tmeet/internal/proxy/rest-proxy"

	"github.com/spf13/cobra"
)

// SettingsSetOptions holds the options for modifying smart recording config.
type SettingsSetOptions struct {
	tmeet          *internal.Tmeet
	RecordFileID   string // Record file ID (required)
	SubtitleSwitch int    // Transcript switch: 1-off 2-on
	EnableAISwitch int    // Smart recording switch: 1-off 2-on
}

// newSettingsSetCmd modifies the smart recording config for a record file.
func newSettingsSetCmd(tmeet *internal.Tmeet) *cobra.Command {
	opts := &SettingsSetOptions{tmeet: tmeet}
	cmd := &cobra.Command{
		Use:   "settings-set",
		Short: "set smart recording config (subtitle switch / AI switch) for a record file (write operation, confirm before executing)",
		RunE: middleWare.Chain(
			opts.Run,
			middleWare.WithApiCmd(cmdutil.StaticApiCmd(cmdutil.ApiCmdRecordModifySmartConfig)),
		),
	}

	cmd.Flags().StringVar(&opts.RecordFileID, "record-file-id", "", "record file id (required)")
	cmd.Flags().IntVar(&opts.SubtitleSwitch, "subtitle-switch", 0, "transcript switch: 1-off 2-on")
	cmd.Flags().IntVar(&opts.EnableAISwitch, "enable-ai-switch", 0, "smart recording switch: 1-off 2-on")

	_ = cmd.MarkFlagRequired("record-file-id")

	return cmd
}

// Run executes the settings-set command.
func (o *SettingsSetOptions) Run(cmd *cobra.Command, args []string) error {
	if !cmd.Flags().Changed("subtitle-switch") && !cmd.Flags().Changed("enable-ai-switch") {
		return fmt.Errorf("at least one of --subtitle-switch or --enable-ai-switch must be provided")
	}

	body := map[string]interface{}{
		"operator_id":      o.tmeet.UserConfig.OpenId,
		"operator_id_type": 2, // OpenId
		"record_file_id":   o.RecordFileID,
	}
	if cmd.Flags().Changed("subtitle-switch") {
		body["subtitle_switch"] = o.SubtitleSwitch
	}
	if cmd.Flags().Changed("enable-ai-switch") {
		body["enable_ai_switch"] = o.EnableAISwitch
	}

	req := &thttp.Request{
		ApiURI: "/v1/mcp/records/smart-config",
		Body:   body,
	}
	rsp, err := restProxy.RequestProxy(cmd.Context(), http.MethodPost, o.tmeet, req)
	if err != nil {
		return err
	}
	output.FormatPrint(cmd, rsp.TraceId, rsp.Message, rsp.Data,
		output.WithCompact(middleWare.GetCompactFields(cmd.Context())))
	return nil
}
