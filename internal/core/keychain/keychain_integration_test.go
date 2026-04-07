package keychain

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// 使用当前平台真实实现（darwin Keychain / Linux 文件 master key / Windows 注册表+DPAPI），
// 数据目录隔离到 t.TempDir()，避免污染用户环境。
func TestIntegrationSetGetRemove(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("TMEET_CLI_DATA_DIR", dataDir)

	kc := New()
	account := "integration-cli-sdk"
	payload := `{"secret":"x","access_token":"y"}`

	t.Cleanup(func() {
		_ = kc.Remove(ServiceName, account)
	})

	if err := kc.Set(ServiceName, account, payload); err != nil {
		t.Fatalf("Set: %v", err)
	}

	got, err := kc.Get(ServiceName, account)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != payload {
		t.Fatalf("Get() = %q, want %q", got, payload)
	}

	if err := kc.Remove(ServiceName, account); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	_, err = kc.Get(ServiceName, account)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("after Remove, Get err = %v, want ErrNotFound", err)
	}
}

func TestIntegrationConcurrentSetSameAccount(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("TMEET_CLI_DATA_DIR", dataDir)

	kc := New()
	account := "integration-concurrent"
	t.Cleanup(func() { _ = kc.Remove(ServiceName, account) })

	const n = 8
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		go func(v int) {
			errs <- kc.Set(ServiceName, account, fmt.Sprintf(`{"i":%d}`, v))
		}(i)
	}
	for i := 0; i < n; i++ {
		if err := <-errs; err != nil {
			t.Errorf("Set goroutine: %v", err)
		}
	}

	_, err := kc.Get(ServiceName, account)
	if err != nil {
		t.Fatalf("Get after concurrent Set: %v", err)
	}
}

func TestIntegrationEncFilesUnderDataDirUnixLike(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode: skip filesystem layout check")
	}
	if runtime.GOOS == "windows" {
		t.Skip("Windows stores ciphertext in registry, no .enc under data dir")
	}

	dataDir := t.TempDir()
	t.Setenv("TMEET_CLI_DATA_DIR", dataDir)

	kc := New()
	account := "integration-fs-layout"
	t.Cleanup(func() { _ = kc.Remove(ServiceName, account) })

	if err := kc.Set(ServiceName, account, `{"k":"v"}`); err != nil {
		t.Fatal(err)
	}

	encPath := filepath.Join(dataDir, account+".enc")
	if _, err := os.Stat(encPath); err != nil {
		t.Fatalf("expected .enc at %s: %v", encPath, err)
	}

	raw, err := os.ReadFile(encPath)
	if err != nil {
		t.Fatalf("ReadFile .enc: %v", err)
	}
	if bytes.Contains(raw, []byte(`{"k":"v"}`)) {
		t.Fatal(".enc file must not contain plaintext; data should be encrypted")
	}
}

func TestIntegrationLoadExistingMasterKey(t *testing.T) {
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		t.Skip("master key not stored as file on " + runtime.GOOS)
	}

	dataDir := t.TempDir()
	t.Setenv("TMEET_CLI_DATA_DIR", dataDir)

	account := "reload-test"
	payload := `{"token":"persist-across-instances"}`

	kc1 := New()
	if err := kc1.Set(ServiceName, account, payload); err != nil {
		t.Fatalf("Set with kc1: %v", err)
	}

	kc2 := New()
	got, err := kc2.Get(ServiceName, account)
	if err != nil {
		t.Fatalf("Get with kc2 (reload): %v", err)
	}
	if got != payload {
		t.Fatalf("kc2.Get() = %q, want %q", got, payload)
	}

	t.Cleanup(func() { _ = kc2.Remove(ServiceName, account) })
}

func TestIntegrationMasterKeyWrongSize(t *testing.T) {
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		t.Skip("master key not stored as file on " + runtime.GOOS)
	}

	dataDir := t.TempDir()
	t.Setenv("TMEET_CLI_DATA_DIR", dataDir)

	mkPath := filepath.Join(dataDir, MasterKeyAccount)
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(mkPath, []byte("too-short-key"), 0600); err != nil {
		t.Fatal(err)
	}

	kc := New()
	_, err := kc.Get(ServiceName, "any-account")
	if err == nil {
		t.Fatal("Get should fail when master key file has wrong size")
	}
}

func TestIntegrationMasterKeyFilePermission(t *testing.T) {
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		t.Skip("master key not stored as file on " + runtime.GOOS)
	}

	dataDir := t.TempDir()
	t.Setenv("TMEET_CLI_DATA_DIR", dataDir)

	kc := New()
	account := "perm-check"
	t.Cleanup(func() { _ = kc.Remove(ServiceName, account) })

	if err := kc.Set(ServiceName, account, `{"k":"v"}`); err != nil {
		t.Fatal(err)
	}

	mkPath := filepath.Join(dataDir, MasterKeyAccount)
	info, err := os.Stat(mkPath)
	if err != nil {
		t.Fatalf("Stat master key: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf("master key permission = %o, want 0600", perm)
	}
}
