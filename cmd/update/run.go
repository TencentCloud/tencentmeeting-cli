package update

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"tmeet/internal"
	"tmeet/internal/cmdutil"
	middleWare "tmeet/internal/cmdutil/middleware"
	"tmeet/internal/config"
	"tmeet/internal/core/filelock"
	"tmeet/internal/exception"
	"tmeet/internal/log"
	"tmeet/internal/notice"
	"tmeet/internal/output"
	"tmeet/internal/upgrade"

	"github.com/spf13/cobra"
)

// Lock file guarding the "download → npm install" critical section.
// One `tmeet update run` at a time; a second concurrent run on the
// same host fails fast rather than queueing — see RunOptions.Run for
// the rationale.
const updateLockFileName = "update.lock"

// npmPackageName is the public npm package coordinate used when the
// server opts into the registry path by setting target_from_npm=true.
// Kept as a const (rather than config) so a server-side flag flip
// cannot redirect installs to an arbitrary package.
const npmPackageName = "@tencentcloud/tmeet"

// RunOptions holds options for `tmeet update run`.
type RunOptions struct {
	tmeet *internal.Tmeet
}

// newRunCmd downloads the newest CLI tarball and installs it globally
// via npm.  The CDN serves an npm pack-style `.tgz`; running
// `npm install -g` against it exercises the same postinstall path a
// public-registry install would, so Linux / macOS / Windows all take
// one code path.
//
// Why npm instead of in-place binary swap:
//   - The payload is a tarball, not a bare executable; renaming it
//     over the running binary would just move a .tgz into argv[0].
//   - npm already solves the "replace a running executable" problem
//     on Windows via its postinstall shim (rename exe → .old before
//     writing).  Delegating keeps us OS-agnostic without a Win32
//     ReplaceFileW branch.
//   - A user who already has the CLI globally has npm available by
//     definition — the same tool that installed it can replace it.
func newRunCmd(tmeet *internal.Tmeet) *cobra.Command {
	opts := &RunOptions{tmeet: tmeet}
	cmd := &cobra.Command{
		Use:   "run",
		Short: "download and install the latest tmeet CLI",
		RunE: middleWare.Chain(
			opts.Run,
			middleWare.WithApiCmd(cmdutil.StaticApiCmd(cmdutil.ApiCmdUpdateRun)),
		),
	}
	return cmd
}

// cmdLogf adapts output.EventStderr into the upgrade.Logf shape so the
// internal/upgrade package stays decoupled from *cobra.Command.  It is
// the ONE adapter in this file — every other upgrade.* call takes the
// returned closure, keeping the coupling local.
func cmdLogf(cmd *cobra.Command) upgrade.Logf {
	return func(format string, args ...interface{}) {
		output.EventStderr(cmd, format, args...)
	}
}

