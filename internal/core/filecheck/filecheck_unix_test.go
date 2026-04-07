//go:build !windows

package filecheck

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateBeforeReadRegularFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "secret.enc")
	if err := os.WriteFile(p, []byte("payload"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := ValidateBeforeRead(p); err != nil {
		t.Fatalf("regular file owned by current user should pass: %v", err)
	}
}

func TestValidateFileDirectory(t *testing.T) {
	dir := t.TempDir()
	err := ValidateBeforeRead(dir)
	if err == nil {
		t.Fatal("ValidateBeforeRead should reject directories")
	}
}

func TestValidateBeforeReadRejectsWrongUID(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("需要 root 权限来 chown 文件")
	}
	dir := t.TempDir()
	p := filepath.Join(dir, "other-owner.enc")
	if err := os.WriteFile(p, []byte("sensitive"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chown(p, 65534, 65534); err != nil {
		t.Fatalf("Chown: %v", err)
	}
	if err := ValidateBeforeRead(p); err == nil {
		t.Fatal("ValidateBeforeRead should reject file owned by a different user")
	}
}

func TestValidateBeforeReadRejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	realFile := filepath.Join(dir, "real.enc")
	linkFile := filepath.Join(dir, "link.enc")
	if err := os.WriteFile(realFile, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(realFile, linkFile); err != nil {
		t.Fatalf("Symlink: %v", err)
	}
	if err := ValidateBeforeRead(linkFile); err == nil {
		t.Fatal("ValidateBeforeRead should reject symlink target path")
	}
}
