//go:build windows

package filecheck

import (
	"path/filepath"
	"testing"
)

func TestValidateBeforeReadWindowsNoOp(t *testing.T) {
	// 任意路径（含不存在）均不校验，恒为 nil。
	cases := []string{
		filepath.Join(t.TempDir(), "nope.enc"),
		t.TempDir(),
		`C:\this\path\should\not\be\read`,
	}
	for _, p := range cases {
		if err := ValidateBeforeRead(p); err != nil {
			t.Errorf("ValidateBeforeRead(%q) = %v, want nil", p, err)
		}
	}
}
