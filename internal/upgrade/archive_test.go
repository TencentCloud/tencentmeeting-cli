package upgrade

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// -----------------------------------------------------------------------------
// ExtractTarGz
// -----------------------------------------------------------------------------

func Test_ExtractTarGz_HappyPath(t *testing.T) {
	// Build a tarball with a nested file and verify the "package/"
	// prefix is peeled: output should land at dest/foo.txt and
	// dest/sub/bar.txt.
	tgz := buildNpmPackTgz(t, map[string]string{
		"foo.txt":     "hello",
		"sub/bar.txt": "world",
	})
	archive := filepath.Join(t.TempDir(), "pkg.tgz")
	if err := os.WriteFile(archive, tgz, 0o644); err != nil {
		t.Fatalf("write archive: %v", err)
	}
	dest := t.TempDir()
	if err := ExtractTarGz(archive, dest); err != nil {
		t.Fatalf("ExtractTarGz: %v", err)
	}

	foo, err := os.ReadFile(filepath.Join(dest, "foo.txt"))
	if err != nil || string(foo) != "hello" {
		t.Fatalf("foo.txt: content=%q err=%v", foo, err)
	}
	bar, err := os.ReadFile(filepath.Join(dest, "sub", "bar.txt"))
	if err != nil || string(bar) != "world" {
		t.Fatalf("sub/bar.txt: content=%q err=%v", bar, err)
	}

	// "package/" prefix must be peeled: no residual package/ directory
	// should exist under dest.
	if _, err := os.Stat(filepath.Join(dest, "package")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected no residual package/ dir, got stat err=%v", err)
	}
}

func Test_ExtractTarGz_ZipSlip(t *testing.T) {
	// Craft a tarball whose header name tries to escape dest.
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	_ = tw.WriteHeader(&tar.Header{
		Name:     "package/../../etc/evil.conf",
		Typeflag: tar.TypeReg,
		Mode:     0o644,
		Size:     4,
	})
	_, _ = tw.Write([]byte("evil"))
	_ = tw.Close()
	_ = gz.Close()

	archive := filepath.Join(t.TempDir(), "evil.tgz")
	if err := os.WriteFile(archive, buf.Bytes(), 0o644); err != nil {
		t.Fatalf("write archive: %v", err)
	}
	dest := t.TempDir()
	err := ExtractTarGz(archive, dest)
	if err == nil {
		t.Fatalf("expected zip-slip detection error, got nil")
	}
	if !strings.Contains(err.Error(), "escapes destination") {
		t.Fatalf("expected 'escapes destination' error, got %v", err)
	}
}

