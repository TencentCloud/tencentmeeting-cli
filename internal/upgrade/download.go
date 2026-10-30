// download.go — HTTP download + checksum helpers for the upgrade flow.
//
// Everything here is stateless and side-effect-only via the filesystem
// (temp files) and the caller-supplied Logf (progress / warnings).  No
// cobra, no notice, no package-level globals: the upgrade package can
// be lifted into another command (or a test harness) without dragging
// cmd/update along.

package upgrade

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"tmeet/internal/exception"
)

// DownloadTimeout is the recommended per-call wall-clock deadline for
// DownloadToTemp when fetching the CLI tarball or the skills archive.
// Exported so the cmd/update layer and tests apply the same ceiling
// the production flow uses.
//
// Ten minutes is sized for the worst realistic foreground case: a
// ~50MiB payload over a weak link (hotel Wi-Fi, congested 4G, cross-
// region egress) sustaining only ~100 KiB/s.  `tmeet update run` is
// an interactive command, so biasing toward "let a slow user finish"
// over "fail fast on a healthy network" is the right trade-off — a
// stalled connection is caught much sooner by DownloadIdleTimeout
// below, so this ceiling only trips on genuinely-slow-but-progressing
// transfers.
const DownloadTimeout = 10 * time.Minute

// DownloadIdleTimeout bounds how long DownloadToTemp will wait for
// the next byte of body once the response is streaming.  It is NOT
// the whole-request deadline (DownloadTimeout handles that); it is
// the "the server went silent" tripwire that lets a dead TCP
// connection fail fast on a healthy client instead of waiting out
// the full wall-clock timeout.
//
// Sixty seconds is enough to ride through a brief network blip or a
// slow CDN edge warming up, while still surfacing a hung server /
// black-holed route long before DownloadTimeout fires.
const DownloadIdleTimeout = 60 * time.Second

// downloadClient is the shared http.Client used by DownloadToTemp.
// It is kept at package scope so connections can be pooled across
// the (at most two) downloads a single `tmeet update run` performs.
//
// Timeouts here are deliberately NOT the whole-request timeout —
// that lives on the caller's context so Ctrl-C still cancels mid-
// transfer.  These bound only the handshake / header / idle-conn
// phases so a stuck TLS or stuck "waiting for headers" fails without
// us having to wait out DownloadTimeout.
var downloadClient = &http.Client{
	Transport: &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          16,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   15 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		// ResponseHeaderTimeout bounds the "sent request, waiting
		// for the first response byte" phase.  Once headers are in
		// we switch to idleTimeoutReader for per-read enforcement.
		ResponseHeaderTimeout: 30 * time.Second,
	},
}

// idleTimeoutReader wraps an io.ReadCloser and cancels the owning
// context (via cancel) if no Read makes progress for longer than
// idle.  It is cheaper than resp.Body.SetReadDeadline (which net/http
// does not expose on the body) and plays nicely with io.Copy — a
// cancelled ctx surfaces as a Read error on the underlying body on
// the next network read, which io.Copy returns unchanged.
//
// Correctness notes:
//   - The timer is reset on every successful Read, so a steady
//     trickle of bytes keeps the connection alive indefinitely (up
//     to the caller's wall-clock deadline).
//   - Stop() races with the timer's AfterFunc goroutine; both paths
//     are idempotent because cancel() is a no-op on repeat calls.
//   - stopped is checked atomically so Close() after a natural EOF
//     does not spuriously cancel an unrelated reuse of ctx.
type idleTimeoutReader struct {
	rc      io.ReadCloser
	timer   *time.Timer
	idle    time.Duration
	cancel  context.CancelFunc
	stopped atomic.Bool
}

func newIdleTimeoutReader(rc io.ReadCloser, idle time.Duration, cancel context.CancelFunc) *idleTimeoutReader {
	r := &idleTimeoutReader{rc: rc, idle: idle, cancel: cancel}
	r.timer = time.AfterFunc(idle, func() {
		if !r.stopped.Load() {
			cancel()
		}
	})
	return r
}

func (r *idleTimeoutReader) Read(p []byte) (int, error) {
	n, err := r.rc.Read(p)
	if n > 0 {
		// Reset the idle window on every byte of progress.  Reset's
		// return value is intentionally ignored: a racing fire from
		// AfterFunc is harmless once we have made forward progress
		// (the ctx cancellation would only matter on the NEXT read,
		// and we are about to reset the timer again anyway).
		r.timer.Reset(r.idle)
	}
	return n, err
}

func (r *idleTimeoutReader) Close() error {
	r.stopped.Store(true)
	r.timer.Stop()
	return r.rc.Close()
}

