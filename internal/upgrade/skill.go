// skill.go — downloads and swaps in a fresh tmeet-skill tree under
// <ConfigDir>/skills/.
//
// Separated from the CLI-side upgrade path for two reasons:
//   - Different payload semantics: the skill archive is a zip, the CLI
//     tarball is a tgz; they also have different checksum fields on the
//     wire.
//   - Different failure semantics in the caller: a failed skill install
//     is non-fatal (the CLI itself is already up to date), whereas a
//     failed CLI install must abort.  Keeping the skill flow in its own
//     file makes it easy for the caller to wire it in behind a
//     best-effort error check.
//
// Atomicity contract (same as before): if any step fails, the
// pre-existing tmeet-skill directory (if any) is left untouched.  The
// swap is done via rename-rename-remove so there is no window where
// the final path points to half-extracted contents.

package upgrade

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"tmeet/internal/config"
	"tmeet/internal/exception"
)

// SkillDirName is the on-disk directory under <ConfigDir>/skills/ that
// holds the extracted tmeet-skill payload.
const SkillDirName = "tmeet-skill"

// SkillInstallPath returns the on-disk directory where the extracted
// tmeet-skill payload lives (<ConfigDir>/skills/tmeet-skill).  It is
// the single source of truth used both by InstallSkill (to know where
// to install) and by callers that want to report the location on
// success.
func SkillInstallPath() string {
	return filepath.Join(config.GetConfigDir(), "skills", SkillDirName)
}

// InstallSkill downloads the skills zip archive from cdnURL, verifies
// its SHA-256 against wantChecksum, extracts it into a staging
// directory, and then atomically swaps it into place at
// <ConfigDir>/skills/tmeet-skill.
//
// Atomicity contract: if any step fails, the pre-existing tmeet-skill
// directory (if any) is left untouched.  The swap is done via rename-
// rename-remove so there is no window where the final path points to
// half-extracted contents:
//
//  1. extract to  <skills>/.tmeet-skill-staging-<n>/
//  2. rename old  <skills>/tmeet-skill        → <skills>/.tmeet-skill-old-<n>/
//  3. rename new  <skills>/.tmeet-skill-staging-<n> → <skills>/tmeet-skill
//  4. remove      <skills>/.tmeet-skill-old-<n>/
//
// If step 3 fails we attempt to roll back step 2 so the user does not
// end up with a missing skill directory on a transient filesystem
// error.
//
// logf receives download progress and warnings.  Pass nil to run
// silently; pass ctx with a reasonable deadline (callers typically use
// cmd.Context() so Ctrl-C propagates).
func InstallSkill(ctx context.Context, logf Logf, cdnURL, wantChecksum string) error {
	dctx, cancel := context.WithTimeout(ctx, DownloadTimeout)
	defer cancel()
	if logf != nil {
		logf("Downloading tmeet-skill ...")
	}
	tmpPath, size, sum, err := DownloadToTemp(dctx, logf, cdnURL, "", ".zip")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmpPath) }()
	if size == 0 {
		return exception.UpdateDownloadError.With("downloaded skill archive is empty")
	}
	if err := VerifyChecksum(sum, wantChecksum); err != nil {
		return err
	}

	skillsRoot := filepath.Dir(SkillInstallPath())
	if err := os.MkdirAll(skillsRoot, 0o755); err != nil {
		return exception.UpdateInstallError.With("create skills dir %s: %v", skillsRoot, err)
	}

	finalPath := SkillInstallPath()
	stagingPath, err := os.MkdirTemp(skillsRoot, "."+SkillDirName+"-staging-*")
	if err != nil {
		return exception.UpdateInstallError.With("create staging dir: %v", err)
	}
	// If anything below fails before the swap, make sure the staging
	// debris does not linger.  keepStaging is flipped to true just
	// before the rename-into-place so success leaves the directory
	// under its final name instead of trying to double-remove it.
	keepStaging := false
	defer func() {
		if !keepStaging {
			_ = os.RemoveAll(stagingPath)
		}
	}()

	if logf != nil {
		logf("Extracting tmeet-skill ...")
	}
	if err := ExtractZip(tmpPath, stagingPath); err != nil {
		return err
	}

	// Promote staging → final.  Rename the old tree aside first so the
	// final rename-into-place is a single atomic step; this keeps the
	// pre-existing tree available for rollback if the final rename
	// errors out (e.g. target is held open by another tool on Windows).
	var backupPath string
	if _, err := os.Stat(finalPath); err == nil {
		backupPath = finalPath + ".old-" + fmt.Sprintf("%d", time.Now().UnixNano())
		if err := os.Rename(finalPath, backupPath); err != nil {
			return exception.UpdateInstallError.With("backup existing skill dir: %v", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return exception.UpdateInstallError.With("stat existing skill dir: %v", err)
	}

	if err := os.Rename(stagingPath, finalPath); err != nil {
		// Try to put the old tree back before giving up so a transient
		// error does not leave the user with no skill at all.
		if backupPath != "" {
			if rbErr := os.Rename(backupPath, finalPath); rbErr != nil {
				return exception.UpdateInstallError.With(
					"install new skill dir: %v (rollback also failed: %v; previous skill preserved at %s)",
					err, rbErr, backupPath)
			}
		}
		return exception.UpdateInstallError.With("install new skill dir: %v", err)
	}
	keepStaging = true // staging path has been consumed by the rename above

	if backupPath != "" {
		if err := os.RemoveAll(backupPath); err != nil {
			// Non-fatal: the new tree is in place; a stale backup is
			// just disk clutter the next run will overwrite.
			if logf != nil {
				logf("warning: could not remove old skill backup %s: %v", backupPath, err)
			}
		}
	}
	if logf != nil {
		logf("Skills installed at %s.", finalPath)
	}
	return nil
}
