package rest_proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"strconv"
	"time"
	"tmeet/internal"
	"tmeet/internal/auth"
	"tmeet/internal/config"
	"tmeet/internal/core/thttp"
	"tmeet/internal/exception"
	"tmeet/internal/utils/retry"
)

const (
	OpenSourceCLI = "CLI"
	Success       = "success"
)

// ProxyRsp is the proxy response.
type ProxyRsp struct {
	Message string `json:"message,omitempty"`
	Data    string `json:"data,omitempty"`     // Response data
	TraceId string `json:"trace_id,omitempty"` // Response trace_id
}

// ProxyError is the proxy error.
type ProxyError struct {
	ErrorInfo *ProxyErrorInfo `json:"error_info"`
}

// ProxyErrorInfo is the unified error detail response structure.
type ProxyErrorInfo struct {
	ErrorCode    int32  `json:"error_code,omitempty"`
	NewErrorCode int32  `json:"new_error_code,omitempty"` // Error code with prefix (only valid for error codes exposed before refactoring)
	ErrorMessage string `json:"message,omitempty"`
}

// RequestProxy is the restapi request proxy.
func RequestProxy(ctx context.Context, method string, tmeet *internal.Tmeet, req *thttp.Request) (*ProxyRsp, error) {
	// Validate & refresh token.
	if err := auth.NewTmeetAuth(tmeet).RefreshToken(ctx); err != nil {
		return nil, err
	}

	var rsp *ProxyRsp
	opts := retry.DefaultOptions
	// Stop retrying immediately on token expiry to avoid pointless retries.
	opts.RetryIf = func(err error) bool {
		return !exception.Is(err, exception.TokenExpiredError)
	}

	err := retry.Do(ctx, func(ctx context.Context) error {
		tempRsp, err := requestProxy(ctx, method, tmeet, req)
		if err != nil {
			if err == exception.TokenExpiredError {
				// Token expired, clear local token and propagate error; RetryIf will stop retrying.
				_ = config.ClearUserConfig()
			}
			return err
		}
		rsp = tempRsp
		return nil
	}, opts)

	return rsp, err
}

func requestProxy(ctx context.Context, method string, tmeet *internal.Tmeet, req *thttp.Request) (*ProxyRsp, error) {
	opts := []thttp.RequestOptionFunc{
		header(tmeet.UserConfig.OpenId, tmeet.SystemInfo.MachineID, tmeet.CLIVersion,
			tmeet.SystemInfo.OS, tmeet.SystemInfo.Agent, tmeet.SystemInfo.Model),
		authenticator(tmeet.UserConfig.OpenId, tmeet.UserConfig.AccessToken),
	}

	rsp, err := doRequest(ctx, method, tmeet, req, opts)
	if err != nil {
		return nil, exception.NetworkError
	}

	var traceId string
	if rsp.Header != nil {
		traceId = rsp.Header.Get("X-TC-Trace")
	}

	// Non-200 status code always indicates an error.
	if rsp.StatusCode != http.StatusOK {
		proxyError := &ProxyError{}
		if marshalErr := json.Unmarshal(rsp.RawBody, proxyError); marshalErr == nil {
			if proxyError.ErrorInfo != nil &&
				proxyError.ErrorInfo.NewErrorCode == exception.ServerCodeTokenExpired {
				// Token invalid/expired, prompt user to re-login.
				return nil, exception.TokenExpiredError
			}
		}
		return nil, exception.RestBusinessError.With("request failed, http status:%d, business err: %s, trace:%s", rsp.StatusCode, string(rsp.RawBody), traceId)
	}

	return &ProxyRsp{
		Data:    string(rsp.RawBody),
		TraceId: traceId,
		Message: Success,
	}, nil
}

// doRequest dispatches HTTP requests by method.
func doRequest(ctx context.Context, method string, tmeet *internal.Tmeet, req *thttp.Request, opts []thttp.RequestOptionFunc) (*thttp.Response, error) {
	switch method {
	case http.MethodGet:
		return tmeet.RestClient.Get(ctx, req, opts...)
	case http.MethodPost:
		return tmeet.RestClient.Post(ctx, req, opts...)
	case http.MethodPut:
		return tmeet.RestClient.Put(ctx, req, opts...)
	case http.MethodDelete:
		return tmeet.RestClient.Delete(ctx, req, opts...)
	default:
		return nil, exception.InvalidRestApiMethodError
	}
}

// authenticator builds the authentication information.
func authenticator(openId, accessToken string) thttp.RequestOptionFunc {
	// oauth2 authenticator
	rn := rand.New(rand.NewSource(time.Now().UnixNano()))
	nonce := uint64(100000 + rn.Intn(900000))
	curTs := strconv.FormatInt(time.Now().Unix(), 10)
	x := &thttp.OAuth2Authenticator{
		Nonce:       nonce,
		Timestamp:   curTs,
		AccessToken: accessToken,
		OpenId:      openId,
	}

	return thttp.WithRequestAuthenticator(x)
}

// header builds the common request headers.
func header(openId, machineId, version, os, agent, model string) thttp.RequestOptionFunc {
	x := http.Header{}
	x.Set("Tmeet-Unique-ID", fmt.Sprintf("%s*%s", openId, machineId))
	x.Set("Tmeet-Device-Info", fmt.Sprintf("%s;%s;%s", os, agent, model))
	x.Set("Tmeet-Open-Source", OpenSourceCLI)
	x.Set("Tmeet-Cli-Ver", version)
	return thttp.WithRequestHeader(x)
}
