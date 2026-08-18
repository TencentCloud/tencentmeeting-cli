package rest_proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"net/http"
	"strconv"
	"time"
	"tmeet/internal"
	"tmeet/internal/auth"
	"tmeet/internal/cmdutil/apicmdctx"
	"tmeet/internal/common"
	"tmeet/internal/config"
	"tmeet/internal/core/thttp"
	"tmeet/internal/exception"
	"tmeet/internal/log"
	"tmeet/internal/utils/retry"
)

const (
	OpenSourceCLI = "CLI"
	Success       = "success"
	// agentOperatorIdTypeOpenId is the operator_id_type value for OpenId-typed operator_id
	// when calling /v1/cli/refresh-sub-agent-token (and other agent OpenAPI endpoints).
	agentOperatorIdTypeOpenId = 2
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
	if err := auth.NewTmeetAuth(tmeet).RefreshToken(ctx, config.ClearUserConfig); err != nil {
		return nil, err
	}

	var rsp *ProxyRsp
	opts := retry.DefaultOptions
	// Stop retrying immediately on token expiry or non-retryable business errors to avoid pointless retries.
	opts.RetryIf = func(err error) bool {
		return !exception.Is(err, exception.TokenExpiredError) &&
			!exception.Is(err, exception.NotRetryRequestError)
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

	if err != nil {
		// restapi common err log
		log.Errorf(ctx, "restapi proxy err: %v", err)
		return nil, err
	}
	return rsp, nil
}

// RequestProxyWithAgent is the restapi request proxy for OpenAPI calls made under
// the agent (sub-account) identity.
//
// The HTTP request still authenticates as the master account (OAuth2 header,
// Tmeet-Unique-ID, Tmeet-Cli-Ver, etc.). Two extra headers are added so that the
// server can route the call as the agent identity:
//   - Tmeet-Agent-ID:  the active agent's open_id
//   - Tmeet-Agent-AK:  the active agent's access_token
//
// Both master-account and agent-account tokens are auto-refreshed before the call.
func RequestProxyWithAgent(ctx context.Context, method string, tmeet *internal.Tmeet,
	req *thttp.Request) (*ProxyRsp, error) {
	// Validate & refresh master token.
	if err := auth.NewTmeetAuth(tmeet).RefreshToken(ctx, config.ClearUserConfig); err != nil {
		return nil, err
	}
	// Validate & refresh agent token; HTTP transport is injected via fetcher to
	// keep auth -> rest-proxy decoupled.
	if err := auth.NewTmeetAuth(tmeet).RefreshAgentToken(ctx, agentTokenFetcher(tmeet)); err != nil {
		return nil, err
	}

	agentCfg, err := config.GetAgentAccountConfig()
	if err != nil {
		return nil, exception.GetUserConfigUnknownError.With("read local agent config failed: %v", err)
	}
	if agentCfg == nil || agentCfg.AgentOpenId == "" {
		return nil, exception.AgentNotFoundError
	}

	var rsp *ProxyRsp
	opts := retry.DefaultOptions
	opts.RetryIf = func(err error) bool {
		return !exception.Is(err, exception.TokenExpiredError) &&
			!exception.Is(err, exception.NotRetryRequestError)
	}

	err = retry.Do(ctx, func(ctx context.Context) error {
		tempRsp, err := requestProxyWithAgent(ctx, method, tmeet, req, agentCfg)
		if err != nil {
			if err == exception.TokenExpiredError {
				_ = config.ClearUserConfig()
			}
			return err
		}
		rsp = tempRsp
		return nil
	}, opts)

	if err != nil {
		log.Errorf(ctx, "restapi proxy with agent err: %v", err)
		return nil, err
	}
	return rsp, nil
}

func requestProxy(ctx context.Context, method string, tmeet *internal.Tmeet, req *thttp.Request) (*ProxyRsp, error) {
	opts := []thttp.RequestOptionFunc{
		header(ctx,
			tmeet.UserConfig.OpenId,
			tmeet.SystemInfo.MachineID,
			tmeet.CLIVersion,
			tmeet.SystemInfo.OS,
			tmeet.SystemInfo.Agent,
			tmeet.SystemInfo.Model,
			tmeet.CmdPath,
			apicmdctx.Get(ctx)),
		authenticator(tmeet.UserConfig.OpenId, tmeet.UserConfig.AccessToken),
	}

	rsp, err := doRequest(ctx, method, tmeet, req, opts)
	if err != nil {
		return nil, exception.NetworkError
	}
	if rsp.StatusCode == http.StatusRequestTimeout ||
		rsp.StatusCode == http.StatusGatewayTimeout {
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

		return nil, exception.NotRetryRequestError.With(
			"request failed, http status:%d, business err: %s, trace:%s", rsp.StatusCode, string(rsp.RawBody), traceId)
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
	nonce := uint64(100000 + rand.IntN(900000))
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
//
// apiCmd is the api-schema identifier of the current invocation (see
// internal/cmdutil/api_schema.go). When it is empty, the Tmeet-Cli-Name
// header is omitted.
func header(ctx context.Context, openId, machineId, version,
	os, agent, model, cmdPath, apiCmd string) thttp.RequestOptionFunc {
	x := http.Header{}
	x.Set("Tmeet-Unique-ID", common.BuildUniqueID(openId, machineId))
	x.Set("Tmeet-Device-Info", fmt.Sprintf("%s;%s;%s", os, agent, model))
	x.Set("Tmeet-Open-Source", OpenSourceCLI)
	x.Set("Tmeet-Cli-Ver", version)
	x.Set("Tmeet-Trace", fmt.Sprintf("%s;%s", cmdPath, ctx.Value(log.CtxTraceIDKey)))
	if apiCmd != "" {
		x.Set("Tmeet-Cli-Name", apiCmd)
	}
	return thttp.WithRequestHeader(x)
}

// agentHeader builds the master-account common headers and additionally injects
// the agent identity headers (Tmeet-Agent-ID / Tmeet-Agent-AK).
//
// All original headers and the OAuth2 authenticator are still based on the master
// account; the two extra headers tell the server to route the call as the agent.
func agentHeader(ctx context.Context, openId, machineId, version,
	os, agent, model, cmdPath, agentOpenId, agentAccessToken string) thttp.RequestOptionFunc {
	x := http.Header{}
	x.Set("Tmeet-Unique-ID", common.BuildUniqueID(openId, machineId))
	x.Set("Tmeet-Device-Info", fmt.Sprintf("%s;%s;%s", os, agent, model))
	x.Set("Tmeet-Open-Source", OpenSourceCLI)
	x.Set("Tmeet-Cli-Ver", version)
	x.Set("Tmeet-Trace", fmt.Sprintf("%s;%s", cmdPath, ctx.Value(log.CtxTraceIDKey)))
	x.Set("Tmeet-Agent-ID", agentOpenId)
	x.Set("Tmeet-Agent-AK", agentAccessToken)
	return thttp.WithRequestHeader(x)
}

// requestProxyWithAgent issues the actual HTTP request under the master-account
// OAuth2 authenticator with extra agent headers attached.
func requestProxyWithAgent(ctx context.Context, method string, tmeet *internal.Tmeet,
	req *thttp.Request, agentCfg *config.AgentAccountConfig) (*ProxyRsp, error) {
	opts := []thttp.RequestOptionFunc{
		agentHeader(ctx,
			tmeet.UserConfig.OpenId,
			tmeet.SystemInfo.MachineID,
			tmeet.CLIVersion,
			tmeet.SystemInfo.OS,
			tmeet.SystemInfo.Agent,
			tmeet.SystemInfo.Model,
			tmeet.CmdPath,
			agentCfg.AgentOpenId,
			agentCfg.AccessToken),
		authenticator(tmeet.UserConfig.OpenId, tmeet.UserConfig.AccessToken),
	}

	rsp, err := doRequest(ctx, method, tmeet, req, opts)
	if err != nil {
		return nil, exception.NetworkError
	}
	if rsp.StatusCode == http.StatusRequestTimeout ||
		rsp.StatusCode == http.StatusGatewayTimeout {
		return nil, exception.NetworkError
	}

	var traceId string
	if rsp.Header != nil {
		traceId = rsp.Header.Get("X-TC-Trace")
	}

	if rsp.StatusCode != http.StatusOK {
		proxyError := &ProxyError{}
		if marshalErr := json.Unmarshal(rsp.RawBody, proxyError); marshalErr == nil {
			if proxyError.ErrorInfo != nil &&
				proxyError.ErrorInfo.NewErrorCode == exception.ServerCodeTokenExpired {
				return nil, exception.TokenExpiredError
			}
		}
		return nil, exception.NotRetryRequestError.With(
			"request failed, http status:%d, business err: %s, trace:%s",
			rsp.StatusCode, string(rsp.RawBody), traceId)
	}

	return &ProxyRsp{
		Data:    string(rsp.RawBody),
		TraceId: traceId,
		Message: Success,
	}, nil
}

// agentTokenFetcher returns an auth.AgentTokenFetcher that refreshes the agent
// token by calling /v1/cli/refresh-sub-agent-token under the master-account
// identity (reusing requestProxy so the master access_token is valid).
func agentTokenFetcher(tmeet *internal.Tmeet) auth.AgentTokenFetcher {
	return func(ctx context.Context, agentRk string) (*auth.AgentTokenData, error) {
		agentCfg, err := config.GetAgentAccountConfig()
		if err != nil {
			return nil, err
		}
		if agentCfg == nil || agentCfg.AgentOpenId == "" {
			return nil, exception.AgentNotFoundError
		}
		body := map[string]interface{}{
			"operator_id":      tmeet.UserConfig.OpenId,
			"operator_id_type": agentOperatorIdTypeOpenId,
			"agent_id":         agentCfg.AgentOpenId,
			"agent_rk":         agentRk,
		}
		rsp, err := requestProxy(ctx, http.MethodPost, tmeet, &thttp.Request{
			ApiURI: "/v1/cli/refresh-sub-agent-token",
			Body:   body,
		})
		if err != nil {
			return nil, err
		}
		data := &auth.AgentTokenData{}
		if err = json.Unmarshal([]byte(rsp.Data), data); err != nil {
			return nil, exception.RefreshAgentTokenFailedError.With(
				"decode refresh sub agent token response failed: %v", err)
		}
		return data, nil
	}
}