// DownloadToTemp streams url into a tempfile under dir, returning the
// path, byte count, hex-encoded SHA-256 of the body, and any error.
// Pass dir="" to use the OS temp dir.  The tempfile is NOT cleaned up
// on error — the caller's deferred os.Remove handles that so a partial
// file never races with the final consumer.
//
// The suffix argument controls the tempfile extension.  Pass ".tgz"
// when the download will be fed to `npm install -g <path>` (npm sniffs
// the filename to decide tarball mode); pass ".zip" for a zip archive;
// anything else is accepted verbatim.  An empty suffix defaults to
// ".tgz" so legacy callers keep their old behaviour.
//
// The body is tee'd through three sinks in one pass: the on-disk
// tempfile, a SHA-256 hasher (so the caller can verify integrity
// without re-reading the file), and a ProgressWriter that emits a
// throttled percentage line through logf.  Hashing inline (rather than
// re-reading the file after the download) keeps the hot path O(1) in
// extra reads.  Pass logf=nil to run silently.
//
// Timeout model: the caller's ctx carries the wall-clock deadline
// (typically DownloadTimeout).  On top of that, DownloadToTemp
// enforces an idle timeout per read (DownloadIdleTimeout) by wrapping
// resp.Body, so a stalled connection fails in O(seconds) instead of
// having to burn the full wall-clock budget.  A slow-but-progressing
// transfer is NOT penalised — every byte of progress resets the idle
// window.
func DownloadToTemp(ctx context.Context, logf Logf, url, dir, suffix string) (string, int64, string, error) {
	// Derive a cancellable child ctx so the per-read idle watchdog
	// can abort the in-flight request without affecting the caller's
	// own ctx (e.g. we must not cancel cmd.Context()).
	reqCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
	if err != nil {
		return "", 0, "", exception.UpdateDownloadError.With("build download request: %v", err)
	}
	resp, err := downloadClient.Do(req)
	if err != nil {
		// Surface a clearer message when the parent ctx expired
		// (wall-clock deadline) vs. a transport-level failure.
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return "", 0, "", exception.UpdateDownloadError.With(
				"download %s timed out after %s: %v", url, DownloadTimeout, err)
		}
		return "", 0, "", exception.UpdateDownloadError.With("download %s: %v", url, err)
	}
	// Wrap the body so inactivity (no Read progress for
	// DownloadIdleTimeout) cancels the request context, which
	// propagates to the next Read as an error and unwinds io.Copy.
	body := newIdleTimeoutReader(resp.Body, DownloadIdleTimeout, cancel)
	defer func() { _ = body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", 0, "", exception.UpdateDownloadError.With("download %s: unexpected status %d", url, resp.StatusCode)
	}

	if suffix == "" {
		suffix = ".tgz"
	}
	tmp, err := os.CreateTemp(dir, "tmeet-update-*"+suffix)
	if err != nil {
		return "", 0, "", exception.UpdateTempFileError.With("create tempfile: %v", err)
	}
	tmpPath := tmp.Name()

	// resp.ContentLength is -1 when the server did not send a
	// Content-Length header (chunked transfer).  ProgressWriter
	// degrades gracefully: it falls back to "X MiB downloaded" with no
	// percentage.
	hasher := sha256.New()
	prog := NewProgressWriter(logf, resp.ContentLength)

	// Order inside MultiWriter does not matter semantically, but
	// putting the file first means that if an interrupt happens
	// mid-copy the bytes we have are already on disk (so cleanup via
	// os.Remove is definite rather than racey).
	n, copyErr := io.Copy(io.MultiWriter(tmp, hasher, prog), body)
	prog.Finish(n)
	syncErr := tmp.Sync()
	closeErr := tmp.Close()
	if copyErr != nil {
		// Classify the copy failure for a more actionable message:
		// an idle-timeout cancel manifests as a ctx error on reqCtx
		// but NOT on the parent ctx (which is still live).  The
		// parent-ctx deadline path is distinguished separately so
		// users can tell "stalled mid-transfer" apart from "we hit
		// the overall wall clock".
		switch {
		case errors.Is(ctx.Err(), context.DeadlineExceeded):
			return tmpPath, n, "", exception.UpdateDownloadError.With(
				"download %s exceeded %s wall-clock deadline after %d bytes: %v",
				url, DownloadTimeout, n, copyErr)
		case reqCtx.Err() != nil && ctx.Err() == nil:
			return tmpPath, n, "", exception.UpdateDownloadError.With(
				"download %s stalled for more than %s after %d bytes: %v",
				url, DownloadIdleTimeout, n, copyErr)
		}
		return tmpPath, n, "", exception.UpdateDownloadError.With("write tempfile: %v", copyErr)
	}
	if syncErr != nil {
		return tmpPath, n, "", exception.UpdateTempFileError.With("sync tempfile: %v", syncErr)
	}
	if closeErr != nil {
		return tmpPath, n, "", exception.UpdateTempFileError.With("close tempfile: %v", closeErr)
	}
	return tmpPath, n, hex.EncodeToString(hasher.Sum(nil)), nil
}

// VerifyChecksum compares the hex-encoded SHA-256 computed during
// download against the expected value supplied by the version-info API.
// Accepts either bare hex ("a1b2...") or an OCI-style "sha256:..."
// prefix.  The server contract is bare hex today, but tolerating both
// forms avoids a cross-team sync if the wire format ever grows the
// prefix.
//
// Empty expected → the server did not ship a checksum for this
// release.  Integrity verification is mandatory, so we refuse to
// install rather than silently trusting the bytes we just pulled off
// the network.  Pass logf=nil to suppress logging; the error is
// returned regardless.
func VerifyChecksum(got, want string) error {
	want = strings.TrimSpace(want)
	if want == "" {
		return exception.UpdateChecksumError.With(
			"server did not provide a checksum for this release (got sha256=%s); refusing to install",
			got)
	}
	want = strings.TrimPrefix(want, "sha256:")
	want = strings.TrimPrefix(want, "SHA256:")
	if !strings.EqualFold(got, want) {
		return exception.UpdateChecksumError.With(
			"downloaded tarball failed checksum verification (expected sha256=%s, got sha256=%s); refusing to install",
			want, got)
	}
	return nil
}
