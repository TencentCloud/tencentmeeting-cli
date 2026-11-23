package utils

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"hash/crc64"
	"io"
	"os"
	"strconv"
)

// crc64ECMATable is cached module-wide so each CalcFileInfo call avoids the
// (non-trivial) table build cost. The ECMA-182 polynomial is the one used
// by Tencent COS for the `x-cos-hash-crc64ecma` response header, so the
// value we compute here lines up byte-for-byte with what the server will
// report back after an upload.
var crc64ECMATable = crc64.MakeTable(crc64.ECMA)

// FileDigests bundles the file size and all digests produced in a single
// streaming pass by CalcFileInfo. Callers pick whichever subset they need
// by field name (log upload uses Size+SHA256+MD5; meeting attachment
// upload-auth uses Size+SHA1+MD5+CRC64), which keeps call sites readable
// and makes it safe to add new digests in the future without touching
// every caller.
type FileDigests struct {
	Size   int64  // file size in bytes
	SHA1   string // hex-encoded sha1
	SHA256 string // hex-encoded sha256
	MD5    string // hex-encoded md5
	CRC64  string // crc64 ECMA as unsigned decimal, matches COS x-cos-hash-crc64ecma
}

// CalcFileInfo returns the file size plus four digests (sha1, sha256, md5,
// crc64 ECMA) of the given file, all computed in a single streaming pass
// via io.MultiWriter. The CRC64 value is emitted as an unsigned decimal
// string to match COS's wire format.
func CalcFileInfo(filePath string) (FileDigests, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return FileDigests{}, err
	}
	defer f.Close()

	sha1h := sha1.New()
	sha256h := sha256.New()
	md5h := md5.New()
	crc64h := crc64.New(crc64ECMATable)
	w := io.MultiWriter(sha1h, sha256h, md5h, crc64h)
	size, err := io.Copy(w, f)
	if err != nil {
		return FileDigests{}, err
	}
	return FileDigests{
		Size:   size,
		SHA1:   hex.EncodeToString(sha1h.Sum(nil)),
		SHA256: hex.EncodeToString(sha256h.Sum(nil)),
		MD5:    hex.EncodeToString(md5h.Sum(nil)),
		CRC64:  strconv.FormatUint(crc64h.Sum64(), 10),
	}, nil
}
