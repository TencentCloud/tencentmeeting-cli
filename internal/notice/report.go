package notice

import (
	"context"
	"net/http"

	"tmeet/internal"
	"tmeet/internal/core/thttp"
	restProxy "tmeet/internal/proxy/rest-proxy"
)

// preferenceReportURI is the REST API that captures the user's choice between
// `remind` and `never_remind`. Server-side identifier:
// `open_platform-ReportCLiUpdatePreference`.
const preferenceReportURI = "/v1/cli/update-preference-report"

// reportPreference uploads the user's just-chosen preference to the server.
//
// Semantics:
//   - value == PreferenceNeverRemind → server-side telemetry key
//     `e#openclaw_mcp_runtime#never_remind_update_set` is incremented and
//     the user is added to the "do not remind" allowlist.
//   - value == PreferenceRemind     → the server removes the user from the
//     "do not remind" allowlist (CLI needs no local branching: same URI,
//     different value).
//
// Failures are non-fatal: the local preference file is already persisted by
// the time this function is called, so callers log-and-continue. We still
// propagate the error so logging can surface the actual cause.
func reportPreference(ctx context.Context, tmeet *internal.Tmeet, value string) error {
	if tmeet == nil || tmeet.UserConfig == nil || tmeet.UserConfig.OpenId == "" {
		return errNoActiveUser
	}

	req := &thttp.Request{
		ApiURI: preferenceReportURI,
		Body: map[string]interface{}{
			"operator_id":       tmeet.UserConfig.OpenId,
			"operator_id_type":  "2", // 2 = open_id
			"update_preference": value,
		},
	}

	// Response body is intentionally ignored — the endpoint defines no
	// body and we rely purely on the status-code / error contract that
	// restProxy enforces.
	_, err := restProxy.RequestProxy(ctx, http.MethodPost, tmeet, req)
	return err
}
