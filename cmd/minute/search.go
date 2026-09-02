package minute

import (
	"net/http"
	"strconv"
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

// Character length limits for search fields.
const (
	maxQueryLen = 50
)

// pageSizeMaxMinutes is the max page size for minute search.
const pageSizeMaxMinutes = 50

// SearchOptions holds the options for searching minutes.
type SearchOptions struct {
	tmeet     *internal.Tmeet
	Query     string // search keyword, max 50 characters
	From      string // start time, ISO 8601, e.g. 2026-03-12T15:00+08:00
	To        string // end time, ISO 8601, e.g. 2026-03-12T15:00+08:00
	PageSize  int    // page size, default 20, max 50
	PageToken string // page token for pagination
}

// newSearchCmd searches Yuanbao minutes.
func newSearchCmd(tmeet *internal.Tmeet) *cobra.Command {
	opts := &SearchOptions{tmeet: tmeet}
	cmd := &cobra.Command{
		Use:   "search",
		Short: "search Yuanbao minutes by keyword or time range",
		RunE: middleWare.Chain(
			opts.Run,
			middleWare.WithApiCmd(cmdutil.StaticApiCmd(cmdutil.ApiCmdMinuteSearch)),
		),
	}

	cmd.Flags().StringVar(&opts.Query, "query", "", "search keyword, max 50 characters")
	cmd.Flags().StringVar(&opts.From, "start", "", "lower bound of search time window (ISO 8601, e.g. 2026-03-12T15:00+08:00)")
	cmd.Flags().StringVar(&opts.To, "end", "", "upper bound of search time window (ISO 8601, e.g. 2026-03-12T15:00+08:00)")
	cmd.Flags().IntVar(&opts.PageSize, "page-size", 20, "page size, default 20, max 50")
	cmd.Flags().StringVar(&opts.PageToken, "page-token", "", "page token for pagination")

	return cmd
}

// Run executes the search command.
func (o *SearchOptions) Run(cmd *cobra.Command, args []string) error {
	queryParams := thttp.QueryParams{}
	queryParams.Set("operator_id", o.tmeet.UserConfig.OpenId)
	queryParams.Set("operator_id_type", "2") // OpenId

	// page handler
	pageSize, err := cmdutil.ClampingPageSize(cmd, o.PageSize, pageSizeMaxMinutes)
	if err != nil {
		return err
	}
	queryParams.Set("page_token", o.PageToken)
	queryParams.Set("page_size", strconv.Itoa(pageSize))

	// search keyword
	if o.Query != "" {
		if err := utils.CharacterLimit("--query", o.Query, maxQueryLen); err != nil {
			return err
		}
		queryParams.Set("q", o.Query)
	}

	// time range
	var start, end int64
	if o.From != "" {
		start, err = utils.ISO8601ToTimeStamp(o.From)
		if err != nil {
			return exception.InvalidArgsError.With("--start format error: %v", err)
		}
		queryParams.Set("from", o.From)
	}
	if o.To != "" {
		end, err = utils.ISO8601ToTimeStamp(o.To)
		if err != nil {
			return exception.InvalidArgsError.With("--end format error: %v", err)
		}
		queryParams.Set("to", o.To)
	}
	if start > 0 && end > 0 && start >= end {
		return exception.InvalidArgsError.With("--start must be earlier than --end")
	}

	req := &thttp.Request{
		ApiURI:      "/v1/mcp/asr/search-minutes-mcp",
		QueryParams: queryParams,
	}
	rsp, err := restProxy.RequestProxy(cmd.Context(), http.MethodGet, o.tmeet, req)
	if err != nil {
		return err
	}

	output.FormatPrint(cmd, rsp.TraceId, rsp.Message, rsp.Data,
		output.WithTotalCountLogic())
	return nil
}
