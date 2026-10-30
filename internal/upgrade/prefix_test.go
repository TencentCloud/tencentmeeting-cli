package upgrade

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// -----------------------------------------------------------------------------
// DeriveInstallPrefix
// -----------------------------------------------------------------------------

func Test_DeriveInstallPrefix(t *testing.T) {
	// Isolate every subtest from any ambient TMEET_NPM_PACKAGE_DIR
	// the dev may have exported (and from each other's Setenv's).
	// t.Setenv("", "") would be nicer but Go doesn't support it;
	// an explicit empty value does the job because os.Getenv treats
	// unset and empty identically for our purposes.
	clearPkgDir := func(t *testing.T) { t.Helper(); t.Setenv("TMEET_NPM_PACKAGE_DIR", "") }

	t.Run("empty exe path and no env var", func(t *testing.T) {
		clearPkgDir(t)
		if got := DeriveInstallPrefix(""); got != "" {
			t.Fatalf("want empty for empty input, got %q", got)
		}
	})

	if runtime.GOOS == "windows" {
		t.Run("windows: exe under <prefix>\\node_modules", func(t *testing.T) {
			clearPkgDir(t)
			// Real npm layout on Windows: <prefix>\node_modules\@scope\pkg\dist\binary.exe
			// Prefix is simply the parent of node_modules — no "lib" layer.
			exe := filepath.Join(`C:\Users\x\AppData\Roaming\npm`,
				"node_modules", "@tencentcloud", "tmeet", "dist", "tmeet-Windows-x86_64.exe")
			want := `C:\Users\x\AppData\Roaming\npm`
			if got := DeriveInstallPrefix(exe); got != want {
				t.Fatalf("got %q, want %q", got, want)
			}
		})
		t.Run("windows: env var overrides exe path", func(t *testing.T) {
			t.Setenv("TMEET_NPM_PACKAGE_DIR",
				filepath.Join(`C:\nvm\v18`, "node_modules", "@tencentcloud", "tmeet"))
			// exe lives in tmpdir (sandbox fallback) — walk would fail.
			exe := filepath.Join(`C:\Users\x\AppData\Local\Temp`, "tmeet-cache", "tmeet-Windows-x86_64.exe")
			want := `C:\nvm\v18`
			if got := DeriveInstallPrefix(exe); got != want {
				t.Fatalf("got %q, want %q", got, want)
			}
		})
		return
	}

	// --- POSIX ------------------------------------------------------

	t.Run("posix nvm layout via exe path", func(t *testing.T) {
		clearPkgDir(t)
		exe := "/Users/x/.nvm/versions/node/v18.19.0/lib/node_modules/@tencentcloud/tmeet/dist/tmeet-macOS-AppleSilicon"
		want := "/Users/x/.nvm/versions/node/v18.19.0"
		if got := DeriveInstallPrefix(exe); got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})

	t.Run("posix workbuddy layout via exe path", func(t *testing.T) {
		// Regression case for the vendored-Node-manager bug:
		// workbuddy keeps npm and the CLI packages in sibling trees,
		// but the global package still follows the standard
		// <prefix>/lib/node_modules/<scope>/<pkg> layout.
		clearPkgDir(t)
		exe := "/Users/x/.workbuddy/binaries/node/cli-connector-packages/lib/node_modules/@tencentcloud/tmeet/dist/tmeet-Linux-x86_64"
		want := "/Users/x/.workbuddy/binaries/node/cli-connector-packages"
		if got := DeriveInstallPrefix(exe); got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})

	t.Run("posix system layout via exe path", func(t *testing.T) {
		clearPkgDir(t)
		exe := "/usr/local/lib/node_modules/@tencentcloud/tmeet/dist/tmeet-macOS-Intel"
		if got := DeriveInstallPrefix(exe); got != "/usr/local" {
			t.Fatalf("got %q", got)
		}
	})

	t.Run("posix brew layout via exe path", func(t *testing.T) {
		clearPkgDir(t)
		exe := "/opt/homebrew/lib/node_modules/@tencentcloud/tmeet/dist/tmeet-macOS-AppleSilicon"
		if got := DeriveInstallPrefix(exe); got != "/opt/homebrew" {
			t.Fatalf("got %q", got)
		}
	})

	t.Run("env var wins over exe path", func(t *testing.T) {
		// Simulates the sandbox branch of scripts/tmeet.js that
		// copies the binary to os.tmpdir() before launching it:
		// exe path has no node_modules ancestor, but the shim
		// still gave us the real package root via env var.
		t.Setenv("TMEET_NPM_PACKAGE_DIR",
			"/usr/local/lib/node_modules/@tencentcloud/tmeet")
		exe := "/tmp/tmeet-cache/tmeet-Linux-x86_64"
		if got := DeriveInstallPrefix(exe); got != "/usr/local" {
			t.Fatalf("got %q, want /usr/local", got)
		}
	})

	t.Run("env var takes priority even when exe path is also derivable", func(t *testing.T) {
		// If the user has somehow ended up with two different trees
		// in play (shim from tree A, binary from tree B), the shim's
		// view is the authoritative one — it is the one that
		// actually launched us.
		t.Setenv("TMEET_NPM_PACKAGE_DIR",
			"/opt/homebrew/lib/node_modules/@tencentcloud/tmeet")
		exe := "/usr/local/lib/node_modules/@tencentcloud/tmeet/dist/tmeet-macOS-Intel"
		if got := DeriveInstallPrefix(exe); got != "/opt/homebrew" {
			t.Fatalf("got %q, want /opt/homebrew", got)
		}
	})

	t.Run("empty env var falls through to exe path", func(t *testing.T) {
		// An explicitly-empty env var should behave identically to
		// "unset" (standard POSIX semantics).  os.Getenv returns ""
		// in both cases, which is what the implementation checks.
		t.Setenv("TMEET_NPM_PACKAGE_DIR", "")
		exe := "/usr/local/lib/node_modules/@tencentcloud/tmeet/dist/tmeet-macOS-Intel"
		if got := DeriveInstallPrefix(exe); got != "/usr/local" {
			t.Fatalf("got %q", got)
		}
	})

	t.Run("sandbox tmpdir without env var returns empty", func(t *testing.T) {
		// Opposite of the "env var wins" case: no shim env var AND
		// a binary that lives outside any node_modules tree.
		// We'd rather report "unresolved" than guess.
		clearPkgDir(t)
		if got := DeriveInstallPrefix("/tmp/tmeet-cache/tmeet-Linux-x86_64"); got != "" {
			t.Fatalf("want empty, got %q", got)
		}
	})

	t.Run("local project node_modules rejected (parent not 'lib')", func(t *testing.T) {
		// A dev-time binary that happens to be referenced from
		// inside a project's local node_modules — NOT a global
		// install.  The POSIX "lib" guard rejects it so we don't
		// override npm's default with a project path.
		clearPkgDir(t)
		exe := "/Users/x/some-project/node_modules/@tencentcloud/tmeet/dist/tmeet-macOS-AppleSilicon"
		if got := DeriveInstallPrefix(exe); got != "" {
			t.Fatalf("want empty for project-local layout, got %q", got)
		}
	})

	t.Run("non-npm paths return empty", func(t *testing.T) {
		clearPkgDir(t)
		for _, p := range []string{
			"/tmp/tmeet",
			"/opt/my-tool/sbin/tmeet",
			"/home/x/dev/wemeet-cli/dist/tmeet-Linux-x86_64",
		} {
			if got := DeriveInstallPrefix(p); got != "" {
				t.Fatalf("%s: want empty, got %q", p, got)
			}
		}
	})

	t.Run("root-level node_modules returns empty", func(t *testing.T) {
		// "/node_modules/..." would try to derive prefix="/" which
		// is nonsensical as an npm prefix.  Guard against it.
		clearPkgDir(t)
		if got := DeriveInstallPrefix("/node_modules/@scope/pkg/dist/tmeet"); got != "" {
			t.Fatalf("want empty for root-level node_modules, got %q", got)
		}
	})
}

