package utils

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
)

// CalcFileInfo returns the file size (bytes), sha256 hex and md5 hex of the given file.
func CalcFileInfo(filePath string) (int64, string, string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return 0, "", "", err
	}
	defer f.Close()

	sha256h := sha256.New()
	md5h := md5.New()
	w := io.MultiWriter(sha256h, md5h)
	size, err := io.Copy(w, f)
	if err != nil {
		return 0, "", "", err
	}
	return size, hex.EncodeToString(sha256h.Sum(nil)), hex.EncodeToString(md5h.Sum(nil)), nil
}
