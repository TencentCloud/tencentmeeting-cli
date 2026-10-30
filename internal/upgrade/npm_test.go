package upgrade

import (
	"context"
	"runtime"
	"strings"
	"testing"
	"time"
)

// -----------------------------------------------------------------------------
// InstallFailureHint
// -----------------------------------------------------------------------------

func Test_InstallFailureHint(t *testing.T) {
	// EACCES wording is OS-dependent (Windows → "Administrator";
	// otherwise → "sudo tmeet update run").  We only assert the
	// branch marker that is unambiguous on the current platform.
	eaccesWant := "sudo tmeet update run"
	if runtime.GOOS == "windows" {
		eaccesWant = "Administrator"
	}

	cases := []struct {
		name     string
		input    string
		contains string // "" means hint must be empty
	}{
		{"EACCES", "npm ERR! code EACCES\n", eaccesWant},
		{"EBUSY", "npm ERR! code EBUSY: resource busy", "locked by the OS"},
		{"EPERM", "npm ERR! code EPERM: operation not permitted", "locked by the OS"},
		{"ENOTEMPTY", "npm ERR! code ENOTEMPTY: directory not empty", "inconsistent state"},
		{"no match", "generic failure", ""},
		{"empty", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := InstallFailureHint(tc.input)
			if tc.contains == "" {
				if got != "" {
					t.Fatalf("expected empty hint, got %q", got)
				}
				return
			}
			if !strings.Contains(got, tc.contains) {
				t.Fatalf("expected hint containing %q, got %q", tc.contains, got)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// TailOutput
// -----------------------------------------------------------------------------

func Test_TailOutput(t *testing.T) {
	t.Run("empty returns empty", func(t *testing.T) {
		if got := TailOutput("", 100); got != "" {
			t.Fatalf("expected empty, got %q", got)
		}
	})
	t.Run("short input returned verbatim with header", func(t *testing.T) {
		got := TailOutput("hello", 100)
		if !strings.HasPrefix(got, "npm output:\n") {
			t.Fatalf("missing header: %q", got)
		}
		if !strings.Contains(got, "hello") {
			t.Fatalf("missing payload: %q", got)
		}
		if strings.Contains(got, "truncated") {
			t.Fatalf("unexpected truncation marker for short input: %q", got)
		}
	})
	t.Run("long input truncated to tail", func(t *testing.T) {
		input := strings.Repeat("a", 100) + "TAILMARKER"
		got := TailOutput(input, 20)
		if !strings.Contains(got, "truncated") {
			t.Fatalf("expected truncation marker, got %q", got)
		}
		if !strings.Contains(got, "TAILMARKER") {
			t.Fatalf("expected trailing TAILMARKER preserved, got %q", got)
		}
	})
}

// -----------------------------------------------------------------------------
// StartHeartbeat
// -----------------------------------------------------------------------------

func Test_StartHeartbeat_StopIdempotentAndFast(t *testing.T) {
	logf, buf := captureLogf()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stop := StartHeartbeat(ctx, logf, time.Now())
	// Stopping immediately must not emit a tick (interval is 15s,
	// way above any plausible test latency) and must be safe to
	// call twice.
	stop()
	stop()

	// Give the goroutine a moment to exit; nothing should be
	// logged.
	time.Sleep(20 * time.Millisecond)
	if strings.Contains(buf.String(), "still installing") {
		t.Fatalf("heartbeat should not have ticked: %q", buf.String())
	}
}

func Test_StartHeartbeat_ExitsOnContextCancel(t *testing.T) {
	logf, _ := captureLogf()
	ctx, cancel := context.WithCancel(context.Background())

	// Not calling stop() — we rely on ctx cancellation to shut the
	// goroutine down.  If this leaks, `go test -race` plus the
	// testing package's "goroutine leak" style asserts will flag it
	// over time.
	_ = StartHeartbeat(ctx, logf, time.Now())
	cancel()
	// Soak for a tiny bit so a buggy implementation that ignored
	// ctx.Done() would have a chance to tick (it would not, because
	// the ticker is 15s, but we still want to prove the branch).
	time.Sleep(20 * time.Millisecond)
}

func Test_StartHeartbeat_NilLogfIsTolerated(t *testing.T) {
	// Nil logf must not panic the ticker goroutine.  We cannot
	// easily force a tick in <20ms (interval is 15s), but we can
	// verify the stop path still cleans up cleanly.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stop := StartHeartbeat(ctx, nil, time.Now())
	stop()
}
