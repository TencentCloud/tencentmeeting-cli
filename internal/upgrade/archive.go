// archive.go — tar.gz / zip extractors used by the upgrade flow.
//
// Both extractors share a few invariants:
//
//   - zip-slip defence: a resolved entry path must stay strictly inside
//     dest.  We check with a cleaned-prefix comparison because
//     filepath.Join alone does not reject names crafted to escape via
//     "..".
//
//   - no symlinks / specials: the upgrade and skill payloads are
//     data-only, so symlinks / block devices / named pipes are skipped
//     silently.  That shrinks the attack surface (a malicious archive
//     cannot point us at /etc/passwd) and keeps the extracted tree
//     portable across Windows / POSIX.
//
//   - size cap (SkillArchiveMaxBytes): the running total of extracted
//     bytes is capped; crossing it aborts with an error.  Protects
//     against zip-bomb-style payloads.
//
//   - normalised modes: 0o755 on dirs, 0o644 on files.  We do not trust
//     the mode bits in the archive to be sensible.
//
// Error surface:
//     every path returns exception.UpdateExtractError.With(...) so
//     the cmd/update layer sees a single, uniform error type.  If more
//     consumers show up and that coupling bites, swap the sentinel for
//     a package-local error wrapped with errors.Is and let the cmd layer
//     translate.  See the package header for the rationale we picked.

package upgrade

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"tmeet/internal/exception"
)

// SkillArchiveMaxBytes caps the on-disk size of the extracted skill
// tree.  500 MiB is generous enough for any realistic skill bundle
// while being small enough that a runaway extract aborts quickly.  Used
// by both ExtractTarGz and ExtractZip.
const SkillArchiveMaxBytes = int64(500 << 20)

