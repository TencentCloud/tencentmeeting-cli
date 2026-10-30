// npm.go — invoking the local `npm install -g <target>` subprocess
// with timeouts, a heartbeat tick, env-prefix injection, and
// human-friendly failure diagnostics.
//
// Decoupled from cobra: callers hand in a Logf for the heartbeat and
// an existing context for cancellation.  The subprocess inherits
// os.Environ(), optionally overridden with a single npm_config_prefix
// entry; the rest of the environment (PATH, HTTP_PROXY, ...) stays
// intact so corporate egress / mirror / cert setups keep working.

package upgrade

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"tmeet/internal/exception"
)

// NpmInstallTimeout bounds the `npm install -g <tgz>` subprocess.
// Includes package unpack + postinstall script execution.  10m is
// deliberately generous: postinstall scripts may still hit the
// network, and we would rather wait than fail a slow-but-successful
// install.  The ceiling keeps a hung call from pinning the upgrade
// lock forever.
const NpmInstallTimeout = 10 * time.Minute

// NpmHeartbeatInterval controls how often the "still installing" tick
// is printed to logf while `npm install -g` is running.  15s balances
// "user does not think it is stuck" against "not spammy on fast
// machines".
const NpmHeartbeatInterval = 15 * time.Second

// RunNpmInstall executes `npm install -g <target>` with a heartbeat
// goroutine ticking through logf so the subprocess does not appear
// stuck.  target is either a local tarball path or an npm coordinate
// like "@tencentcloud/tmeet@1.2.3" — npm's install resolves both forms.
//
// When prefix is non-empty we inject it as npm_config_prefix in the
// subprocess environment so a stray ~/.npmrc or ambient
// NPM_CONFIG_PREFIX cannot redirect the install at the last second.
// Empty prefix leaves the environment untouched so wrappers like Volta
// or workbuddy keep whatever they already set up for us.
//
// Captures stdout/stderr so a successful install stays quiet (the
// JSON response in the caller is the real contract); on failure,
// surfaces the tail of npm's output plus any actionable hint we can
// derive from it, wrapped in a single UpdateNpmInstallError.
func RunNpmInstall(ctx context.Context, logf Logf, npmPath, target, prefix string) error {
	ictx, icancel := context.WithTimeout(ctx, NpmInstallTimeout)
	defer icancel()

	installStart := time.Now()
	stopHeartbeat := StartHeartbeat(ictx, logf, installStart)

	var stdout, stderr bytes.Buffer
	nc := exec.CommandContext(ictx, npmPath, "install", "-g", target)
	nc.Stdout = &stdout
	nc.Stderr = &stderr
	if prefix != "" {
		// Replace, not append — if npm_config_prefix is already in the
		// inherited env it would otherwise take precedence over our
		// explicit value.  WithEnvOverride does the de-dup for us.
		nc.Env = WithEnvOverride(os.Environ(), "npm_config_prefix", prefix)
	}
	runErr := nc.Run()
	stopHeartbeat()
	if ictx.Err() == context.DeadlineExceeded {
		return exception.UpdateNpmInstallError.With("npm install timed out after %s", NpmInstallTimeout)
	}
	if runErr != nil {
		combined := stdout.String() + stderr.String()
		return exception.UpdateNpmInstallError.With(
			"npm install failed: %v\n%s%s",
			runErr,
			InstallFailureHint(combined),
			TailOutput(combined, 2000),
		)
	}
	if logf != nil {
		logf("Install finished in %s.", time.Since(installStart).Round(time.Second))
	}
	return nil
}

// StartHeartbeat prints a "still running" line through logf every
// NpmHeartbeatInterval until the returned stop() is called or ctx is
// cancelled.  The caller must invoke stop() exactly once
// (defer-friendly) to guarantee the goroutine exits before the
// function returns.  Pass logf=nil to silence the ticks while still
// using the goroutine as a lifecycle hook.
func StartHeartbeat(ctx context.Context, logf Logf, start time.Time) func() {
	done := make(chan struct{})
	var once sync.Once
	stop := func() { once.Do(func() { close(done) }) }
	go func() {
		t := time.NewTicker(NpmHeartbeatInterval)
		defer t.Stop()
		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				return
			case <-t.C:
				if logf != nil {
					logf("  ... still installing (%s elapsed)",
						time.Since(start).Round(time.Second))
				}
			}
		}
	}()
	return stop
}

// InstallFailureHint maps common npm failure signatures to a one-line
// actionable hint.  Empty return means we have nothing useful to add
// beyond the raw stderr.  Order matters: more specific causes first.
func InstallFailureHint(combined string) string {
	switch {
	case strings.Contains(combined, "EACCES"):
		if runtime.GOOS == "windows" {
			return "hint: permission denied; try running the terminal as Administrator\n"
		}
		return "hint: permission denied; try `sudo tmeet update run`, or configure a user-writable npm prefix (https://docs.npmjs.com/resolving-eacces-permissions-errors)\n"
	case strings.Contains(combined, "EBUSY"), strings.Contains(combined, "EPERM"):
		// Windows: the running tmeet.exe is locked by the OS.  The
		// published npm package's postinstall is expected to rename
		// exe→.old before writing; if we land here it means that
		// mechanism is missing or already failed.  Point the user at
		// the manual workaround.
		return "hint: the running tmeet.exe may be locked by the OS; close other tmeet processes and retry\n"
	case strings.Contains(combined, "ENOTEMPTY"):
		return "hint: the npm global prefix may be in an inconsistent state; retry, or reinstall with `npm i -g @tencentcloud/tmeet@latest`\n"
	}
	return ""
}

// TailOutput returns the last `maxLen` runes of s, prefixed with a
// header so the user can tell what they are looking at.  Keeping only
// the tail is deliberate: npm's stderr can be hundreds of lines on
// failure (deprecation warnings, retried HTTP calls) and the
// actionable error is almost always at the end.
func TailOutput(s string, maxLen int) string {
	if s == "" {
		return ""
	}
	r := []rune(s)
	if len(r) > maxLen {
		s = "...(truncated)...\n" + string(r[len(r)-maxLen:])
	}
	return "npm output:\n" + s
}
