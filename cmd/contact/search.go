package contact

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

// SearchOptions holds the options for searching contacts.
type SearchOptions struct {
	tmeet          *internal.Tmeet
	Username       string // Username to search
	JobTitle       string // Job title to search
	DepartmentName string // DepartmentName to search
}

// newSearchCmd searches contacts.
func newSearchCmd(tmeet *internal.Tmeet) *cobra.Command {
	opts := &SearchOptions{tmeet: tmeet}
	cmd := &cobra.Command{
		Use:   "search",
		Short: "search contacts",
		RunE: middleWare.Chain(
			opts.Run,
			middleWare.WithApiCmd(cmdutil.StaticApiCmd(cmdutil.ApiCmdContactSearch)),
			middleWare.WithCompact(tmeet),
		),
	}

	cmd.Flags().StringVar(&opts.Username, "username", "", "username to search (required)")
	cmd.Flags().StringVar(&opts.JobTitle, "job-title", "",
		"job title to filter results when the username search returns too many matches")
	cmd.Flags().StringVar(&opts.DepartmentName, "department-name", "",
		"department name to filter results when the username search returns too many matches")

	// mark required flags
	_ = cmd.MarkFlagRequired("username")

	return cmd
}

// Run executes the search contacts command.
func (o *SearchOptions) Run(cmd *cobra.Command, args []string) error {
	queryParams := thttp.QueryParams{}
	queryParams.Set("username", o.Username)
	queryParams.Set("job_title", o.JobTitle)
	queryParams.Set("department_name", o.DepartmentName)
	queryParams.Set("operator_id", o.tmeet.UserConfig.OpenId)
	queryParams.Set("operator_id_type", "2") // OpenId

	req := &thttp.Request{
		ApiURI:      "/v1/contacts/members/search",
		QueryParams: queryParams,
	}
	rsp, err := restProxy.RequestProxy(cmd.Context(), http.MethodGet, o.tmeet, req)
	if err != nil {
		return err
	}

	output.FormatPrint(cmd, rsp.TraceId, rsp.Message, rsp.Data,
		output.WithCompact(middleWare.GetCompactFields(cmd.Context())),
		output.WithContactSearchLogic())
	return nil
}