// ExtractTarGz unpacks a gzipped tar archive into dest.  The archive is
// expected to follow the npm pack layout: all real entries live under a
// single top-level "package/" directory.  That prefix is peeled off so
// callers get a flat tree under dest (dest/<file> instead of
// dest/package/<file>).  Entries outside "package/" are skipped
// silently so a mis-packed tarball degrades to "fewer files extracted"
// rather than a hard failure.
//
// Safety guards: see package-level documentation.
func ExtractTarGz(archivePath, dest string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return exception.UpdateExtractError.With("open archive: %v", err)
	}
	defer func() { _ = f.Close() }()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return exception.UpdateExtractError.With("gzip reader: %v", err)
	}
	defer func() { _ = gz.Close() }()

	tr := tar.NewReader(gz)
	destAbs, err := filepath.Abs(dest)
	if err != nil {
		return exception.UpdateExtractError.With("resolve dest: %v", err)
	}

	var total int64
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return exception.UpdateExtractError.With("tar entry: %v", err)
		}
		// npm pack normalises paths to forward slashes; keep that
		// assumption explicit so a stray backslash on a tar packed on
		// Windows does not slip past the prefix check below.
		name := strings.ReplaceAll(hdr.Name, `\`, "/")
		name = strings.TrimPrefix(name, "./")

		// npm pack layout: everything is nested under "package/".  Peel
		// that off; drop anything else.
		const npmPrefix = "package/"
		switch {
		case name == strings.TrimSuffix(npmPrefix, "/"):
			continue // the top-level "package" directory itself
		case !strings.HasPrefix(name, npmPrefix):
			// Not under package/ — unusual, skip to keep the layout
			// predictable (callers always get a flat tree under dest).
			continue
		}
		rel := strings.TrimPrefix(name, npmPrefix)
		if rel == "" {
			continue
		}

		target := filepath.Join(destAbs, filepath.FromSlash(rel))
		// zip-slip: the resolved target must stay inside destAbs.
		// filepath.Join cleans ".." but a crafted name like
		// "package/../../etc/passwd" could still land outside dest, so
		// verify with a prefix check against the cleaned destAbs.
		if !strings.HasPrefix(target+string(filepath.Separator), destAbs+string(filepath.Separator)) &&
			target != destAbs {
			return exception.UpdateExtractError.With("tar entry escapes destination: %s", hdr.Name)
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return exception.UpdateExtractError.With("mkdir %s: %v", target, err)
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return exception.UpdateExtractError.With("mkdir parent of %s: %v", target, err)
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
			if err != nil {
				return exception.UpdateExtractError.With("create %s: %v", target, err)
			}
			// LimitReader applied to the per-entry remaining budget so
			// a single oversized file aborts the whole extract.
			remaining := SkillArchiveMaxBytes - total
			if remaining <= 0 {
				_ = out.Close()
				return exception.UpdateExtractError.With("skill archive exceeds %d bytes", SkillArchiveMaxBytes)
			}
			n, copyErr := io.Copy(out, io.LimitReader(tr, remaining+1))
			closeErr := out.Close()
			if copyErr != nil {
				return exception.UpdateExtractError.With("write %s: %v", target, copyErr)
			}
			if closeErr != nil {
				return exception.UpdateExtractError.With("close %s: %v", target, closeErr)
			}
			total += n
			if total > SkillArchiveMaxBytes {
				return exception.UpdateExtractError.With("skill archive exceeds %d bytes", SkillArchiveMaxBytes)
			}
		default:
			// symlinks, hardlinks, devices, fifos, etc. — not something
			// a data-only skill bundle should contain; skip silently.
			continue
		}
	}
}

// ExtractZip unpacks a zip archive into dest.  The archive layout is
// not contractually fixed between the server and the CLI, so we take a
// best-effort approach: if every entry shares a single top-level
// directory (e.g. "tmeet-skill/", "package/", or any name the server
// picks), that prefix is peeled so callers always see a flat tree under
// dest; otherwise the archive is unpacked verbatim.
//
// Safety guards (parity with ExtractTarGz):
//   - Each resolved entry path must stay inside dest (zip-slip defence).
//   - Only regular files and directories are honoured; anything that
//     could be a symlink per mode bits is skipped (archive/zip models
//     symlinks via file mode on Unix).
//   - Running total of extracted bytes is capped at SkillArchiveMaxBytes.
//   - File modes are masked to 0o755 (dirs) / 0o644 (files) so a
//     surprising executable bit on a data file never ships to disk.
func ExtractZip(archivePath, dest string) error {
	zr, err := zip.OpenReader(archivePath)
	if err != nil {
		return exception.UpdateExtractError.With("zip reader: %v", err)
	}
	defer func() { _ = zr.Close() }()

	destAbs, err := filepath.Abs(dest)
	if err != nil {
		return exception.UpdateExtractError.With("resolve dest: %v", err)
	}

	// First pass: detect a shared top-level directory.  zip entry names
	// use forward slashes per spec, so splitting on "/" is safe even on
	// Windows.  We only trust a shared prefix if EVERY entry begins
	// with the same first segment; otherwise fall back to verbatim
	// extraction.
	prefix := DetectSharedTopDir(zr.File)

	var total int64
	for _, f := range zr.File {
		name := strings.ReplaceAll(f.Name, `\`, "/")
		name = strings.TrimPrefix(name, "./")
		if name == "" {
			continue
		}

		// Peel the shared top-level directory if we found one.  We skip
		// the directory entry itself (its name is exactly `prefix`) so
		// an empty rel does not produce a dest-root entry.
		rel := name
		if prefix != "" {
			switch {
			case name == strings.TrimSuffix(prefix, "/"):
				continue
			case strings.HasPrefix(name, prefix):
				rel = strings.TrimPrefix(name, prefix)
			}
		}
		if rel == "" {
			continue
		}

		target := filepath.Join(destAbs, filepath.FromSlash(rel))
		if !strings.HasPrefix(target+string(filepath.Separator), destAbs+string(filepath.Separator)) &&
			target != destAbs {
			return exception.UpdateExtractError.With("zip entry escapes destination: %s", f.Name)
		}

		mode := f.Mode()
		switch {
		case f.FileInfo().IsDir():
			if err := os.MkdirAll(target, 0o755); err != nil {
				return exception.UpdateExtractError.With("mkdir %s: %v", target, err)
			}
		case mode&os.ModeSymlink != 0:
			// Symlinks are expressed via mode bits in zip; a data-only
			// skill bundle should not contain any, so skip to shrink
			// the attack surface (same policy as ExtractTarGz).
			continue
		case mode.IsRegular():
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return exception.UpdateExtractError.With("mkdir parent of %s: %v", target, err)
			}
			remaining := SkillArchiveMaxBytes - total
			if remaining <= 0 {
				return exception.UpdateExtractError.With("skill archive exceeds %d bytes", SkillArchiveMaxBytes)
			}

			rc, err := f.Open()
			if err != nil {
				return exception.UpdateExtractError.With("open %s in zip: %v", f.Name, err)
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
			if err != nil {
				_ = rc.Close()
				return exception.UpdateExtractError.With("create %s: %v", target, err)
			}
			// LimitReader uses remaining+1 so a file that exactly fills
			// the remaining budget still succeeds, but one byte more
			// trips the total check below.
			n, copyErr := io.Copy(out, io.LimitReader(rc, remaining+1))
			rcErr := rc.Close()
			closeErr := out.Close()
			if copyErr != nil {
				return exception.UpdateExtractError.With("write %s: %v", target, copyErr)
			}
			if rcErr != nil {
				return exception.UpdateExtractError.With("close zip entry %s: %v", f.Name, rcErr)
			}
			if closeErr != nil {
				return exception.UpdateExtractError.With("close %s: %v", target, closeErr)
			}
			total += n
			if total > SkillArchiveMaxBytes {
				return exception.UpdateExtractError.With("skill archive exceeds %d bytes", SkillArchiveMaxBytes)
			}
		default:
			// Devices, named pipes, etc.  Nothing a data-only skill
			// bundle should contain; skip silently.
			continue
		}
	}
	return nil
}

// DetectSharedTopDir returns the single leading segment (with trailing
// slash) that is a prefix of every file in files, or "" when there is
// no such shared prefix.  It treats the archive as having "no shared
// top" when at least one entry is at the archive root — this matches a
// flat zip the server might ship.
func DetectSharedTopDir(files []*zip.File) string {
	var first string
	for _, f := range files {
		name := strings.ReplaceAll(f.Name, `\`, "/")
		name = strings.TrimPrefix(name, "./")
		if name == "" {
			continue
		}
		idx := strings.IndexByte(name, '/')
		if idx < 0 {
			// A root-level entry disqualifies the shared-top-dir model.
			return ""
		}
		seg := name[:idx+1] // include trailing slash
		if first == "" {
			first = seg
			continue
		}
		if seg != first {
			return ""
		}
	}
	return first
}
