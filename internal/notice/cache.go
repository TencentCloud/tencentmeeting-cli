package notice

import (
	"encoding/json"
	"path/filepath"
	"time"

	"tmeet/internal/config"
	"tmeet/internal/core/filecache"
)

// userDir returns the per-account data directory used by both the
// last-notice sentinel and the preference file.
//
// The directory is NOT created eagerly: filecache.Set runs os.MkdirAll(0700)
// as part of its write path, so the first successful Save* call creates
// everything. This keeps the read path side-effect free for brand-new
// accounts that have never been written to.
//
// OpenId is validated through config.ValidateOpenId to guard against path
// traversal — it is going to appear as a filesystem path segment.
func userDir(openId string) (string, error) {
	if err := config.ValidateOpenId(openId); err != nil {
		return "", err
	}
	return filepath.Join(config.GetConfigDir(), "data", openId), nil
}

// newCache builds the per-account filecache instance. filecache.New itself
// does zero I/O, so it is cheap to call on every access and keeps the API
// stateless.
func newCache(openId string) (*filecache.Cache, error) {
	dir, err := userDir(openId)
	if err != nil {
		return nil, err
	}
	return filecache.New(dir), nil
}

// isNoticeFresh reports whether a prior proactive notice is still inside
// its gap window for openId.
//
// The function is the single throttle gate consulted by BuildWithOutcome
// (see builder.go). Any error / corruption / missing file degrades to
// "not fresh" so the system fails open: at worst the user sees one extra
// prompt. filecache already treats a corrupted envelope as a miss, we
// just propagate that signal verbatim.
func isNoticeFresh(openId string) bool {
	fc, err := newCache(openId)
	if err != nil {
		return false
	}
	_, fresh, _ := fc.Get(keyLastNotice)
	return fresh
}

// markNotified records that a proactive notice was just emitted for
// openId, writing last_notice.json with TTL = noticeGap. A non-positive
// noticeGap falls back to defaultNoticeGap, matching the pre-notice_gap
// cadence.
//
// Best-effort: a write failure means the user may see one extra prompt
// later, which is strictly less harmful than aborting a business command
// over notice bookkeeping. The error is therefore swallowed by design.
func markNotified(openId string, noticeGap int64) {
	gap := time.Duration(noticeGap) * time.Second
	if gap <= 0 {
		gap = defaultNoticeGap
	}
	fc, err := newCache(openId)
	if err != nil {
		return
	}
	data, err := json.Marshal(&LastNotice{At: time.Now().Unix()})
	if err != nil {
		return
	}
	_ = fc.Set(keyLastNotice, data, gap)
}

// clearLastNotice removes the throttle sentinel for openId. Used by
// `tmeet update {run,check} --force` so a forced refresh also forces the
// NEXT proactive notice to go through, matching the user expectation
// that --force "starts from scratch". Idempotent — filecache.Delete is a
// no-op when the entry does not exist.
func clearLastNotice(openId string) error {
	fc, err := newCache(openId)
	if err != nil {
		return err
	}
	return fc.Delete(keyLastNotice)
}
