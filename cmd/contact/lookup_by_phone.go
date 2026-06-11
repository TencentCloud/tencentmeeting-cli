package contact

import (
	"fmt"
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

const (
	maxPhonesPerLookup = 50 // Maximum number of phone numbers allowed in a single lookup
)

// LookupByPhoneOptions holds the options for looking up users by phone.
type LookupByPhoneOptions struct {
	tmeet  *internal.Tmeet
	Phones []string // Phone numbers array
}

// newLookupByPhoneCmd creates a command to look up users by phone number.
func newLookupByPhoneCmd(tmeet *internal.Tmeet) *cobra.Command {
	opts := &LookupByPhoneOptions{tmeet: tmeet}
	cmd := &cobra.Command{
		Use:   "lookup-by-phone",
		Short: "look up users by phone number",
		RunE: middleWare.Chain(
			opts.Run,
			middleWare.WithApiCmd(cmdutil.StaticApiCmd(cmdutil.ApiCmdContactLookupByPhone)),
			middleWare.WithCompact(tmeet),
		),
	}

	cmd.Flags().StringSliceVar(&opts.Phones, "phones", []string{}, fmt.Sprintf(
		"phone numbers to look up, comma-separated or repeat the flag, max %d"+
			"\neg. --phones 13800138000,13900139000 or "+
			"--phones 13800138000 --phones 13900139000 (required)", maxPhonesPerLookup))

	// mark required flags
	_ = cmd.MarkFlagRequired("phones")

	return cmd
}

// Run executes the lookup by phone command.
func (o *LookupByPhoneOptions) Run(cmd *cobra.Command, args []string) error {
	// Validate phone number count (maximum common.maxPhonesPerLookup)
	if len(o.Phones) > maxPhonesPerLookup {
		return exception.InvalidArgsError.With("phone numbers exceed maximum limit of %d", maxPhonesPerLookup)
	}

	// Validate phone number format
	for _, phone := range o.Phones {
		if err := utils.ValidatePhone(phone); err != nil {
			return err
		}
	}

	body := map[string]interface{}{
		"operator_id":      o.tmeet.UserConfig.OpenId,
		"operator_id_type": 2, // OpenId
		"users_phone":      o.Phones,
	}

	req := &thttp.Request{
		ApiURI: "/v1/contacts/members/lookup-by-phone",
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