func Test_ExtractTarGz_SkipsNonPackageEntries(t *testing.T) {
	// A real npm pack never emits anything outside package/, but a
	// malformed archive might.  The extractor should silently skip
	// such entries rather than fail or write them.
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	_ = tw.WriteHeader(&tar.Header{
		Name:     "sibling/other.txt",
		Typeflag: tar.TypeReg,
		Mode:     0o644,
		Size:     3,
	})
	_, _ = tw.Write([]byte("nope"[:3]))
	_ = tw.WriteHeader(&tar.Header{
		Name:     "package/kept.txt",
		Typeflag: tar.TypeReg,
		Mode:     0o644,
		Size:     4,
	})
	_, _ = tw.Write([]byte("keep"))
	_ = tw.Close()
	_ = gz.Close()

	archive := filepath.Join(t.TempDir(), "mixed.tgz")
	if err := os.WriteFile(archive, buf.Bytes(), 0o644); err != nil {
		t.Fatalf("write archive: %v", err)
	}
	dest := t.TempDir()
	if err := ExtractTarGz(archive, dest); err != nil {
		t.Fatalf("ExtractTarGz: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, "sibling")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("sibling entry should have been skipped")
	}
	got, err := os.ReadFile(filepath.Join(dest, "kept.txt"))
	if err != nil || string(got) != "keep" {
		t.Fatalf("kept.txt: content=%q err=%v", got, err)
	}
}

func Test_ExtractTarGz_SkipsSymlinks(t *testing.T) {
	// A symlink in a data-only skill bundle is unusual and could be
	// used to point at a sensitive file.  The extractor skips them
	// silently.
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	_ = tw.WriteHeader(&tar.Header{
		Name:     "package/link",
		Typeflag: tar.TypeSymlink,
		Linkname: "/etc/passwd",
		Mode:     0o777,
	})
	_ = tw.Close()
	_ = gz.Close()

	archive := filepath.Join(t.TempDir(), "sym.tgz")
	if err := os.WriteFile(archive, buf.Bytes(), 0o644); err != nil {
		t.Fatalf("write archive: %v", err)
	}
	dest := t.TempDir()
	if err := ExtractTarGz(archive, dest); err != nil {
		t.Fatalf("ExtractTarGz: %v", err)
	}
	// The symlink must not have been materialised.
	if _, err := os.Lstat(filepath.Join(dest, "link")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("symlink should have been skipped")
	}
}

// -----------------------------------------------------------------------------
// ExtractZip
// -----------------------------------------------------------------------------

func Test_ExtractZip_SharedTopDirPeeled(t *testing.T) {
	// Every entry shares the "tmeet-skill/" prefix → the extractor
	// must peel it so files land flat under dest.
	zipBytes := buildSkillZip(t, "tmeet-skill", map[string]string{
		"index.md":     "hello",
		"sub/data.txt": "world",
	})
	archive := filepath.Join(t.TempDir(), "pkg.zip")
	if err := os.WriteFile(archive, zipBytes, 0o644); err != nil {
		t.Fatalf("write archive: %v", err)
	}
	dest := t.TempDir()
	if err := ExtractZip(archive, dest); err != nil {
		t.Fatalf("ExtractZip: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(dest, "index.md"))
	if err != nil || string(got) != "hello" {
		t.Fatalf("index.md: content=%q err=%v", got, err)
	}
	got, err = os.ReadFile(filepath.Join(dest, "sub", "data.txt"))
	if err != nil || string(got) != "world" {
		t.Fatalf("sub/data.txt: content=%q err=%v", got, err)
	}
	// Residual top-level dir must be gone.
	if _, err := os.Stat(filepath.Join(dest, "tmeet-skill")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected no residual tmeet-skill/ dir, got stat err=%v", err)
	}
}

func Test_ExtractZip_FlatLayout(t *testing.T) {
	// Entries live at the archive root → nothing to peel, extract
	// verbatim.
	zipBytes := buildSkillZip(t, "", map[string]string{
		"index.md":     "flat",
		"sub/data.txt": "nested",
	})
	archive := filepath.Join(t.TempDir(), "flat.zip")
	if err := os.WriteFile(archive, zipBytes, 0o644); err != nil {
		t.Fatalf("write archive: %v", err)
	}
	dest := t.TempDir()
	if err := ExtractZip(archive, dest); err != nil {
		t.Fatalf("ExtractZip: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(dest, "index.md"))
	if err != nil || string(got) != "flat" {
		t.Fatalf("index.md: content=%q err=%v", got, err)
	}
	got, err = os.ReadFile(filepath.Join(dest, "sub", "data.txt"))
	if err != nil || string(got) != "nested" {
		t.Fatalf("sub/data.txt: content=%q err=%v", got, err)
	}
}

func Test_ExtractZip_MixedRootsSkipsPeeling(t *testing.T) {
	// Two different top-level segments → no shared prefix; both top
	// dirs must land verbatim under dest.  Covers the server
	// producing a zip without a wrapper dir but with multiple
	// top-level dirs.
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, name := range []string{"a/one.txt", "b/two.txt"} {
		hdr := &zip.FileHeader{Name: name, Method: zip.Deflate}
		hdr.SetMode(0o644)
		w, err := zw.CreateHeader(hdr)
		if err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
		_, _ = w.Write([]byte(name))
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	archive := filepath.Join(t.TempDir(), "mixed.zip")
	if err := os.WriteFile(archive, buf.Bytes(), 0o644); err != nil {
		t.Fatalf("write archive: %v", err)
	}
	dest := t.TempDir()
	if err := ExtractZip(archive, dest); err != nil {
		t.Fatalf("ExtractZip: %v", err)
	}
	one, err := os.ReadFile(filepath.Join(dest, "a", "one.txt"))
	if err != nil || string(one) != "a/one.txt" {
		t.Fatalf("a/one.txt: content=%q err=%v", one, err)
	}
	two, err := os.ReadFile(filepath.Join(dest, "b", "two.txt"))
	if err != nil || string(two) != "b/two.txt" {
		t.Fatalf("b/two.txt: content=%q err=%v", two, err)
	}
}

func Test_ExtractZip_ZipSlip(t *testing.T) {
	// A crafted path that tries to escape dest must be rejected before
	// any filesystem write happens.
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	hdr := &zip.FileHeader{Name: "pkg/../../etc/evil.conf", Method: zip.Deflate}
	hdr.SetMode(0o644)
	w, err := zw.CreateHeader(hdr)
	if err != nil {
		t.Fatalf("create entry: %v", err)
	}
	_, _ = w.Write([]byte("evil"))
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	archive := filepath.Join(t.TempDir(), "evil.zip")
	if err := os.WriteFile(archive, buf.Bytes(), 0o644); err != nil {
		t.Fatalf("write archive: %v", err)
	}
	dest := t.TempDir()
	err = ExtractZip(archive, dest)
	if err == nil {
		t.Fatalf("expected zip-slip detection error, got nil")
	}
	if !strings.Contains(err.Error(), "escapes destination") {
		t.Fatalf("expected 'escapes destination' error, got %v", err)
	}
}

func Test_ExtractZip_SkipsSymlinks(t *testing.T) {
	// zip encodes symlinks via file mode bits.  A data-only skill
	// bundle should contain none; extractor must skip rather than
	// materialise.
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	hdr := &zip.FileHeader{Name: "tmeet-skill/link", Method: zip.Deflate}
	hdr.SetMode(0o777 | os.ModeSymlink)
	w, err := zw.CreateHeader(hdr)
	if err != nil {
		t.Fatalf("create link entry: %v", err)
	}
	_, _ = w.Write([]byte("/etc/passwd"))

	// A sibling regular file so the archive has at least one real
	// entry.
	hdr2 := &zip.FileHeader{Name: "tmeet-skill/real.txt", Method: zip.Deflate}
	hdr2.SetMode(0o644)
	w2, err := zw.CreateHeader(hdr2)
	if err != nil {
		t.Fatalf("create real.txt: %v", err)
	}
	_, _ = w2.Write([]byte("ok"))
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}

	archive := filepath.Join(t.TempDir(), "sym.zip")
	if err := os.WriteFile(archive, buf.Bytes(), 0o644); err != nil {
		t.Fatalf("write archive: %v", err)
	}
	dest := t.TempDir()
	if err := ExtractZip(archive, dest); err != nil {
		t.Fatalf("ExtractZip: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(dest, "link")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("symlink should have been skipped")
	}
	// The real file still gets extracted (shared top dir
	// "tmeet-skill/" is peeled, so real.txt lands at dest/real.txt).
	got, err := os.ReadFile(filepath.Join(dest, "real.txt"))
	if err != nil || string(got) != "ok" {
		t.Fatalf("real.txt: content=%q err=%v", got, err)
	}
}

func Test_ExtractZip_RejectsNonZip(t *testing.T) {
	// A tgz served to the zip extractor must fail cleanly instead of
	// writing nothing and reporting success — this is the regression
	// test for the original "gzip: invalid header" bug.
	tgz := buildNpmPackTgz(t, map[string]string{"foo.txt": "hi"})
	archive := filepath.Join(t.TempDir(), "actually-a-tgz.zip")
	if err := os.WriteFile(archive, tgz, 0o644); err != nil {
		t.Fatalf("write archive: %v", err)
	}
	if err := ExtractZip(archive, t.TempDir()); err == nil {
		t.Fatalf("expected error when fed a non-zip archive")
	}
}

// -----------------------------------------------------------------------------
// DetectSharedTopDir
// -----------------------------------------------------------------------------

func Test_DetectSharedTopDir(t *testing.T) {
	build := func(names ...string) []*zip.File {
		files := make([]*zip.File, 0, len(names))
		for _, n := range names {
			files = append(files, &zip.File{FileHeader: zip.FileHeader{Name: n}})
		}
		return files
	}
	cases := []struct {
		name string
		in   []*zip.File
		want string
	}{
		{"all share tmeet-skill/", build("tmeet-skill/a", "tmeet-skill/sub/b"), "tmeet-skill/"},
		{"root-level entry disqualifies", build("tmeet-skill/a", "readme.md"), ""},
		{"two different tops", build("a/x", "b/y"), ""},
		{"single file at root", build("solo.txt"), ""},
		{"empty names skipped", build("", "pkg/x"), "pkg/"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := DetectSharedTopDir(tc.in); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}
