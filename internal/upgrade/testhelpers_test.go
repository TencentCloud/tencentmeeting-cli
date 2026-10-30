package upgrade

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"testing"
)

// -----------------------------------------------------------------------------
// Package-wide test helpers.
//
// Kept in their own file so each *_test.go can focus on a single area
// (archive, download, ...) without re-defining builders.  All helpers
// are small and allocation-free in the common path so running the full
// suite stays well under 2s.
// -----------------------------------------------------------------------------

// isolateConfigDir redirects config.GetConfigDir() to a per-test
// tempdir by setting TMEET_CLI_CONFIG_DIR.  Returns the directory so
// the test can inspect / seed it.
func isolateConfigDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("TMEET_CLI_CONFIG_DIR", dir)
	return dir
}

// sha256Hex returns the lowercase hex SHA-256 of b.  Shared by tests
// that need to compute the expected checksum for a synthetic archive.
func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// captureLogf returns a Logf whose output is appended (one line per
// call, with a trailing newline) to the returned buffer.  Lets tests
// assert on the exact strings the upgrade package would normally hand
// to output.EventStderr, without needing a cobra command around.
func captureLogf() (Logf, *bytes.Buffer) {
	var buf bytes.Buffer
	return func(format string, args ...interface{}) {
		if len(args) == 0 {
			buf.WriteString(format)
		} else {
			buf.WriteString(fmt.Sprintf(format, args...))
		}
		buf.WriteByte('\n')
	}, &buf
}

// buildNpmPackTgz returns an in-memory gzipped tarball that mimics the
// layout `npm pack` produces: a top-level "package/" directory
// containing the files described by `entries`.  Keys are forward-slash
// paths relative to the top-level package root; values are the file
// contents.
//
// A single "package/" directory entry is emitted first so the
// extractor also sees a tar.TypeDir, exercising both header branches.
func buildNpmPackTgz(t *testing.T, entries map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)

	// Top-level dir entry — npm pack always emits one.
	if err := tw.WriteHeader(&tar.Header{
		Name:     "package/",
		Typeflag: tar.TypeDir,
		Mode:     0o755,
	}); err != nil {
		t.Fatalf("write package/ header: %v", err)
	}

	for name, body := range entries {
		if err := tw.WriteHeader(&tar.Header{
			Name:     "package/" + name,
			Typeflag: tar.TypeReg,
			Mode:     0o644,
			Size:     int64(len(body)),
		}); err != nil {
			t.Fatalf("write header for %s: %v", name, err)
		}
		if _, err := tw.Write([]byte(body)); err != nil {
			t.Fatalf("write body for %s: %v", name, err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar writer: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("close gzip writer: %v", err)
	}
	return buf.Bytes()
}

// buildSkillZip returns an in-memory zip archive used to mimic what
// the server actually ships for the skill payload.  When topDir is
// non-empty every file name is prefixed with topDir+"/" (exercising
// the shared-top-dir peeling branch in ExtractZip); when topDir is
// empty the files land directly at the archive root (exercising the
// flat-layout branch).
//
// Keys in entries are forward-slash paths; values are the file
// contents.  Deterministic iteration order is NOT required because
// ExtractZip must handle any order.
func buildSkillZip(t *testing.T, topDir string, entries map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	// If topDir is set, emit the directory entry first so the
	// extractor sees a FileInfo().IsDir() record alongside the file
	// entries.
	if topDir != "" {
		hdr := &zip.FileHeader{Name: topDir + "/", Method: zip.Store}
		hdr.SetMode(0o755 | os.ModeDir)
		if _, err := zw.CreateHeader(hdr); err != nil {
			t.Fatalf("create top dir %q: %v", topDir, err)
		}
	}

	for name, body := range entries {
		full := name
		if topDir != "" {
			full = topDir + "/" + name
		}
		hdr := &zip.FileHeader{Name: full, Method: zip.Deflate}
		hdr.SetMode(0o644)
		w, err := zw.CreateHeader(hdr)
		if err != nil {
			t.Fatalf("create entry %q: %v", full, err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatalf("write entry %q: %v", full, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip writer: %v", err)
	}
	return buf.Bytes()
}
