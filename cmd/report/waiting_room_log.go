package report

import (
	"net/http"
	"strconv"
	"tmeet/internal"
	"tmeet/internal/cmdutil"
	middleWare "tmeet/internal/cmdutil/middleware"
	"tmeet/internal/core/thttp"
	"tmeet/internal/output"
	restProxy "tmeet/internal/proxy/rest-proxy"
	"tmeet/internal/utils"

	"github.com/spf13/cobra"
)

// WaitingRoomOptions holds the options for waiting room.
type WaitingRoomOptions struct {
	tmeet     *internal.Tmeet
	MeetingId string // Meeting ID
	PageSize  int    // Page size
	Page      int    // Page number, deprecated use PageToken instead
	PageToken string // Page token
}

// newWaitingRoomCmd gets the waiting room members list.
func newWaitingRoomCmd(tmeet *internal.Tmeet) *cobra.Command {
	opts := &WaitingRoomOptions{tmeet: tmeet}
	cmd := &cobra.Command{
		Use:   "waiting-room-log",
		Short: "get waiting room members",
		RunE: middleWare.Chain(
			opts.Run,
			middleWare.WithApiCmd(cmdutil.StaticApiCmd(cmdutil.ApiCmdReportWaitingRoomLog)),
			middleWare.WithCompact(tmeet),
		),
	}

	cmd.Flags().StringVar(&opts.MeetingId, "meeting-id", "", "meeting id (required)")
	cmd.Flags().IntVar(&opts.PageSize, "page-size", 100, "page size, default 100, max 100")
	cmd.Flags().IntVar(&opts.Page, "page", 0, "page number, (deprecated, use --page-token instead)")
	cmd.Flags().StringVar(&opts.PageToken, "page-token", "", "page token")

	// mark required flags
	_ = cmd.MarkFlagRequired("meeting-id")
	// mark deprecated flags
	_ = cmd.Flags().MarkDeprecated("page", "use --page-token instead")

	return cmd
}

func (o *WaitingRoomOptions) Run(cmd *cobra.Command, args []string) error {
	queryParams := thttp.QueryParams{}
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
	pageSize, err := cmdutil.ClampingPageSize(cmd, o.PageSize, cmdutil.PageSizeMaxReports)
	if err != nil {
		return err
	}
	queryParams.Set("page_size", strconv.Itoa(pageSize))

	req := &thttp.Request{
		ApiURI:      "/v1/meeting/{meeting_id}/waiting-room",
		PathParams:  thttp.PathParams{"meeting_id": o.MeetingId},
		QueryParams: queryParams,
	}
	rsp, err := restProxy.RequestProxy(cmd.Context(), http.MethodGet, o.tmeet, req)
	if err != nil {
		return err
	}

	convertMap := map[string]utils.FieldConverter{
		"join_time":           utils.TimestampConverter,
		"left_time":           utils.TimestampConverter,
		"schedule_start_time": utils.TimestampConverter,
		"schedule_end_time":   utils.TimestampConverter,
		"instanceid":          utils.InstanceIdConverter,
		"user_name":           utils.Base64DecodeConverter,
		"subject":             utils.Base64DecodeConverter,
	}
	output.FormatPrint(cmd, rsp.TraceId, rsp.Message, rsp.Data,
		output.WithCompact(middleWare.GetCompactFields(cmd.Context())),
		output.WithConvert(convertMap))
	return nil
}