// Run executes `tmeet update run`.
func (o *RunOptions) Run(cmd *cobra.Command, args []string) error {
	// Enforce single-flight across processes BEFORE doing anything
	// else.  ProcessLock.TryLock is non-blocking: if another
	// `tmeet update run` is in flight we return immediately rather
	// than queueing.  The lock is auto-released when the process
	// exits (even on panic) because flock is tied to the fd and the
	// kernel drops it on exit.
	lockPath := filepath.Join(config.GetConfigDir(), updateLockFileName)
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o700); err != nil {
		return exception.UpdateLockError.With("failed to prepare update lock dir: %v", err)
	}
	plock := filelock.NewProcessLock(lockPath)
	if err := plock.TryLock(); err != nil {
		if errors.Is(err, filelock.ErrHeld) {
			return exception.UpdateLockError.With("another `tmeet update run` is already in progress, please retry later")
		}
		return exception.UpdateLockError.With("failed to acquire update lock: %v", err)
	}
	defer func() { _ = plock.Unlock() }()

	openId := o.tmeet.UserConfig.OpenId

	// No per-call timeout wrapper — the underlying REST client
	// already enforces its own timeout policy, and nesting a
	// shorter one here would just mask legitimately-slow but
	// successful fetches. `update run` is an explicit user action,
	// so we always talk to the server to be certain we install the
	// latest advertised artefact.
	payload, err := notice.FetchFromAPI(cmd.Context(), o.tmeet)
	if err != nil {
		return err
	}

	// Early exits that keep `run` idempotent and user-friendly.
	if payload.TargetVersion == "" {
		return exception.UpdateServerPayloadError.With("server returned no target version — cannot update")
	}
	if !notice.IsNewer(o.tmeet.CLIVersion, payload.TargetVersion) {
		return respondAlreadyLatest(cmd, o.tmeet.CLIVersion, payload.TargetVersion)
	}
	if payload.TargetCDN == "" {
		return exception.UpdateServerPayloadError.With("server returned no download URL — cannot update")
	}

	// Probe npm up front: a clear "npm not found" error here is
	// better than a vague failure after we have already spent 2
	// minutes downloading a tarball we cannot install.
	npmPath, err := exec.LookPath("npm")
	if err != nil {
		return exception.UpdateNpmMissingError.With(
			"npm is required to install the update but was not found in PATH; "+
				"install Node.js (https://nodejs.org) and retry (looked up: %v)", err)
	}

	logf := cmdLogf(cmd)

	// Resolve where `npm install -g` should land so it lands in the
	// SAME install tree that currently owns the running tmeet binary,
	// rather than wherever the ambient npm's default prefix happens
	// to point.  Rationale: with nvm / fnm / vendored Node managers
	// like workbuddy, the npm on PATH is in a completely different
	// directory tree from the one that installed us; without an
	// explicit prefix the upgrade would silently land in the "wrong"
	// tree.
	//
	// Volta is the one case we deliberately bypass: it proxies every
	// bin through a shim, so os.Executable() returns the shim path
	// and the derived prefix would point at Volta's internal cache
	// rather than the npm package layout.  Letting Volta's own
	// install flow run is safer than overriding it.
	exePath, exeErr := os.Executable()
	if exeErr == nil {
		if resolved, resErr := filepath.EvalSymlinks(exePath); resErr == nil {
			exePath = resolved
		}
	}
	var (
		installPrefix string
		prefixSource  string
	)
	switch {
	case upgrade.IsVoltaEnv():
		prefixSource = "volta_skipped"
		logf("info: detected Volta environment; letting the ambient npm pick the install location.")
		logf("      if the upgrade lands in the wrong place, run: volta install %s", npmPackageName)
	case exeErr != nil:
		prefixSource = "unresolved"
		logf("warning: could not resolve current executable (%v); falling back to npm's default prefix.", exeErr)
	default:
		installPrefix = upgrade.DeriveInstallPrefix(exePath)
		if installPrefix == "" {
			prefixSource = "unresolved"
			logf("warning: could not derive install prefix from %s; falling back to npm's default prefix.", exePath)
		} else {
			prefixSource = "derived"
			logf("info: resolved install prefix %s", installPrefix)
		}
	}

	// Install the CLI itself.  Two sources are supported, selected
	// by the server-provided target_from_npm flag:
	//   - true  → `npm install -g @tencentcloud/tmeet@latest` (public
	//     registry; npm handles download, checksum, and global
	//     install all in one shot — no local tarball needed).  We
	//     deliberately pin to `@latest` rather than the server-
	//     reported version: target_from_npm=true means "the latest
	//     published release is the one we want", and `@latest` is
	//     what the registry promotes once a release is live.
	//   - false → download the CDN tarball, verify SHA-256 locally,
	//     then `npm install -g <local.tgz>`.  Used for staged /
	//     internal rollouts that have not been published to the
	//     public registry.
	source := "npm"
	if payload.TargetFromNPM {
		if err := installFromRegistry(cmd, npmPath, installPrefix); err != nil {
			return err
		}
	} else {
		source = "cdn"
		if err := installFromCDN(cmd, npmPath, installPrefix, payload); err != nil {
			return err
		}
	}

	// CLI install succeeded — reset the keyLastNotice sentinel so a
	// stale "update available" nudge from an older gap window does
	// not survive past the install we just performed.  Best-effort:
	// a sentinel delete failure must not fail an otherwise-successful
	// update.
	_ = notice.ResetNoticeGate(openId)

	// Skills install is best-effort: a failed skills refresh after a
	// successful CLI update should not fail the whole operation — the
	// user already got what they came for, and `tmeet update run` is
	// safe to retry for just the skills piece.  A warning on stderr
	// is enough; the JSON stdout contract still says "updated".
	skillStatus := "skipped"
	skillPath := ""
	if payload.TargetSkillCDN != "" {
		if err := upgrade.InstallSkill(cmd.Context(), logf, payload.TargetSkillCDN, payload.TargetSkillCDNChecksum); err != nil {
			logf("warning: skills update failed: %v", err)
			logf("hint: the CLI is up to date; retry `npx skills add TencentCloud/tencentmeeting-cli -y -g` to refresh skills only")
			skillStatus = "failed"
		} else {
			skillStatus = "updated"
			skillPath = upgrade.SkillInstallPath()
		}
	}

	resp := map[string]interface{}{
		"status":         "updated",
		"from_version":   o.tmeet.CLIVersion,
		"target_version": payload.TargetVersion,
		"installer":      npmPath,
		"source":         source,
		"skill_status":   skillStatus,
		"install_prefix": installPrefix,
		"prefix_source":  prefixSource,
	}
	if skillPath != "" {
		resp["skill_path"] = skillPath
	}
	data, _ := json.Marshal(resp)
	output.FormatPrint(cmd, "", "success", string(data), output.WithoutUpdateNotice())
	log.Infof(cmd.Context(), "tmeet update run: %s -> %s installed via %s (source=%s, skill=%s, prefix=%q source=%s)",
		o.tmeet.CLIVersion, payload.TargetVersion, npmPath, source, skillStatus, installPrefix, prefixSource)
	return nil
}

