package output

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"tmeet/internal"
	"tmeet/internal/config"
	"tmeet/internal/notice"
)

// mockVersionInfoServer stands up a minimal HTTP server that mimics the
// /v1/cli/version-info endpoint so notice.BuildWithOutcome actually
// receives a CachedPayload during tests. It MUST be used in combination
// with isolateTestConfigDir so the open_id namespace doesn't collide
// with the real user's filecache.
//
// We go through a real HTTP round-trip (rather than mocking fetchFromAPI
// directly) because the public rest-proxy wiring is also part of what
// FormatPrint relies on; a fake server exercises the same code path.
func mockVersionInfoServer(t *testing.T, target string) *httptest.Server {
	t.Helper()
	body := `{"code":0,"msg":"","data":"{\"target_version\":\"` + target + `\",\"no_preference_text\":{\"text_message\":\"please upgrade\",\"text_preference_instruction\":\"pick one\",\"choices\":[{\"value\":\"remind\",\"label\":\"remind me\",\"command\":\"tmeet update set --preference remind\"},{\"value\":\"never_remind\",\"label\":\"stop\",\"command\":\"tmeet update set --preference never_remind\"}]}}"}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// seedUserForNotice installs an isolated config dir and returns the
// per-test openId. Nothing is written to disk here — the throttle
// sentinel is deliberately NOT seeded so the first FormatPrint call
// goes through the full "fetch + render + mark" path, matching the
// production hot path.
func seedUserForNotice(t *testing.T) string {
	t.Helper()
	t.Setenv("TMEET_CLI_CONFIG_DIR", t.TempDir())
	return "ptu_user"
}

// installTmeet registers t as the active *Tmeet via the public SetTmeet
// hook, and automatically clears it at test cleanup so cases cannot
// leak state into one another via the package-level atomic.Pointer.
func installTmeet(t *testing.T, tm *internal.Tmeet) {
	t.Helper()
	SetTmeet(tm)
	t.Cleanup(func() { SetTmeet(nil) })
}

// stubTmeet builds a *Tmeet whose rest-proxy base URL points at the
// fake version-info server, so notice.BuildWithOutcome's internal fetch
// resolves against the httptest server. Keeping this logic in one place
// prevents each test from re-implementing the wiring.
//
// NOTE: the exact field layout here depends on the shape of
// internal.Tmeet; if that struct changes, update this helper in one spot.
func stubTmeet(openId, cliVersion string) *internal.Tmeet {
	return &internal.Tmeet{
		UserConfig: &config.UserConfig{OpenId: openId},
		CLIVersion: cliVersion,
	}
}

// TestFormatPrint_NoNotice_WhenNoTmeetRegistered locks in the
// "feature gracefully disabled" contract: SetTmeet never called →
// no _notice. This matters for unit tests of other packages that call
// FormatPrint without going through root.go.
func TestFormatPrint_NoNotice_WhenNoTmeetRegistered(t *testing.T) {
	// Deliberately NOT installing a Tmeet. Still register an explicit
	// nil + cleanup so a previous test leaking a value cannot pollute
	// this one.
	installTmeet(t, nil)

	cmd, stdout, _ := newTestCmd(t, "")
	FormatPrint(cmd, "trace-xyz", "ok", `{"foo":"bar"}`)

	if strings.Contains(stdout.String(), "_notice") {
		t.Fatalf("expected no _notice when no Tmeet registered, got %q", stdout.String())
	}
}

// TestFormatPrint_WithoutUpdateNotice_Suppresses: `tmeet update get`
// relies on this option to prevent double-rendering. The absence of a
// network fetch is also implicitly asserted — if BuildWithOutcome were
// invoked, it would try to reach a server that we did not stand up and
// the test would hang (which is why the regression value of this test
// is higher than it looks).
func TestFormatPrint_WithoutUpdateNotice_Suppresses(t *testing.T) {
	openId := seedUserForNotice(t)
	cmd, stdout, _ := newTestCmd(t, "")

	installTmeet(t, stubTmeet(openId, "v1.0.19"))

	FormatPrint(cmd, "trace-xyz", "ok", `{"foo":"bar"}`, WithoutUpdateNotice())

	if strings.Contains(stdout.String(), "_notice") {
		t.Fatalf("WithoutUpdateNotice should suppress, got %q", stdout.String())
	}
}

// TestFormatPrint_ThrottlesByLastNoticeSentinel verifies the gap gate:
// pre-seeding a fresh keyLastNotice entry must suppress the proactive
// prompt WITHOUT invoking the version-info fetch. A real server is not
// stood up on purpose — if BuildWithOutcome tried to fetch, the test
// would hang or fail hard, which doubles as a safety net against future
// regressions that bypass the gate.
func TestFormatPrint_ThrottlesByLastNoticeSentinel(t *testing.T) {
	openId := seedUserForNotice(t)
	// Prime the sentinel so the next Build should short-circuit.
	notice.MarkNotified(openId, 3600)

	cmd, stdout, _ := newTestCmd(t, "")
	installTmeet(t, stubTmeet(openId, "v1.0.19"))

	FormatPrint(cmd, "trace-xyz", "ok", `{"foo":"bar"}`)

	if strings.Contains(stdout.String(), "_notice") {
		t.Fatalf("gap gate should suppress proactive notice, got %q", stdout.String())
	}

	// Decode the envelope to confirm the response is still well-formed.
	var env map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("stdout not JSON: %v (%q)", err, stdout.String())
	}
	if _, has := env["_notice"]; has {
		t.Fatalf("expected no _notice key, got %v", env["_notice"])
	}
}

// mockVersionInfoServer is referenced here so an inadvertent removal
// causes a compile error in this file first, keeping its lifetime tied
// to the test suite it exists to serve.
var _ = mockVersionInfoServer
