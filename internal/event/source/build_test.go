// build_test.go — coverage for the production Build entry point.
//
// We pin three behaviours:
//
//  1. Empty Token returns MockSource (safety rail: never dial production
//     unauth).
//  2. Empty / whitespace-only Token also routes to MockSource (the
//     trim is a guard against config corruption that leaves a blank
//     string in the keychain).
//  3. Non-empty Token returns a WSSource whose URL is composed from
//     core.GetWSSEndpoint() + the canonical wss path.  We assert the
//     full composed URL rather than the host alone so a future
//     refactor that drops the path silently breaks this test.

package source

import (
	"fmt"
	"testing"
	"time"

	"tmeet/internal/core"
)

func TestBuild_NoTokenReturnsMockSource(t *testing.T) {
	src := Build(Params{
		Token:        "",
		OpenID:       "open-1",
		MachineID:    "dev",
		MockInterval: time.Second,
	})
	if _, ok := src.(*MockSource); !ok {
		t.Errorf("expected *MockSource, got %T", src)
	}
}

func TestBuild_BlankTokenReturnsMockSource(t *testing.T) {
	// Whitespace-only token is treated as empty: we trim before the
	// nil-check so a corrupted config can't slip an unauth request
	// past the safety rail.
	src := Build(Params{
		Token:        "   ",
		OpenID:       "open-1",
		MachineID:    "dev",
		MockInterval: time.Second,
	})
	if _, ok := src.(*MockSource); !ok {
		t.Errorf("expected *MockSource for blank token, got %T", src)
	}
}

func TestBuild_NonEmptyTokenReturnsWSSource(t *testing.T) {
	src := Build(Params{
		Token:     "tok",
		OpenID:    "open-1",
		MachineID: "dev",
	})
	wss, ok := src.(*WSSource)
	if !ok {
		t.Fatalf("expected *WSSource, got %T", src)
	}
	wantUniq := "open-1*dev"
	if wss.Token != "tok" || wss.OpenID != "open-1" || wss.CLIUniqID != wantUniq {
		t.Errorf("creds = (%q, %q, %q), want (tok, open-1, %s)",
			wss.Token, wss.OpenID, wss.CLIUniqID, wantUniq)
	}
	// URL is composed wss://<host><path>; both pieces must be present.
	want := fmt.Sprintf("%s://%s%s", wssScheme, core.GetWSSEndpoint(), wssPath)
	if wss.URL != want {
		t.Errorf("URL = %q, want %q", wss.URL, want)
	}
}
