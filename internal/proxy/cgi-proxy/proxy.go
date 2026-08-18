package cgi_proxy

import (
	"context"
	"encoding/json"
	"net/http"
	"tmeet/internal"
	"tmeet/internal/core/thttp"
	"tmeet/internal/exception"
)

type ProxyRsp[T any] struct {
	Code    int32  `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
	Nonce   string `json:"nonce,omitempty"`
	Data    T      `json:"data"`
}

func RequestProxy[T any](ctx context.Context, method string, tmeet *internal.Tmeet, req *thttp.Request) (*ProxyRsp[T], error) {
	var rsp *thttp.Response
	var err error
	switch method {
	case http.MethodGet:
		rsp, err = tmeet.CGIClient.Get(ctx, req,
			thttp.WithRequestAuthenticator(thttp.DefaultJsonAuthenticator))
	case http.MethodPost:
		rsp, err = tmeet.CGIClient.Post(ctx, req,
			thttp.WithRequestAuthenticator(thttp.DefaultJsonAuthenticator))
	default:
		return nil, exception.InvalidNormalApiMethodError
	}

	if err != nil {
		return nil, exception.NetworkError
	}
	if rsp.StatusCode != http.StatusOK {
		return nil, exception.NetworkError.With("request failed, http status:%d", rsp.StatusCode)
	}

	var traceId string
	if rsp.Header != nil {
		traceId = rsp.Header.Get("Gw-Trace-Id")
	}

	var proxyRsp ProxyRsp[T]
	err = json.Unmarshal(rsp.RawBody, &proxyRsp)
	if err != nil {
		return nil, exception.ResponseDecodeError
	}
	proxyRsp.Nonce = traceId
	return &proxyRsp, nil
}
