package record

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
)

func TestDownloadFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "video/mp4")
		_, _ = w.Write([]byte("fake-mp4"))
	}))
	defer server.Close()

	path := filepath.Join(t.TempDir(), "record.mp4")
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	result, err := downloadFile(cmd, server.URL, path, "file-1")
	if err != nil {
		t.Fatal(err)
	}
	if result.Bytes != int64(len("fake-mp4")) {
		t.Fatalf("Bytes = %d", result.Bytes)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "fake-mp4" {
		t.Fatalf("downloaded data = %q, err = %v", data, err)
	}
}

func TestDownloadFileRejectsHTML(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<html></html>"))
	}))
	defer server.Close()

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	_, err := downloadFile(cmd, server.URL, filepath.Join(t.TempDir(), "record.mp4"), "file-1")
	if err == nil {
		t.Fatal("expected HTML response to be rejected")
	}
}
