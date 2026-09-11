package cgi_proxy

import (
	"context"
	"encoding/json"
	"net/http"
	"tmeet/internal"
	"tmeet/internal/core/thttp"
	"tmeet/internal/exception"
	"tmeet/internal/log"
)

type ProxyRsp[T any] struct {
	Code    int32  `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
	Nonce   string `json:"nonce,omitempty"`
	Data    T      `json:"data"`
}

func RequestProxy[T any](ctx context.Context, method string, tmeet *internal.Tmeet, req *thttp.Request) (*ProxyRsp[T], error) {
	rsp, err := doCGIRequest(ctx, method, tmeet, req)
	if err != nil {
		// Network-layer failure only (c.clt.Do returned an error — the
		// request never got a response).  The default transport honours
		// HTTP(S)_PROXY / ALL_PROXY, and a common cause here is a stale proxy
		// inherited from a now-dead parent (e.g. an agent sandbox tore down
		// its proxy sidecar but our long-lived daemon kept the env var).
		// Retry once over a direct-connection client that ignores the proxy
		// env.  A non-200 status is NOT retried here: the server did respond,
		// so the network path is fine and re-sending won't help.  We build a
		// fresh Request so the retry doesn't inherit the authenticators /
		// headers the first attempt already appended.
		log.Warnf(ctx, "cgi-proxy: request %s %s failed via default transport, retrying without proxy: %v",
			method, req.ApiURI, err)
		retryReq := &thttp.Request{
			ApiURI:      req.ApiURI,
			Body:        req.Body,
			PathParams:  req.PathParams,
			QueryParams: req.QueryParams,
		}
		rsp, err = doCGIRequest(ctx, method, tmeet, retryReq, thttp.WithRequestClient(thttp.DefaultNoProxyHttpClient))
		if err != nil {
			return nil, exception.NetworkError
		}
	}

	// Status-code validation runs once on whichever response we ended up
	// with (proxy or direct).  A non-200 is a server-side reject, not a
	// transport problem — no retry.
	if rsp.StatusCode != http.StatusOK {
		log.Errorf(ctx, "cgi-proxy: request %s %s got http status:%d, header:%s",
			method, req.ApiURI, rsp.StatusCode, redactHeader(rsp.Header))
		return nil, exception.NetworkError.With("request failed, http status:%d", rsp.StatusCode)
	}

	var traceId string
	if rsp.Header != nil {
		traceId = rsp.Header.Get("Gw-Trace-Id")
	}

	var proxyRsp ProxyRsp[T]
	if uerr := json.Unmarshal(rsp.RawBody, &proxyRsp); uerr != nil {
		log.Errorf(ctx, "cgi-proxy: request %s %s decode failed: %v, header:%s",
			method, req.ApiURI, uerr, redactHeader(rsp.Header))
		return nil, exception.ResponseDecodeError
	}
	proxyRsp.Nonce = traceId
	return &proxyRsp, nil
}

// doCGIRequest issues a single CGI request.  The returned error represents a
// network-layer failure ONLY (the request never got a response); HTTP status
// validation is the caller's job so it can distinguish "should retry over a
// different transport" (network err) from "server rejected us" (non-200).
// extraOpts lets the caller inject per-request options such as a
// direct-connection client for the no-proxy retry path.
func doCGIRequest(ctx context.Context, method string, tmeet *internal.Tmeet,
	req *thttp.Request, extraOpts ...thttp.RequestOptionFunc) (*thttp.Response, error) {
	opts := append([]thttp.RequestOptionFunc{
		thttp.WithRequestAuthenticator(thttp.DefaultJsonAuthenticator),
	}, extraOpts...)

	var rsp *thttp.Response
	var err error
	switch method {
	case http.MethodGet:
		rsp, err = tmeet.CGIClient.Get(ctx, req, opts...)
	case http.MethodPost:
		rsp, err = tmeet.CGIClient.Post(ctx, req, opts...)
	default:
		return nil, exception.InvalidNormalApiMethodError
	}

	if err != nil {
		log.Errorf(ctx, "cgi-proxy: request %s %s failed: %v", method, req.ApiURI, err)
		return nil, exception.NetworkError
	}
	return rsp, nil
}

// redactHeader formats an http.Header for logging while redacting
// sensitive entries (Set-Cookie, Authorization) that may carry session
// credentials.
func redactHeader(h http.Header) string {
	if len(h) == 0 {
		return "{}"
	}
	sensitive := map[string]bool{
		"Set-Cookie":    true,
		"Authorization": true,
		"Cookie":        true,
	}
	out := make(map[string][]string, len(h))
	for k, v := range h {
		if sensitive[http.CanonicalHeaderKey(k)] {
			out[k] = []string{"[REDACTED]"}
		} else {
			out[k] = v
		}
	}
	b, _ := json.Marshal(out)
	return string(b)
}
