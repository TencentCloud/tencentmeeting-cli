// factory.go — Source selection logic for the bus daemon.
//
// Single construction entry point: Build(params).
//
// Build takes credentials + deviceID and returns either:
//
//   - MockSource — when Token is empty (no logged-in user).  This is a
//     safety rail: an accidentally-published `tmeet event _bus` would
//     otherwise spew unauthenticated requests at the production gateway
//     and risk getting the user's IP soft-banned.
//   - WSSource  — wired against core.GetWSSEndpoint() with the standard
//     `wss://<host>/wemeet-socket/mercury-wss-cli/connection` path.  The
//     URL is NOT overridable via environment variables on purpose: the
//     gateway is part of the contract, not a configuration knob, and
//     allowing env-injected URLs invites accidental traffic to staging /
//     a malicious endpoint via a stray shell export.
//
// Tests that need a non-production WSS server should construct a
// *WSSource directly with their own URL — see wssource_test.go for the
// pattern.

package source

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"tmeet/internal/common"
	"tmeet/internal/core"
	tlog "tmeet/internal/log"
)

// WSS connection coordinates.  Pinned in source so the bus daemon has
// no way to redirect itself: the gateway path is part of the wire
// protocol contract, not a deployment toggle.
const (
	wssScheme = "wss"
	wssPath   = "/wemeet-socket/mercury-wss-cli/connection"
)

// Params bundles the inputs Build needs from the bus daemon.
//
// Empty Token / OpenID / MachineID is allowed (Build returns a
// MockSource in that case so a not-logged-in development run doesn't
// accidentally shoot empty credentials at production).
type Params struct {
	Token        string
	OpenID       string
	MachineID    string
	MockInterval time.Duration // forwarded to MockSource
	Logger       *tlog.Logger  // nil-tolerated; *tlog.Logger has nil-safe methods

	// TokenProvider, when non-nil, is invoked by WSSource before each
	// (re)connect and before each heartbeat to obtain the live
	// access-token.  The bus daemon wires this to TmeetAuth.RefreshToken
	// so a token rotation that happens while the WS session is alive
	// is propagated to the gateway via AuthRefreshReq instead of
	// silently expiring the session.
	//
	// Optional: when nil the WSSource keeps the legacy behaviour
	// (token frozen at construction).  Tests typically leave this
	// unset.
	TokenProvider func(ctx context.Context) (string, error)

	// OnAuthFailed mirrors WSSource.OnAuthFailed — invoked when the
	// gateway rejected AuthBind at the envelope layer (Head.Status != 0,
	// e.g. 10006).  The bus daemon wires this to
	// config.ClearUserConfigUnResource so a token the gateway has
	// declared dead is wiped from the keychain immediately, rather
	// than waiting for the next interactive command to discover it.
	// Optional; ignored when nil or when Build returned a MockSource.
	OnAuthFailed func(ctx context.Context, code int, err error)
}

// Build selects an event Source from explicit parameters.
//
// Selection rules:
//
//   - Empty Token (no logged-in user) ⇒ MockSource.
//   - Otherwise ⇒ WSSource against `wss://<core.GetWSSEndpoint()>/<wssPath>`,
//     configured with the supplied token / openID / cliUniqID and
//     canonical timeouts (handshake / read / heartbeat / backoff
//     defaults applied by WSSource itself).
//
// We deliberately do NOT consult os.Getenv here.  See the package
// header for the reasoning.
func Build(p Params) Source {
	// Lifecycle messages emitted from a one-shot setup function; no caller
	// ctx to inherit, so we use context.Background() purely as the traceID
	// carrier (which here is empty).  *tlog.Logger.Infof is nil-receiver
	// safe, no need to guard p.Logger.
	ctx := context.Background()
	if strings.TrimSpace(p.Token) == "" {
		p.Logger.Infof(ctx, "event source: using MockSource (no token; user not logged in)")
		return &MockSource{Interval: p.MockInterval}
	}
	url := fmt.Sprintf("%s://%s%s", wssScheme, core.GetWSSEndpoint(), wssPath)
	p.Logger.Infof(ctx, "event source: using WSSource url=<redacted>")
	return &WSSource{
		URL:           url,
		Token:         p.Token,
		OpenID:        p.OpenID,
		CLIUniqID:     common.BuildUniqueID(p.OpenID, p.MachineID),
		Headers:       http.Header{},
		TokenProvider: p.TokenProvider,
		OnAuthFailed:  p.OnAuthFailed,
		Logger:        p.Logger,
		// Cap reconnect attempts so a permanently-unreachable gateway
		// (e.g. dead refresh-token, sustained DNS / TLS failure) doesn't
		// keep the bus daemon spinning forever.  Thirty back-to-back
		// failures with the WSSource exponential backoff (1s→30s cap)
		// works out to ~18 minutes of wall-clock retries before Run()
		// gives up and the daemon exits, which the supervising shell /
		// systemd unit can then surface to the user.  A runOnce that
		// stays connected for >60s resets the counter, so this only
		// fires for the pathological "never made it past handshake"
		// case.
		MaxConsecutiveFailures: 30,
	}
}
