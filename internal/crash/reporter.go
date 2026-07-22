// Package crash provides CLI process crash (panic) reporting to the server.
//
// It reuses the existing REST proxy (internal/proxy/rest-proxy) so that the
// crash report shares the same authentication, retry, and common header logic
// (e.g. Tmeet-Cli-Ver and Tmeet-Trace carry cli_version and cli_cmd, so they
// are intentionally omitted from the request body).
package crash

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"tmeet/internal"
	"tmeet/internal/core/thttp"
	"tmeet/internal/log"
	restProxy "tmeet/internal/proxy/rest-proxy"
)

// crashReportURI is the server endpoint to receive CLI crash reports.
const crashReportURI = "/v1/cli/crash"

// reportTimeout limits the total time spent on a single crash report so
// that a slow network does not delay the CLI exit noticeably.
const reportTimeout = 1 * time.Second

// Report sends a single crash record to the server.
//
// cli_version and cli_cmd are NOT put into the body because the REST proxy
// header (Tmeet-Cli-Ver / Tmeet-Trace) already carries them.
//
// The call is best-effort: any error (including panic inside the report path
// itself) is swallowed so that crash reporting never impacts the primary
// CLI exit flow.
func Report(ctx context.Context, tmeet *internal.Tmeet, errorCode int, crashStack string) {
	// Defence-in-depth: never let the reporter itself take the CLI down.
	defer func() { _ = recover() }()

	if tmeet == nil {
		return
	}

	if tmeet.UserConfig == nil {
		return
	}

	body := map[string]interface{}{
		"operator_id":      tmeet.UserConfig.OpenId,
		"operator_id_type": 2, // openId fixed
		"crash_stack":      crashStack,
		"error_code":       strconv.FormatInt(int64(errorCode), 10),
	}

	// Independent short timeout to avoid making the user wait on crash exit.
	reportCtx, cancel := context.WithTimeout(ctx, reportTimeout)
	defer cancel()

	req := &thttp.Request{ApiURI: crashReportURI, Body: body}
	if _, err := restProxy.RequestProxy(reportCtx, http.MethodPost, tmeet, req); err != nil {
		// Silent degradation: only record locally.
		log.Warnf(ctx, "crash report failed: %v", err)
	}
}
