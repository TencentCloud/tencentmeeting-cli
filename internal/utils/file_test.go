package utils

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"testing"
)

// TestCalcFileInfo tests CalcFileInfo with various scenarios.
func TestCalcFileInfo(t *testing.T) {
	tests := []struct {
		name     string
		content  []byte
		wantSize int64
	}{
		{
			name:     "normal content",
			content:  []byte("hello, tmeet!"),
			wantSize: 13,
		},
		{
			name:     "empty file",
			content:  []byte{},
			wantSize: 0,
		},
		{
			name:     "binary content",
			content:  []byte{0x00, 0xFF, 0x10, 0xAB},
			wantSize: 4,
		},
		{
			name:     "multi-line text",
			content:  []byte("line1\nline2\nline3\n"),
			wantSize: 18,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 创建临时文件
			f, err := os.CreateTemp(t.TempDir(), "calc_file_info_*.tmp")
			if err != nil {
				t.Fatalf("failed to create temp file: %v", err)
			}
			if _, err = f.Write(tt.content); err != nil {
				t.Fatalf("failed to write temp file: %v", err)
			}
			_ = f.Close()

			// 计算期望值
			sha256h := sha256.Sum256(tt.content)
			md5h := md5.Sum(tt.content)
			wantHash := hex.EncodeToString(sha256h[:])
			wantMD5 := hex.EncodeToString(md5h[:])

			// 调用被测函数
			gotSize, gotHash, gotMD5, err := CalcFileInfo(f.Name())
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if gotSize != tt.wantSize {
				t.Errorf("size mismatch: got=%d, want=%d", gotSize, tt.wantSize)
			}
			if gotHash != wantHash {
				t.Errorf("sha256 mismatch: got=%s, want=%s", gotHash, wantHash)
			}
			if gotMD5 != wantMD5 {
				t.Errorf("md5 mismatch: got=%s, want=%s", gotMD5, wantMD5)
			}
		})
	}
}

// TestCalcFileInfo_FileNotFound tests that a non-existent file returns an error.
func TestCalcFileInfo_FileNotFound(t *testing.T) {
	_, _, _, err := CalcFileInfo("/non/existent/path/file.zip")
	if err == nil {
		t.Error("expected error for non-existent file, but got none")
	}
}
