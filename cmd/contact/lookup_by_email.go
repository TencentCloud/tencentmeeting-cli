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
	maxEmailsPerLookup = 50 // Maximum number of email addresses allowed in a single lookup
)

// LookupByEmailOptions holds the options for looking up users by email.
type LookupByEmailOptions struct {
	tmeet  *internal.Tmeet
	Emails []string // Email addresses array
}

// newLookupByEmailCmd creates a command to look up users by email address.
func newLookupByEmailCmd(tmeet *internal.Tmeet) *cobra.Command {
	opts := &LookupByEmailOptions{tmeet: tmeet}
	cmd := &cobra.Command{
		Use:   "lookup-by-email",
		Short: "look up users by email address",
		RunE: middleWare.Chain(
			opts.Run,
			middleWare.WithApiCmd(cmdutil.StaticApiCmd(cmdutil.ApiCmdContactLookupByEmail)),
			middleWare.WithCompact(tmeet),
		),
	}

	cmd.Flags().StringSliceVar(&opts.Emails, "emails", []string{}, fmt.Sprintf(
		"email addresses to look up, comma-separated or repeat the flag, max %d"+
			"\neg. --emails user1@example.com,user2@example.com or "+
			"--emails user1@example.com --emails user2@example.com (required)", maxEmailsPerLookup))

	// mark required flags
	_ = cmd.MarkFlagRequired("emails")

	return cmd
}

// Run executes the lookup by email command.
func (o *LookupByEmailOptions) Run(cmd *cobra.Command, args []string) error {
	// Validate email count (maximum common.maxEmailsPerLookup)
	if len(o.Emails) > maxEmailsPerLookup {
		return exception.InvalidArgsError.With("email addresses exceed maximum limit of %d", maxEmailsPerLookup)
	}

	// Validate email format
	for _, email := range o.Emails {
		if err := utils.ValidateEmail(email); err != nil {
			return err
		}
	}

	body := map[string]interface{}{
		"operator_id":      o.tmeet.UserConfig.OpenId,
		"operator_id_type": 2, // OpenId
		"users_email":      o.Emails,
	}

	req := &thttp.Request{
		ApiURI: "/v1/contacts/members/lookup-by-email",
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