// installFromRegistry runs `npm install -g @tencentcloud/tmeet@latest`
// against the public registry.  Pinning to `@latest` (rather than the
// server-reported version) matches the signal the server is sending
// with target_from_npm=true: "the newest published release is the
// target".  It also sidesteps the race where the server already
// flipped the flag but a specific version has not yet propagated
// through registry mirrors.
//
// Delegates integrity to npm (it verifies the registry's sha512
// integrity field itself), so no local tarball or checksum step is
// required.
//
// prefix forces the install to land under <prefix>/lib/node_modules
// instead of wherever the ambient npm's default would route it; see
// Run's resolve-install-prefix flow for how it is picked.  Empty
// prefix means "use npm's default" (deliberately not overridden).
func installFromRegistry(cmd *cobra.Command, npmPath, prefix string) error {
	logf := cmdLogf(cmd)
	target := npmPackageName + "@latest"
	logf("Installing %s via npm registry (this may take a minute) ...", target)
	return upgrade.RunNpmInstall(cmd.Context(), logf, npmPath, target, prefix)
}

// installFromCDN downloads the server-staged tarball from TargetCDN,
// verifies its SHA-256, and then hands it to `npm install -g <path>`.
// Used for pre-publish / internal rollouts where the npm registry
// does not yet have the target version.
//
// prefix has the same semantics as in installFromRegistry.
func installFromCDN(cmd *cobra.Command, npmPath, prefix string, payload *notice.CachedPayload) error {
	logf := cmdLogf(cmd)

	dctx, cancel := context.WithTimeout(cmd.Context(), upgrade.DownloadTimeout)
	defer cancel()
	logf("Downloading tmeet %s ...", payload.TargetVersion)
	tmpPath, size, sum, err := upgrade.DownloadToTemp(dctx, logf, payload.TargetCDN, "", ".tgz")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmpPath) }()

	if size == 0 {
		return exception.UpdateDownloadError.With("downloaded tarball is empty")
	}
	if err := upgrade.VerifyChecksum(sum, payload.TargetCDNChecksum); err != nil {
		return err
	}

	logf("Installing via npm (this may take a minute) ...")
	return upgrade.RunNpmInstall(cmd.Context(), logf, npmPath, tmpPath, prefix)
}

// respondAlreadyLatest renders a no-op success response for the
// "already on the latest version" case.  We treat it as success
// (exit 0) because the user's intent — "be on the latest version" —
// is already satisfied; an error here would be surprising on repeated
// runs.
func respondAlreadyLatest(cmd *cobra.Command, current, target string) error {
	resp := map[string]interface{}{
		"status":          "already_latest",
		"current_version": current,
		"target_version":  target,
	}
	data, _ := json.Marshal(resp)
	output.FormatPrint(cmd, "", "success", string(data), output.WithoutUpdateNotice())
	return nil
}
