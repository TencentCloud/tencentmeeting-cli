package utils

import (
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"tmeet/internal/exception"
)

// writeTempFile creates a file inside t.TempDir() with the given content and
// returns the absolute path. Used by both PUT and download tests to avoid
// repeating the same bootstrap.
func writeTempFile(t *testing.T, content []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "payload.bin")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	return path
}

// mustMD5Base64 returns base64(binary MD5(content)) — the exact form COS
// expects in the Content-MD5 header.
func mustMD5Base64(t *testing.T, content []byte) string {
	t.Helper()
	sum := md5.Sum(content)
	return base64.StdEncoding.EncodeToString(sum[:])
}

func TestHexMD5ToBase64(t *testing.T) {
	// Dynamically derive the expected pair so the test never falls out of
	// sync with the MD5 implementation: MD5("") hex -> its base64 form.
	emptySum := md5.Sum(nil)
	emptyHex := hex.EncodeToString(emptySum[:])
	emptyB64 := base64.StdEncoding.EncodeToString(emptySum[:])

	tests := []struct {
		name    string
		hexIn   string
		want    string
		wantErr bool
	}{
		{
			name:  "empty string MD5",
			hexIn: emptyHex,
			want:  emptyB64,
		},
		{
			name:    "invalid hex",
			hexIn:   "not-a-hex-string",
			wantErr: true,
		},
		{
			name:    "odd length hex",
			hexIn:   "abc",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := HexMD5ToBase64(tt.hexIn)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil (out=%q)", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// TestPutFileToCOS_Success verifies that PutFileToCOS sends the exact bytes,
// the expected Content-Length, Content-MD5 and Authorization headers and
// returns nil on a 200 response from the target.
func TestPutFileToCOS_Success(t *testing.T) {
	payload := []byte("hello, cos put!")
	md5B64 := mustMD5Base64(t, payload)

	var (
		gotMethod string
		gotAuth   string
		gotMD5    string
		gotCT     string
		gotLen    int64
		gotBody   []byte
	)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotAuth = r.Header.Get("Authorization")
		gotMD5 = r.Header.Get("Content-MD5")
		gotCT = r.Header.Get("Content-Type")
		gotLen = r.ContentLength
		body, _ := io.ReadAll(r.Body)
		gotBody = body
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	path := writeTempFile(t, payload)
	err := PutFileToCOS(context.Background(), path, ts.URL+"/key",
		"sig-xxx", md5B64, "application/zip", uint64(len(payload)))
	if err != nil {
		t.Fatalf("PutFileToCOS: unexpected error: %v", err)
	}

	if gotMethod != http.MethodPut {
		t.Errorf("method: got %q, want PUT", gotMethod)
	}
	if gotAuth != "sig-xxx" {
		t.Errorf("auth: got %q, want sig-xxx", gotAuth)
	}
	if gotMD5 != md5B64 {
		t.Errorf("md5: got %q, want %q", gotMD5, md5B64)
	}
	if gotCT != "application/zip" {
		t.Errorf("content-type: got %q", gotCT)
	}
	if gotLen != int64(len(payload)) {
		t.Errorf("content-length: got %d, want %d", gotLen, len(payload))
	}
	if string(gotBody) != string(payload) {
		t.Errorf("body mismatch: got %q, want %q", gotBody, payload)
	}
}

// TestPutFileToCOS_DefaultContentType verifies that an empty contentType
// falls back to application/octet-stream.
func TestPutFileToCOS_DefaultContentType(t *testing.T) {
	payload := []byte("x")
	var gotCT string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCT = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	path := writeTempFile(t, payload)
	if err := PutFileToCOS(context.Background(), path, ts.URL,
		"auth", mustMD5Base64(t, payload), "", uint64(len(payload))); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotCT != "application/octet-stream" {
		t.Errorf("expected default octet-stream, got %q", gotCT)
	}
}

// TestPutFileToCOS_NonSuccessStatus ensures the function surfaces a
// UploadToCosError wrapping the server response body.
func TestPutFileToCOS_NonSuccessStatus(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"code":"AccessDenied"}`))
	}))
	defer ts.Close()

	path := writeTempFile(t, []byte("data"))
	err := PutFileToCOS(context.Background(), path, ts.URL,
		"auth", "md5", "application/pdf", 4)
	if err == nil {
		t.Fatal("expected error for 403, got nil")
	}
	if !exception.Is(err, exception.UploadToCosError) {
		t.Errorf("expected UploadToCosError, got %v", err)
	}
	if !strings.Contains(err.Error(), "403") {
		t.Errorf("expected status 403 in error, got %v", err)
	}
}

// TestPutFileToCOS_FileNotFound returns InvalidArgsError without issuing any
// HTTP call.
func TestPutFileToCOS_FileNotFound(t *testing.T) {
	// Fail the test if an HTTP call is made.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("unexpected HTTP call: %s %s", r.Method, r.URL.Path)
	}))
	defer ts.Close()

	err := PutFileToCOS(context.Background(), "/no/such/file.bin", ts.URL,
		"auth", "md5", "application/pdf", 10)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !exception.Is(err, exception.InvalidArgsError) {
		t.Errorf("expected InvalidArgsError, got %v", err)
	}
}

// TestDownloadFromCOS_Success streams a response body atomically into
// targetPath and verifies the final content and that no temp file is left
// behind in the target directory.
func TestDownloadFromCOS_Success(t *testing.T) {
	payload := []byte("downloaded bytes")
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(payload)
	}))
	defer ts.Close()

	dir := t.TempDir()
	target := filepath.Join(dir, "sub", "out.bin")

	if err := DownloadFromCOS(context.Background(), ts.URL, target, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read target: %v", err)
	}
	if string(got) != string(payload) {
		t.Errorf("content mismatch: got %q, want %q", got, payload)
	}

	// Ensure no leftover .tmp file in the target directory.
	entries, _ := os.ReadDir(filepath.Dir(target))
	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp") {
			t.Errorf("unexpected leftover temp file: %s", e.Name())
		}
	}
}

// TestDownloadFromCOS_ExistingNoOverwrite fails fast before any HTTP call
// when overwrite=false and the target already exists.
func TestDownloadFromCOS_ExistingNoOverwrite(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("unexpected HTTP call: %s %s", r.Method, r.URL.Path)
	}))
	defer ts.Close()

	target := writeTempFile(t, []byte("original"))
	err := DownloadFromCOS(context.Background(), ts.URL, target, false)
	if err == nil {
		t.Fatal("expected error for existing target, got nil")
	}
	if !exception.Is(err, exception.InvalidArgsError) {
		t.Errorf("expected InvalidArgsError, got %v", err)
	}
	// The original file must stay intact.
	got, _ := os.ReadFile(target)
	if string(got) != "original" {
		t.Errorf("target modified unexpectedly: %q", got)
	}
}

// TestDownloadFromCOS_Overwrite replaces an existing file atomically.
func TestDownloadFromCOS_Overwrite(t *testing.T) {
	newPayload := []byte("brand new content")
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(newPayload)
	}))
	defer ts.Close()

	target := writeTempFile(t, []byte("stale"))
	if err := DownloadFromCOS(context.Background(), ts.URL, target, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, _ := os.ReadFile(target)
	if string(got) != string(newPayload) {
		t.Errorf("overwrite failed: got %q, want %q", got, newPayload)
	}
}

// TestDownloadFromCOS_NonSuccessStatus propagates server body in the error
// and never creates the target file.
func TestDownloadFromCOS_NonSuccessStatus(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusGone)
		_, _ = w.Write([]byte(`<Error><Code>Expired</Code></Error>`))
	}))
	defer ts.Close()

	target := filepath.Join(t.TempDir(), "should-not-exist.bin")
	err := DownloadFromCOS(context.Background(), ts.URL, target, false)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !exception.Is(err, exception.UploadToCosError) {
		t.Errorf("expected UploadToCosError, got %v", err)
	}
	if !strings.Contains(err.Error(), "410") {
		t.Errorf("expected 410 in error, got %v", err)
	}
	if _, statErr := os.Stat(target); !os.IsNotExist(statErr) {
		t.Errorf("target file should not be created; stat err = %v", statErr)
	}
}
