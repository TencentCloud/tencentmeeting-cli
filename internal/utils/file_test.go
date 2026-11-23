package utils

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"hash/crc64"
	"os"
	"strconv"
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
			sha1h := sha1.Sum(tt.content)
			sha256h := sha256.Sum256(tt.content)
			md5h := md5.Sum(tt.content)
			wantSHA1 := hex.EncodeToString(sha1h[:])
			wantSHA256 := hex.EncodeToString(sha256h[:])
			wantMD5 := hex.EncodeToString(md5h[:])
			wantCRC64 := strconv.FormatUint(
				crc64.Checksum(tt.content, crc64.MakeTable(crc64.ECMA)), 10)

			// 调用被测函数
			got, err := CalcFileInfo(f.Name())
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got.Size != tt.wantSize {
				t.Errorf("size mismatch: got=%d, want=%d", got.Size, tt.wantSize)
			}
			if got.SHA1 != wantSHA1 {
				t.Errorf("sha1 mismatch: got=%s, want=%s", got.SHA1, wantSHA1)
			}
			if got.SHA256 != wantSHA256 {
				t.Errorf("sha256 mismatch: got=%s, want=%s", got.SHA256, wantSHA256)
			}
			if got.MD5 != wantMD5 {
				t.Errorf("md5 mismatch: got=%s, want=%s", got.MD5, wantMD5)
			}
			if got.CRC64 != wantCRC64 {
				t.Errorf("crc64 mismatch: got=%s, want=%s", got.CRC64, wantCRC64)
			}
		})
	}
}

// TestCalcFileInfo_FileNotFound tests that a non-existent file returns an error.
func TestCalcFileInfo_FileNotFound(t *testing.T) {
	_, err := CalcFileInfo("/non/existent/path/file.zip")
	if err == nil {
		t.Error("expected error for non-existent file, but got none")
	}
}
