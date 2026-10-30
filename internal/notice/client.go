package notice

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"tmeet/internal"
	"tmeet/internal/core/thttp"
	restProxy "tmeet/internal/proxy/rest-proxy"
)

// versionInfoURI is the REST API fetched once every 24h to drive the
// `_notice.update` payload. Server-side identifier:
// `open_platform-GetCliVersionInfo`.
const versionInfoURI = "/v1/cli/version-info"

// errNoActiveUser is returned by fetchFromAPI when the caller has no OpenId
// available. It is a package-private sentinel; Build only cares that the
// call failed, not why.
var errNoActiveUser = errors.New("notice: no active user")

// fetchFromAPI calls /v1/cli/version-info and returns a decoded CachedPayload
// ready to be persisted. Returns (nil, err) on any transport or decoding
// failure; the caller must NOT write the cache in that case so a transient
// failure does not get frozen into the 24h window.
//
// restProxy.RequestProxy already handles:
//   - OAuth token refresh (+ graceful token-expiry propagation),
//   - the mandatory Tmeet-Cli-Ver / Tmeet-Open-Source / trace headers,
//   - retry with backoff on transient network errors,
//   - unwrapping the proxy envelope so what we see here is the raw server
//     response body as a string.
//
// A 1-second deadline is applied by the caller (Build) via context, not
// here, so unit tests can plug in their own ctx.
func fetchFromAPI(ctx context.Context, tmeet *internal.Tmeet) (*CachedPayload, error) {
	if tmeet == nil || tmeet.UserConfig == nil || tmeet.UserConfig.OpenId == "" {
		// No active user → no way to form the operator_id query. Treat as
		// a soft failure so Build simply skips the notice; the business
		// command itself is unaffected because this path never bubbles up
		// to the subcommand's RunE.
		return nil, errNoActiveUser
	}

	req := &thttp.Request{
		ApiURI: versionInfoURI,
		QueryParams: thttp.QueryParams{
			"operator_id":      []string{tmeet.UserConfig.OpenId},
			"operator_id_type": []string{"2"}, // 2 = open_id
		},
	}

	rsp, err := restProxy.RequestProxy(ctx, http.MethodGet, tmeet, req)
	if err != nil {
		return nil, err
	}

	var p CachedPayload
	if err := json.Unmarshal([]byte(rsp.Data), &p); err != nil {
		return nil, err
	}
	return &p, nil
}
