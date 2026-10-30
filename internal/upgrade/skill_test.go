package upgrade

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func Test_InstallSkill_FirstTime(t *testing.T) {
	cfgDir := isolateConfigDir(t)
	zipBytes := buildSkillZip(t, "tmeet-skill", map[string]string{
		"index.md":        "# tmeet-skill",
		"commands/a.json": `{"name":"a"}`,
	})
	sum := sha256Hex(zipBytes)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(zipBytes)))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(zipBytes)
	}))
	defer srv.Close()

	logf, _ := captureLogf()
	if err := InstallSkill(context.Background(), logf, srv.URL, sum); err != nil {
		t.Fatalf("InstallSkill: %v", err)
	}

	finalPath := filepath.Join(cfgDir, "skills", SkillDirName)
	if _, err := os.Stat(finalPath); err != nil {
		t.Fatalf("final skill dir missing: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(finalPath, "index.md"))
	if err != nil || string(got) != "# tmeet-skill" {
		t.Fatalf("index.md: content=%q err=%v", got, err)
	}
}

func Test_InstallSkill_OverwritesExistingAndCleansBackup(t *testing.T) {
	cfgDir := isolateConfigDir(t)
	// Seed an "old" skill dir with recognisable content.
	finalPath := filepath.Join(cfgDir, "skills", SkillDirName)
	if err := os.MkdirAll(finalPath, 0o755); err != nil {
		t.Fatalf("seed dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(finalPath, "OLD"), []byte("stale"), 0o644); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	tgz := buildSkillZip(t, "", map[string]string{"NEW": "fresh"})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(tgz)
	}))
	defer srv.Close()

	logf, _ := captureLogf()
	if err := InstallSkill(context.Background(), logf, srv.URL, sha256Hex(tgz)); err != nil {
		t.Fatalf("InstallSkill: %v", err)
	}

	// Old file must be gone, new file must be present.
	if _, err := os.Stat(filepath.Join(finalPath, "OLD")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected old file to be gone, stat err=%v", err)
	}
	got, err := os.ReadFile(filepath.Join(finalPath, "NEW"))
	if err != nil || string(got) != "fresh" {
		t.Fatalf("NEW: content=%q err=%v", got, err)
	}

	// Backup dir should have been cleaned up — the skills root
	// should contain only tmeet-skill now.
	entries, err := os.ReadDir(filepath.Join(cfgDir, "skills"))
	if err != nil {
		t.Fatalf("read skills dir: %v", err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), SkillDirName+".old-") {
			t.Fatalf("backup %q should have been removed", e.Name())
		}
		if strings.HasPrefix(e.Name(), "."+SkillDirName+"-staging-") {
			t.Fatalf("staging %q should have been consumed by rename", e.Name())
		}
	}
}

func Test_InstallSkill_ChecksumMismatchLeavesExistingUntouched(t *testing.T) {
	cfgDir := isolateConfigDir(t)
	finalPath := filepath.Join(cfgDir, "skills", SkillDirName)
	if err := os.MkdirAll(finalPath, 0o755); err != nil {
		t.Fatalf("seed dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(finalPath, "KEEP"), []byte("safe"), 0o644); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	tgz := buildSkillZip(t, "package", map[string]string{"attempted": "payload"})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(tgz)
	}))
	defer srv.Close()

	logf, _ := captureLogf()
	err := InstallSkill(context.Background(), logf, srv.URL,
		"0000000000000000000000000000000000000000000000000000000000000000")
	if err == nil {
		t.Fatalf("expected checksum error")
	}
	if !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("expected checksum error, got %v", err)
	}

	// The pre-existing skill must be entirely unchanged.
	got, err := os.ReadFile(filepath.Join(finalPath, "KEEP"))
	if err != nil || string(got) != "safe" {
		t.Fatalf("existing skill was disturbed: content=%q err=%v", got, err)
	}
}

func Test_InstallSkill_DownloadFailureLeavesExistingUntouched(t *testing.T) {
	cfgDir := isolateConfigDir(t)
	finalPath := filepath.Join(cfgDir, "skills", SkillDirName)
	_ = os.MkdirAll(finalPath, 0o755)
	_ = os.WriteFile(filepath.Join(finalPath, "KEEP"), []byte("safe"), 0o644)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	logf, _ := captureLogf()
	if err := InstallSkill(context.Background(), logf, srv.URL, "deadbeef"); err == nil {
		t.Fatalf("expected download error")
	}
	// Pre-existing skill still there.
	got, err := os.ReadFile(filepath.Join(finalPath, "KEEP"))
	if err != nil || string(got) != "safe" {
		t.Fatalf("existing skill was disturbed on download failure")
	}
}
