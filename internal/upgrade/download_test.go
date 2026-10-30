package upgrade

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// -----------------------------------------------------------------------------
// DownloadToTemp
// -----------------------------------------------------------------------------

func Test_DownloadToTemp(t *testing.T) {
	body := bytes.Repeat([]byte("x"), 300*1024) // > 256 KiB to trip progress once

	t.Run("200 with Content-Length populates sha and size", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(body)))
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(body)
		}))
		defer srv.Close()

		logf, buf := captureLogf()
		path, size, sum, err := DownloadToTemp(context.Background(), logf, srv.URL, t.TempDir(), "")
		if err != nil {
			t.Fatalf("DownloadToTemp: %v", err)
		}
		defer func() { _ = os.Remove(path) }()

		if size != int64(len(body)) {
			t.Fatalf("size = %d, want %d", size, len(body))
		}
		if sum != sha256Hex(body) {
			t.Fatalf("sum = %q, want %q", sum, sha256Hex(body))
		}
		got, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(got, body) {
			t.Fatalf("on-disk bytes mismatch: err=%v, len(got)=%d", err, len(got))
		}
		// Progress is throttled, but the Finish line always prints —
		// look for the terminal "Downloaded ..." marker.
		if !strings.Contains(buf.String(), "Downloaded") {
			t.Fatalf("expected Finish line in logf buffer, got %q", buf.String())
		}
	})

	t.Run("non-200 returns error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer srv.Close()

		logf, _ := captureLogf()
		_, _, _, err := DownloadToTemp(context.Background(), logf, srv.URL, t.TempDir(), "")
		if err == nil || !strings.Contains(err.Error(), "unexpected status 404") {
			t.Fatalf("expected 404 error, got %v", err)
		}
	})

	t.Run("empty body still returns zero size and a valid sha256", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Length", "0")
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		logf, _ := captureLogf()
		path, size, sum, err := DownloadToTemp(context.Background(), logf, srv.URL, t.TempDir(), "")
		if err != nil {
			t.Fatalf("DownloadToTemp: %v", err)
		}
		defer func() { _ = os.Remove(path) }()
		if size != 0 {
			t.Fatalf("size = %d, want 0", size)
		}
		// sha256 of empty: e3b0c442...
		if sum != sha256Hex(nil) {
			t.Fatalf("sum = %q, want sha256(empty) = %q", sum, sha256Hex(nil))
		}
	})

	t.Run("chunked (no Content-Length) still works", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Deliberately NOT setting Content-Length — net/http will
			// then default to chunked transfer encoding, so
			// resp.ContentLength will be -1 on the client side.
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(body)
		}))
		defer srv.Close()

		logf, _ := captureLogf()
		path, size, sum, err := DownloadToTemp(context.Background(), logf, srv.URL, t.TempDir(), "")
		if err != nil {
			t.Fatalf("DownloadToTemp (chunked): %v", err)
		}
		defer func() { _ = os.Remove(path) }()
		if size != int64(len(body)) || sum != sha256Hex(body) {
			t.Fatalf("size/sum mismatch under chunked transfer")
		}
	})

	t.Run("nil logf is tolerated", func(t *testing.T) {
		// Regression test for the newly-supported "silent" mode: pass
		// nil logf and verify the download still produces the right
		// bytes without panicking in the progress writer.
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("silent"))
		}))
		defer srv.Close()

		path, size, sum, err := DownloadToTemp(context.Background(), nil, srv.URL, t.TempDir(), "")
		if err != nil {
			t.Fatalf("DownloadToTemp with nil logf: %v", err)
		}
		defer func() { _ = os.Remove(path) }()
		if size != 6 || sum != sha256Hex([]byte("silent")) {
			t.Fatalf("unexpected size/sum: size=%d sum=%s", size, sum)
		}
	})
}

// -----------------------------------------------------------------------------
// VerifyChecksum
// -----------------------------------------------------------------------------

func Test_VerifyChecksum(t *testing.T) {
	const got = "a1b2c3"

	t.Run("empty want returns error", func(t *testing.T) {
		err := VerifyChecksum(got, "")
		if err == nil {
			t.Fatalf("expected error for empty want, got nil")
		}
		if !strings.Contains(err.Error(), got) {
			t.Fatalf("expected got digest surfaced in error, got %q", err.Error())
		}
	})

	t.Run("empty want with nil logf still errors", func(t *testing.T) {
		// Nil logf must not panic and the empty-want branch must error.
		if err := VerifyChecksum(got, ""); err == nil {
			t.Fatalf("expected error for empty-want nil-logf, got nil")
		}
	})

	t.Run("exact match", func(t *testing.T) {
		if err := VerifyChecksum(got, got); err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
	})

	t.Run("sha256: prefix accepted", func(t *testing.T) {
		if err := VerifyChecksum(got, "sha256:"+got); err != nil {
			t.Fatalf("expected nil for sha256: prefix, got %v", err)
		}
	})

	t.Run("SHA256: prefix accepted", func(t *testing.T) {
		if err := VerifyChecksum(got, "SHA256:"+got); err != nil {
			t.Fatalf("expected nil for SHA256: prefix, got %v", err)
		}
	})

	t.Run("case insensitive", func(t *testing.T) {
		if err := VerifyChecksum("ABCDEF", "abcdef"); err != nil {
			t.Fatalf("expected nil for case-insensitive match, got %v", err)
		}
	})

	t.Run("mismatch returns error with both digests", func(t *testing.T) {
		err := VerifyChecksum("aaaa", "bbbb")
		if err == nil {
			t.Fatalf("expected mismatch error, got nil")
		}
		if !strings.Contains(err.Error(), "aaaa") || !strings.Contains(err.Error(), "bbbb") {
			t.Fatalf("expected both digests surfaced in error, got %q", err.Error())
		}
	})

	t.Run("whitespace in want is trimmed", func(t *testing.T) {
		if err := VerifyChecksum(got, "   "+got+"\n"); err != nil {
			t.Fatalf("expected nil after trim, got %v", err)
		}
	})
}
