package app

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

// GetOptions holds the options for getting app info.
type GetOptions struct {
	tmeet *internal.Tmeet
}

// newGetCmd gets app info.
func newGetCmd(tmeet *internal.Tmeet) *cobra.Command {
	opts := &GetOptions{tmeet: tmeet}
	cmd := &cobra.Command{
		Use:   "get",
		Short: "get your current cli-app info",
		RunE: middleWare.Chain(
			opts.Run,
			middleWare.WithApiCmd(cmdutil.StaticApiCmd(cmdutil.ApiCmdAppInfoGet)),
		),
	}

	return cmd
}

func (o *GetOptions) Run(cmd *cobra.Command, args []string) error {
	queryParams := thttp.QueryParams{}
	queryParams.Set("operator_id", o.tmeet.UserConfig.OpenId)
	queryParams.Set("operator_id_type", "2") // openId

	req := &thttp.Request{
		ApiURI:      "/v1/cli/app-detail",
		QueryParams: queryParams,
	}
	rsp, err := restProxy.RequestProxy(cmd.Context(), http.MethodGet, o.tmeet, req)
	if err != nil {
		return err
	}

	output.FormatPrint(cmd, rsp.TraceId, rsp.Message, rsp.Data)
	return nil
}
