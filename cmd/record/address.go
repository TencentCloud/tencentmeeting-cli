package record

import (
	"net/http"
	"strconv"
	"tmeet/internal"
	"tmeet/internal/cmdutil"
	middleWare "tmeet/internal/cmdutil/middleware"
	"tmeet/internal/core/thttp"
	"tmeet/internal/output"
	restProxy "tmeet/internal/proxy/rest-proxy"

	"github.com/spf13/cobra"
)

// AddressOptions holds the options for getting record download addresses.
type AddressOptions struct {
	tmeet           *internal.Tmeet
	MeetingRecordID string // Meeting record ID
	Page            int    // Page number, starting from 1, deprecated use PageToken instead
	PageSize        int    // Page size
	PageToken       string // Page token
}

// newAddressCmd gets record download addresses.
func newAddressCmd(tmeet *internal.Tmeet) *cobra.Command {
	opts := &AddressOptions{tmeet: tmeet}
	cmd := &cobra.Command{
		Use:   "address",
		Short: "get record download address",
		RunE: middleWare.Chain(
			opts.Run,
			middleWare.WithApiCmd(cmdutil.StaticApiCmd(cmdutil.ApiCmdRecordAddress)),
			middleWare.WithCompact(tmeet),
		),
	}

	cmd.Flags().StringVar(&opts.MeetingRecordID, "meeting-record-id", "", "meeting record id (required)")
	cmd.Flags().IntVar(&opts.Page, "page", 0, "page number, starting from 1, (deprecated, use --page-token instead)")
	cmd.Flags().IntVar(&opts.PageSize, "page-size", 30, "page size, default 30, max 30")
	cmd.Flags().StringVar(&opts.PageToken, "page-token", "", "page token for pagination")

	// mark required flags
	_ = cmd.MarkFlagRequired("meeting-record-id")
	// mark deprecated flags
	_ = cmd.Flags().MarkDeprecated("page", "use page-token instead")

	return cmd
}

func (o *AddressOptions) Run(cmd *cobra.Command, args []string) error {
	queryParams := thttp.QueryParams{}
	queryParams.Set("meeting_record_id", o.MeetingRecordID)
	queryParams.Set("operator_id", o.tmeet.UserConfig.OpenId)
	queryParams.Set("operator_id_type", "2") // OpenId

	// pagination compatibility
	pageValue, pageType := cmdutil.ChoosePageOrToken(o.Page, o.PageToken)
	queryParams.Set("page_type", strconv.Itoa(pageType))
	if pageType == cmdutil.PageTypeOld {
		queryParams.Set("page", pageValue)
	} else {
		queryParams.Set("page_token", pageValue)
	}
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
