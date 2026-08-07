package report

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

// JobResultOptions holds the options for job result query.
type JobResultOptions struct {
	tmeet *internal.Tmeet
	JobId string // Task ID
}

// newJobResultCmd gets the async task result.
func newJobResultCmd(tmeet *internal.Tmeet) *cobra.Command {
	opts := &JobResultOptions{tmeet: tmeet}
	cmd := &cobra.Command{
		Use:   "job-result",
		Short: "get async task result",
		RunE: middleWare.Chain(
			opts.Run,
			middleWare.WithApiCmd(cmdutil.StaticApiCmd(cmdutil.ApiCmdReportJobResult)),
		),
	}

	cmd.Flags().StringVar(&opts.JobId, "job-id", "", "task id (required)")

	// mark required flags
	_ = cmd.MarkFlagRequired("job-id")

	return cmd
}

func (o *JobResultOptions) Run(cmd *cobra.Command, args []string) error {
	queryParams := thttp.QueryParams{}
	queryParams.Set("operator_id", o.tmeet.UserConfig.OpenId)
	queryParams.Set("operator_id_type", "2") // OpenId

	req := &thttp.Request{
		ApiURI:      "/v1/export/{job_id}",
		PathParams:  thttp.PathParams{"job_id": o.JobId},
		QueryParams: queryParams,
	}
	rsp, err := restProxy.RequestProxy(cmd.Context(), http.MethodGet, o.tmeet, req)
	if err != nil {
		return err
	}

	convertMap := map[string]utils.FieldConverter{
		"status": utils.ExportJobStatusConverter,
	}
	output.FormatPrint(cmd, rsp.TraceId, rsp.Message, rsp.Data,
		output.WithConvert(convertMap))
	return nil
}
