package auth

import (
	"context"
	"net/http"
	"tmeet/internal/core/thttp"
	"tmeet/internal/exception"
	cgiProxy "tmeet/internal/proxy/cgi-proxy"
)

type AuthTokenData struct {
	AccessToken            string `json:"access_token,omitempty"`
	RefreshToken           string `json:"refresh_token,omitempty"`
	AccessTokenExpireTime  string `json:"access_token_expire_time,omitempty"`
	RefreshTokenExpireTime string `json:"refresh_token_expire_time,omitempty"`
	OpenID                 string `json:"open_id,omitempty"`
	SdkId                  string `json:"sdk_id,omitempty"`
}

func (w *TmeetAuth) getAuthToken(ctx context.Context, authCode string) (*AuthTokenData, error) {
	req := thttp.Request{
		ApiURI: "/v2/oauth2/oauth/cli-oauth-poll",
		Body: map[string]interface{}{
			"device_id": w.tmeet.SystemInfo.MachineID,
			"auth_code": authCode,
		},
	}
	rsp, err := cgiProxy.RequestProxy[*AuthTokenData](ctx, http.MethodPost, w.tmeet, &req)
	if err != nil {
		return nil, err
	}
	if rsp.Code != 0 || rsp.Data == nil {
		return nil, exception.AuthorizationFailedError.With("authorization failed, %s", rsp.Message)
	}
	return rsp.Data, nil
}

func (w *TmeetAuth) refreshAuthToken(ctx context.Context) (*AuthTokenData, error) {
	req := thttp.Request{
		ApiURI: "/v2/oauth2/oauth/cli-refresh-token",
		Body: map[string]interface{}{
			"device_id":     w.tmeet.SystemInfo.MachineID,
			"refresh_token": w.tmeet.UserConfig.RefreshToken,
			"open_id":       w.tmeet.UserConfig.OpenId,
			"sdk_id":        w.tmeet.UserConfig.SdkId,
		},
	}
	rsp, err := cgiProxy.RequestProxy[*AuthTokenData](ctx, http.MethodPost, w.tmeet, &req)
	if err != nil {
		return nil, err
	}
	if rsp.Code != 0 {
		return nil, exception.RefreshTokenFailedError.With("refresh token failed, %s", rsp.Message)
	}
	return rsp.Data, nil
}
