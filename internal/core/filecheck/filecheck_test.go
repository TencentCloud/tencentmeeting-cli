package filecheck

import (
	"path/filepath"
	"runtime"
	"testing"
)

func TestValidateFileNotExist(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent")
	err := ValidateBeforeRead(path)
	if runtime.GOOS == "windows" {
		// Windows 实现为 no-op，不访问路径，恒为 nil；真实存在性由 os.ReadFile 处理。
		if err != nil {
			t.Fatalf("Windows: ValidateBeforeRead(nonexistent) = %v, want nil", err)
		}
		return
	}
	if err == nil {
		t.Fatal("ValidateBeforeRead should return error for nonexistent file")
	}
}