// -----------------------------------------------------------------------------
// IsVoltaEnv
// -----------------------------------------------------------------------------

func Test_IsVoltaEnv(t *testing.T) {
	t.Run("unset", func(t *testing.T) {
		t.Setenv("VOLTA_HOME", "")
		if IsVoltaEnv() {
			t.Fatalf("IsVoltaEnv should be false when VOLTA_HOME is empty")
		}
	})
	t.Run("set", func(t *testing.T) {
		t.Setenv("VOLTA_HOME", "/Users/x/.volta")
		if !IsVoltaEnv() {
			t.Fatalf("IsVoltaEnv should be true when VOLTA_HOME is set")
		}
	})
}

// -----------------------------------------------------------------------------
// WithEnvOverride
// -----------------------------------------------------------------------------

func Test_WithEnvOverride(t *testing.T) {
	t.Run("appends when key absent", func(t *testing.T) {
		got := WithEnvOverride([]string{"FOO=1", "BAR=2"}, "BAZ", "3")
		if len(got) != 3 || got[2] != "BAZ=3" {
			t.Fatalf("unexpected result: %v", got)
		}
	})
	t.Run("replaces when key present", func(t *testing.T) {
		got := WithEnvOverride([]string{"FOO=1", "npm_config_prefix=/old", "BAR=2"},
			"npm_config_prefix", "/new")
		// Order: everything else preserved, override moved to the end.
		if len(got) != 3 {
			t.Fatalf("want 3 entries, got %v", got)
		}
		if got[len(got)-1] != "npm_config_prefix=/new" {
			t.Fatalf("override not at tail: %v", got)
		}
		for _, e := range got[:len(got)-1] {
			if strings.HasPrefix(e, "npm_config_prefix=") {
				t.Fatalf("duplicate key survived: %v", got)
			}
		}
	})
	t.Run("de-dupes when key repeated", func(t *testing.T) {
		got := WithEnvOverride(
			[]string{"K=a", "K=b", "K=c"}, "K", "final")
		if len(got) != 1 || got[0] != "K=final" {
			t.Fatalf("expected single K=final, got %v", got)
		}
	})
	t.Run("empty env", func(t *testing.T) {
		got := WithEnvOverride(nil, "K", "v")
		if len(got) != 1 || got[0] != "K=v" {
			t.Fatalf("unexpected result: %v", got)
		}
	})
}

// -----------------------------------------------------------------------------
// envKeyMatches (unexported — accessible from same-package tests)
// -----------------------------------------------------------------------------

func Test_envKeyMatches(t *testing.T) {
	// Short entry → never a match (prevents slice-out-of-range on
	// the prefix slice inside envKeyMatches).
	if envKeyMatches("K=", "KEY=") {
		t.Fatalf("short entry should not match")
	}
	// Exact prefix.
	if !envKeyMatches("KEY=value", "KEY=") {
		t.Fatalf("exact prefix should match")
	}
	// Non-matching prefix.
	if envKeyMatches("OTHER=value", "KEY=") {
		t.Fatalf("different key should not match")
	}
	// Case sensitivity is platform-dependent: Windows env is
	// case-insensitive, POSIX is case-sensitive.  envKeyMatches
	// follows the child-process resolution rules, so verify both
	// branches we are actually running under.
	gotCaseInsensitive := envKeyMatches("key=value", "KEY=")
	if runtime.GOOS == "windows" {
		if !gotCaseInsensitive {
			t.Fatalf("windows env should be case-insensitive")
		}
	} else {
		if gotCaseInsensitive {
			t.Fatalf("posix env should be case-sensitive")
		}
	}
}
