package auth

import (
	"context"
	"net/http"
	"tmeet/internal"
	"tmeet/internal/core/thttp"
	"tmeet/internal/log"
	"tmeet/internal/output"
	restProxy "tmeet/internal/proxy/rest-proxy"

	"github.com/spf13/cobra"
)

// ExchangeCodeOptions holds the exchange-code options.
type ExchangeCodeOptions struct {
	tmeet *internal.Tmeet
}

// newExchangeCodeCmd is the internal command `tmeet auth exchange-code`.
//
// It exchanges the current CLI login session for a short-lived, one-time
// authorization code issued by the Tencent Meeting Open Platform
// (POST /v1/users/buddyone/auth-code). The code is intended to be consumed by
// the upstream client that spawned this CLI invocation (e.g. a 3rd-party agent
// desktop client installed on the same machine): the client forwards the code
// to its own backend, which then calls the Open Platform with its appId +
// appSecret to resolve the user's identity, thereby bridging the local CLI
// login state and the 3rd-party platform's user system.
func newExchangeCodeCmd(tmeet *internal.Tmeet) *cobra.Command {
	opts := &ExchangeCodeOptions{tmeet: tmeet}
	cmd := &cobra.Command{
		Use:    "exchange-code",
		Short:  "[internal] exchange current login session for a short-lived one-time auth code",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return opts.Run(cmd, args)
		},
	}
	return cmd
}

func (o *ExchangeCodeOptions) Run(cmd *cobra.Command, _ []string) error {
	rsp, err := o.exchangeAuthCode(cmd.Context())
	if err != nil {
		log.Errorf(cmd.Context(), "exchange auth code failed: %v", err)
		return err
	}
	output.FormatPrint(cmd, rsp.TraceId, rsp.Message, rsp.Data)
	return nil
}

// exchangeAuthCode calls the Open Platform to mint a short-lived one-time
// authorization code for the current logged-in user.
func (o *ExchangeCodeOptions) exchangeAuthCode(ctx context.Context) (*restProxy.ProxyRsp, error) {
	req := &thttp.Request{
		ApiURI: "/v1/users/buddyone/auth-code",
		Body: map[string]interface{}{
			"operator_id":      o.tmeet.UserConfig.OpenId,
			"operator_id_type": 2, // openId, same convention as /v1/cli/get-user-info
		},
	}
	return restProxy.RequestProxy(ctx, http.MethodPost, o.tmeet, req)
}
