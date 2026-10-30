// prefix.go — install-prefix inference and env-manipulation helpers.
//
// Pure utility functions used by RunNpmInstall to make `npm install -g`
// land in the same directory tree that already owns the running tmeet
// binary, regardless of whichever ambient npm is on PATH.  See
// DeriveInstallPrefix's doc for the motivating workbuddy / nvm / fnm
// problem.

package upgrade

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// pkgDirEnvVar carries the authoritative "package root" path that the
// Node shim (scripts/tmeet.js) injects on every launch.  See that
// file's execFileSync call site for the producer side.  When set it
// trumps the exe-path walk below because it survives the sandbox
// fallback that copies the binary to os.tmpdir() — in that case
// os.Executable() points into tmpdir and no amount of walking will
// find a node_modules ancestor.
const pkgDirEnvVar = "TMEET_NPM_PACKAGE_DIR"

// DeriveInstallPrefix infers the npm global prefix that owns the
// current CLI install without trusting the ambient npm's own default.
// This is the key to handling vendored Node managers (nvm, fnm,
// workbuddy) that physically separate the Node runtime from the
// global package tree: with those, `npm prefix -g` reports the runtime
// location, not where tmeet actually lives.
//
// Two signals are considered, in priority order:
//
//  1. The TMEET_NPM_PACKAGE_DIR env var set by scripts/tmeet.js.
//     This is the shim's own view of its package root — authoritative,
//     and robust to the "binary was copied to tmpdir" sandbox branch.
//  2. exePath itself: walk upwards until a "node_modules" segment
//     appears, then apply the npm layout rules (POSIX: parent must be
//     "lib"; Windows: parent is the prefix directly).
//
// Returns "" when neither signal yields a usable prefix (e.g. a dev
// `go run` build, a user who hand-copied the binary to /tmp, or a
// local project node_modules).  An empty result tells the caller to
// fall back to the ambient npm's default instead of forcing a wrong
// prefix.
func DeriveInstallPrefix(exePath string) string {
	// Layer 1: shim-injected env var.  Preferred because it survives
	// the ensureExecutable tmpdir fallback inside scripts/tmeet.js.
	if pkgDir := os.Getenv(pkgDirEnvVar); pkgDir != "" {
		if prefix := walkUpToPrefix(pkgDir); prefix != "" {
			return prefix
		}
	}
	// Layer 2: exe path walk.  Covers the "binary launched without
	// the shim" case (e.g. someone invoked the raw Go binary
	// directly, or installed via a mechanism that doesn't preserve
	// the env var).
	if exePath == "" {
		return ""
	}
	return walkUpToPrefix(exePath)
}

// walkUpToPrefix climbs start's ancestor chain until it hits a
// directory literally named "node_modules", then returns the npm
// prefix that owns it.
//
// Layout the walk assumes npm produced:
//   - POSIX:   <prefix>/lib/node_modules/<scope>/<pkg>/...   →  "lib" is required
//   - Windows: <prefix>\node_modules\<scope>\<pkg>\...       →  no "lib" layer
//
// The first node_modules we encounter is the one that owns our
// package — any deeper node_modules would be a transitive dep of our
// own package, which doesn't change the prefix answer.  The POSIX
// "lib" guard rules out local project checkouts whose node_modules
// sits directly under the project root; without it we would wrongly
// treat a dev workspace as a global install prefix.
//
// Returns "" when no node_modules is found, when the POSIX "lib"
// guard fails, or when the derived prefix degenerates to filesystem
// root.
func walkUpToPrefix(start string) string {
	cur := filepath.Clean(start)
	for {
		if filepath.Base(cur) == "node_modules" {
			parent := filepath.Dir(cur)
			if parent == "." || parent == string(filepath.Separator) {
				return ""
			}
			if runtime.GOOS == "windows" {
				// Windows: node_modules's parent IS the prefix.
				return parent
			}
			// POSIX: node_modules must live under a "lib" directory
			// to look like an npm global install — this rejects
			// project-local node_modules trees that happen to be in
			// the path.
			if filepath.Base(parent) != "lib" {
				return ""
			}
			prefix := filepath.Dir(parent)
			if prefix == "." || prefix == string(filepath.Separator) {
				return ""
			}
			return prefix
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			// Reached filesystem root without finding node_modules.
			return ""
		}
		cur = parent
	}
}

// IsVoltaEnv reports whether the process is running inside a Volta-
// managed Node environment.  Volta owns the full lifecycle of global
// packages: it shims every bin and refuses to let other installers
// touch its managed tools.  The cheapest reliable signal is the
// VOLTA_HOME env var that Volta's setup script exports; we deliberately
// do NOT walk PATH or inspect binaries, because that would be slow and
// easy to false-positive on tools that merely coexist with Volta.
func IsVoltaEnv() bool {
	return os.Getenv("VOLTA_HOME") != ""
}

// WithEnvOverride returns a copy of env with key set to value: any
// existing "key=..." entries are removed (case-insensitive on Windows,
// case-sensitive elsewhere) and a single "key=value" is appended.
// Keeps RunNpmInstall's env-mutating path concise and easy to
// unit-test.
func WithEnvOverride(env []string, key, value string) []string {
	out := make([]string, 0, len(env)+1)
	keyEq := key + "="
	for _, e := range env {
		if envKeyMatches(e, keyEq) {
			continue
		}
		out = append(out, e)
	}
	return append(out, keyEq+value)
}

// envKeyMatches reports whether env entry e has the given "key="
// prefix.  Environment variable names are case-insensitive on Windows
// and case-sensitive on POSIX, matching how child processes resolve
// them.
//
// Unexported because callers outside this package should not care
// about the comparison nuance; WithEnvOverride handles it.
func envKeyMatches(e, keyEq string) bool {
	if len(e) < len(keyEq) {
		return false
	}
	if runtime.GOOS == "windows" {
		return strings.EqualFold(e[:len(keyEq)], keyEq)
	}
	return e[:len(keyEq)] == keyEq
}
