package utils

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"tmeet/internal/exception"
)

// cosHTTPTimeout is the per-request timeout for direct COS PUT/GET calls.
// 5 minutes is enough for the current 100MB attachment cap even on slow
// consumer uplinks (~300kbps).
const cosHTTPTimeout = 5 * time.Minute

// PutFileToCOS uploads the file at filePath to a COS-signed URL via a single
// PUT request. It is the shared primitive behind both troubleshoot-log upload
// and meeting-attachment upload.
//
// Parameters:
//   - uploadURL  : pre-signed COS url (will be query-unescaped before use to
//     match the signed form).
//   - uploadAuth : value of the Authorization header issued together with the
//     URL by the auth endpoint.
//   - contentMD5 : base64(binary MD5); required by COS for integrity check.
//     Caller is expected to convert hex->base64 (see utils.HexMD5ToBase64).
//   - contentType: MIME type to set on the Content-Type header. If empty,
//     "application/octet-stream" is used.
//   - fileSize   : file size in bytes; written to Content-Length to prevent
//     Go's net/http from falling back to chunked transfer encoding (which
//     would break COS signature verification).
//
// The function is retry-safe: it supplies Request.GetBody so the body can be
// re-opened on 307/308 redirects.
func PutFileToCOS(ctx context.Context, filePath, uploadURL, uploadAuth,
	contentMD5, contentType string, fileSize uint64) error {
	f, err := os.Open(filePath)
	if err != nil {
		return exception.InvalidArgsError.With("failed to open file %s: %v", filePath, err)
	}
	defer f.Close()

	decodedURL, err := url.QueryUnescape(uploadURL)
	if err != nil {
		return exception.InvalidArgsError.With("failed to decode upload URL: %v", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, decodedURL, f)
	if err != nil {
		return exception.InvalidArgsError.With("failed to build COS request: %v", err)
	}

	// Explicitly set Request.ContentLength to prevent Go's net/http from
	// treating the length as unknown (because body is an *os.File) and
	// falling back to Transfer-Encoding: chunked, which would break COS
	// signature verification or trigger a 411 Length Required response.
	req.ContentLength = int64(fileSize)
	// Provide GetBody so the body can be re-opened on 307/308 redirects.
	req.GetBody = func() (io.ReadCloser, error) {
		return os.Open(filePath)
	}

	if contentType == "" {
		contentType = "application/octet-stream"
	}
	req.Header.Set("Authorization", uploadAuth)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Connection", "close")
	req.Header.Set("Content-MD5", contentMD5)

	httpClt := &http.Client{Timeout: cosHTTPTimeout}
	resp, err := httpClt.Do(req)
	if err != nil {
		return exception.NetworkError.With("failed to upload file to COS: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return exception.UploadToCosError.With("COS upload failed, status: %d, body: %s", resp.StatusCode, string(body))
	}
	return nil
}

// DownloadFromCOS streams the content of a COS-signed URL into targetPath.
//
// Behavior:
//   - overwrite=false and targetPath already exists -> returns InvalidArgsError
//     without any network call (fail-fast, avoids wasted bandwidth).
//   - The file is first streamed to a sibling "<basename>.*.tmp" file, then
//     atomically renamed to targetPath; so a mid-flight failure never leaves
//     a half-written file at the target path, nor overwrites an existing one.
//   - Non-2xx responses surface as UploadToCosError with the server body for
//     diagnostics (covers expired/unauthorized URLs returned by COS).
func DownloadFromCOS(ctx context.Context, downloadURL, targetPath string, overwrite bool) error {
	if !overwrite {
		if _, err := os.Stat(targetPath); err == nil {
			return exception.InvalidArgsError.With(
				"target file already exists: %s (use --overwrite to replace)", targetPath)
		}
	}

	decodedURL, err := url.QueryUnescape(downloadURL)
	if err != nil {
		return exception.InvalidArgsError.With("failed to decode download URL: %v", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, decodedURL, nil)
	if err != nil {
		return exception.InvalidArgsError.With("failed to build COS GET request: %v", err)
	}

	httpClt := &http.Client{Timeout: cosHTTPTimeout}
	resp, err := httpClt.Do(req)
	if err != nil {
		return exception.NetworkError.With("failed to download file from COS: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return exception.UploadToCosError.With(
			"COS download failed, status: %d, body: %s", resp.StatusCode, string(body))
	}

	// Create the temp file in the same directory to ensure rename is atomic
	// on the same filesystem.
	dir := filepath.Dir(targetPath)
	if err = os.MkdirAll(dir, 0o755); err != nil {
		return exception.InvalidArgsError.With("failed to create target dir %s: %v", dir, err)
	}
	tmpFile, err := os.CreateTemp(dir, "."+filepath.Base(targetPath)+".*.tmp")
	if err != nil {
		return exception.InvalidArgsError.With("failed to create temp file: %v", err)
	}
	tmpPath := tmpFile.Name()
	// Always clean up the temp file regardless of success or failure.
	defer func() { _ = os.Remove(tmpPath) }()

	if _, err = io.Copy(tmpFile, resp.Body); err != nil {
		_ = tmpFile.Close()
		return exception.NetworkError.With("failed to write downloaded file: %v", err)
	}
	if err = tmpFile.Close(); err != nil {
		return exception.InvalidArgsError.With("failed to close temp file: %v", err)
	}

	// Atomic replace; rename is atomic on the same filesystem.
	if err = os.Rename(tmpPath, targetPath); err != nil {
		return exception.InvalidArgsError.With("failed to save file to %s: %v", targetPath, err)
	}
	return nil
}

// HexMD5ToBase64 converts a hex-encoded MD5 digest (as returned by CalcFileInfo)
// into the base64 form required by COS's Content-MD5 header.
func HexMD5ToBase64(hexMD5 string) (string, error) {
	raw, err := hex.DecodeString(hexMD5)
	if err != nil {
		return "", fmt.Errorf("decode md5 hex: %w", err)
	}
	return base64.StdEncoding.EncodeToString(raw), nil
}
