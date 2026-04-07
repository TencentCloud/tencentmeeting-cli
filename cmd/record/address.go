package record

import (
	"net/http"
	"strconv"
	"tmeet/internal"
	"tmeet/internal/core/thttp"
	"tmeet/internal/log"
	restProxy "tmeet/internal/proxy/rest-proxy"

	"github.com/spf13/cobra"
)

// AddressOptions holds the options for getting record download addresses.
type AddressOptions struct {
	tmeet           *internal.Tmeet
	MeetingRecordID string // Meeting record ID
	Page            int    // Page number, starting from 1
	PageSize        int    // Page size
}

// newAddressCmd gets record download addresses.
func newAddressCmd(tmeet *internal.Tmeet) *cobra.Command {
	opts := &AddressOptions{tmeet: tmeet}
	cmd := &cobra.Command{
		Use:   "address",
		Short: "get record download address",
		RunE: func(cmd *cobra.Command, args []string) error {
			return opts.Run(cmd, args)
		},
	}

	cmd.Flags().StringVar(&opts.MeetingRecordID, "meeting-record-id", "", "meeting record id (required)")
	cmd.Flags().IntVar(&opts.Page, "page", 1, "page number, starting from 1")
	cmd.Flags().IntVar(&opts.PageSize, "page-size", 50, "page size")

	_ = cmd.MarkFlagRequired("meeting-record-id")

	return cmd
}

func (o *AddressOptions) Run(cmd *cobra.Command, args []string) error {
	queryParams := thttp.QueryParams{}
	queryParams.Set("meeting_record_id", o.MeetingRecordID)
	queryParams.Set("operator_id", o.tmeet.UserConfig.OpenId)
	queryParams.Set("operator_id_type", "2") // OpenId
	queryParams.Set("page_size", strconv.FormatInt(int64(o.PageSize), 10))
	queryParams.Set("page", strconv.FormatInt(int64(o.Page), 10))

	req := &thttp.Request{
		ApiURI:      "/v1/mcp/addresses",
		QueryParams: queryParams,
	}
	rsp, err := restProxy.RequestProxy(cmd.Context(), http.MethodGet, o.tmeet, req)
	if err != nil {
		return err
	}

	// Parse response.
	log.Infof(cmd, restProxy.Print(cmd, rsp))
	return nil
}
