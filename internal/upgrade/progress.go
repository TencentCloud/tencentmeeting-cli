// progress.go — throttled download progress reporting.
//
// A ProgressWriter is an io.Writer that counts bytes streaming past it
// and, every so often, prints a human-readable progress line through a
// caller-supplied Logf callback. It is used from DownloadToTemp as one
// branch of an io.MultiWriter so the hot path stays single-pass.
//
// Decoupled from *cobra.Command on purpose: callers in cmd/update adapt
// output.EventStderr(cmd, ...) into a Logf; callers in other commands
// (or in tests) can plug in anything with a printf-like shape. See
// upgrade.go-level documentation for the Logf contract.

package upgrade

import (
	"fmt"
	"time"
)

// Logf is a printf-style progress / status callback.  Implementations
// MUST be safe to call from the goroutine that drives the upgrade (we
// never fan this out across goroutines inside this package).  A nil
// Logf is treated as "discard" by every consumer here, so callers that
// want silence can pass nil instead of wrapping no-op funcs.
type Logf func(format string, args ...interface{})

// ProgressMinInterval and ProgressMinBytes gate ProgressWriter's
// intermediate writes: a line prints only when BOTH the time delta since
// the last line (>= ProgressMinInterval) AND the byte delta
// (>= ProgressMinBytes) have crossed their thresholds.  This keeps
// progress visible on a slow link without flooding stderr on a fast one.
// A separate "start" / "finish" line always prints unconditionally via
// Finish().
const (
	ProgressMinInterval = 500 * time.Millisecond
	ProgressMinBytes    = int64(256 << 10)
)

// ProgressWriter is an io.Writer that counts bytes and, every so often,
// emits a progress line through logf.  It is NOT safe for concurrent
// use — io.Copy writes sequentially, which is the only intended caller.
//
// Throttling policy: a line prints only when BOTH (now - lastPrint) >=
// ProgressMinInterval AND (total - lastPrintedBytes) >= ProgressMinBytes.
// The dual gate prevents chattiness on fast links (byte gate) while
// still giving a heartbeat on slow links (time gate acts after enough
// bytes).
type ProgressWriter struct {
	logf             Logf
	total            int64
	contentLength    int64 // -1 when unknown (chunked)
	start            time.Time
	lastPrint        time.Time
	lastPrintedBytes int64
	wroteAny         bool
}

// NewProgressWriter returns a ProgressWriter that reports progress via
// logf.  Pass logf=nil to discard output.  contentLength is the expected
// total byte count; pass -1 (what net/http gives you on chunked bodies)
// to let the writer degrade to a "X MiB downloaded" line without
// percentages.
func NewProgressWriter(logf Logf, contentLength int64) *ProgressWriter {
	now := time.Now()
	return &ProgressWriter{
		logf:          logf,
		contentLength: contentLength,
		start:         now,
		lastPrint:     now,
	}
}

// Write records n bytes and may emit an intermediate progress line if
// BOTH throttle gates have cleared.  Always returns len(b) and nil —
// progress reporting never fails the surrounding io.Copy.
func (p *ProgressWriter) Write(b []byte) (int, error) {
	n := len(b)
	p.total += int64(n)
	now := time.Now()
	if now.Sub(p.lastPrint) >= ProgressMinInterval && p.total-p.lastPrintedBytes >= ProgressMinBytes {
		p.emit(now, false)
	}
	return n, nil
}

// Finish prints the terminal progress line with the full byte count and
// elapsed time.  Call once after the download Copy returns, regardless
// of whether any intermediate line was ever emitted (so even a sub-500ms
// download still shows a confirmation line to the user).
func (p *ProgressWriter) Finish(total int64) {
	p.total = total
	p.emit(time.Now(), true)
}

// emit writes one progress line through logf.  When logf is nil the
// call is a no-op (but we still update the "last printed" bookkeeping
// so a later swap of logf would not emit a flood of stale lines).
func (p *ProgressWriter) emit(now time.Time, final bool) {
	elapsed := now.Sub(p.start)
	if elapsed <= 0 {
		elapsed = time.Millisecond // avoid divide-by-zero on the "finished instantly" path
	}
	speed := float64(p.total) / elapsed.Seconds() / (1 << 20) // MiB/s
	if p.logf != nil {
		switch {
		case p.contentLength > 0:
			pct := float64(p.total) * 100 / float64(p.contentLength)
			if final {
				p.logf("Downloaded %s (100%%) in %s (%.2f MiB/s)",
					HumanBytes(p.total), elapsed.Round(time.Millisecond), speed)
			} else {
				p.logf("  %s / %s (%.1f%%) at %.2f MiB/s",
					HumanBytes(p.total), HumanBytes(p.contentLength), pct, speed)
			}
		default:
			if final {
				p.logf("Downloaded %s in %s (%.2f MiB/s)",
					HumanBytes(p.total), elapsed.Round(time.Millisecond), speed)
			} else {
				p.logf("  %s at %.2f MiB/s",
					HumanBytes(p.total), speed)
			}
		}
	}
	p.lastPrint = now
	p.lastPrintedBytes = p.total
	p.wroteAny = true
}

// HumanBytes formats a byte count with KiB/MiB/GiB units, picking the
// smallest unit that keeps the number in [1, 1024).  Precision is
// capped at 2 decimals for readability.
func HumanBytes(n int64) string {
	const unit = int64(1024)
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := unit, 0
	for x := n / unit; x >= unit; x /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}
