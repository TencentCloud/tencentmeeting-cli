package meeting

import (
	"net/http"
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

// GetOptions holds the options for getting meeting details.
type GetOptions struct {
	tmeet       *internal.Tmeet
	MeetingID   string // Meeting ID, higher priority than meeting code
	MeetingCode string // Meeting code
}

// newGetCmd gets meeting details.
func newGetCmd(tmeet *internal.Tmeet) *cobra.Command {
	opts := &GetOptions{tmeet: tmeet}
	cmd := &cobra.Command{
		Use:   "get",
		Short: "get meeting details",
		RunE: middleWare.Chain(
			opts.Run,
			middleWare.WithApiCmd(cmdutil.FlagSwitch(
				cmdutil.FlagCase{
					When:   cmdutil.WhenStringFlagSet("meeting-id"),
					ApiCmd: cmdutil.ApiCmdMeetingGetById,
				},
				cmdutil.FlagCase{
					When:   cmdutil.WhenStringFlagSet("meeting-code"),
					ApiCmd: cmdutil.ApiCmdMeetingGetByCode,
				},
			)),
		),
	}

	// 填充参数
	cmd.Flags().StringVar(&opts.MeetingID, "meeting-id", "", "meeting ID, higher priority than `meeting_code`")
	cmd.Flags().StringVar(&opts.MeetingCode, "meeting-code", "", "meeting code")

	cmd.MarkFlagsOneRequired("meeting-id", "meeting-code")

	return cmd
}

func (o *GetOptions) Run(cmd *cobra.Command, args []string) error {
	var apiURI string
	pathParams := thttp.PathParams{}
	queryParams := thttp.QueryParams{}
	if o.MeetingID != "" {
		apiURI = "/v1/meetings/{meetingId}"
		pathParams.Set("meetingId", o.MeetingID)
		queryParams.Set("instanceid", "1") // PC, fixed value
		queryParams.Set("operator_id", o.tmeet.UserConfig.OpenId)
		queryParams.Set("operator_id_type", "2") // OpenId
	} else if o.MeetingCode != "" {
		apiURI = "/v1/meetings"
		pathParams = nil
		queryParams.Set("userid", o.tmeet.UserConfig.OpenId)
		queryParams.Set("instanceid", "1") // PC, fixed value
		queryParams.Set("meeting_code", cmdutil.FormatMeetingCode(o.MeetingCode))
	} else {
		return exception.InvalidArgsError.With("--meeting-id or --meeting-code is empty")
	}

	req := &thttp.Request{
		ApiURI:      apiURI,
		QueryParams: queryParams,
		PathParams:  pathParams,
	}
	rsp, err := restProxy.RequestProxy(cmd.Context(), http.MethodGet, o.tmeet, req)
	if err != nil {
		return err
	}

	// Enrich meeting with full recording snapshot via paginated API.
	rsp.Data = string(enrichMeetingWithFullRecords(cmd.Context(), o.tmeet, []byte(rsp.Data), "meeting_info_list"))

	convertMap := map[string]utils.FieldConverter{
		"start_time":                        utils.TimestampConverter,
		"end_time":                          utils.TimestampConverter,
		"meeting_info_list.status":          utils.MeetingStatusConverter,
		"meeting_type":                      utils.MeetingTypeConverter,
		"recurring_type":                    utils.MeetingRecurringTypeConverter,
		"time_zone":                         utils.Base64DecodeConverter,
		"until_date":                        utils.TimestampConverter,
		"until_type":                        utils.MeetingRecurringUntilTypeConverter,
		"media_start_time":                  utils.TimestampConverter,       // recording start time
		"duration":                          utils.DurationSecondsConverter, // recording duration (seconds -> HH:MM:SS)
		"meeting_info_list.records.subject": utils.Base64DecodeConverter,    // recording subject (base64 -> plain text)
	}
	output.FormatPrint(cmd, rsp.TraceId, rsp.Message, rsp.Data,
		output.WithConvert(convertMap))
	return nil
}
