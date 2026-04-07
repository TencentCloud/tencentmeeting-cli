package auth

import (
	"context"
	"net/http"
	"tmeet/internal/core/thttp"
	"tmeet/internal/exception"
	cgiProxy "tmeet/internal/proxy/cgi-proxy"
)

type AuthCodeData struct {
	AuthCode string `json:"auth_code,omitempty"`
}

// getDeviceAuthCode retrieves the device authorization code.
func (w *TmeetAuth) getDeviceAuthCode(ctx context.Context) (string, error) {
	req := thttp.Request{
		ApiURI: "/v2/oauth2/oauth/cli-oauth-init",
		Body: map[string]interface{}{
			"device_id": w.tmeet.SystemInfo.MachineID,
		},
	}
	rsp, err := cgiProxy.RequestProxy[*AuthCodeData](ctx, http.MethodPost, w.tmeet, &req)
	if err != nil {
		return "", err
	}
	if rsp.Code != 0 || rsp.Data == nil {
		return "", exception.GetAuthCodeError
	}
	return rsp.Data.AuthCode, nil
}
