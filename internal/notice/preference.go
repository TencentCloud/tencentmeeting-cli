package notice

import (
	"encoding/json"
	"time"
)

// loadPreference returns the user's locally saved update preference, or an
// empty *Preference (Value == "") when nothing is on file.
//
// The function never returns an error: a missing / corrupted / unexpired
// file all degrade to "unset", which is the exact state Build needs to
// branch on. Surfacing I/O errors to the caller would force every call
// site to replicate the same "treat as unset" handling, so we centralise
// it here.
func loadPreference(openId string) *Preference {
	fc, err := newCache(openId)
	if err != nil {
		return &Preference{}
	}
	raw, fresh, err := fc.Get(keyPreference)
	if err != nil || !fresh || len(raw) == 0 {
		return &Preference{}
	}
	var p Preference
	if err := json.Unmarshal(raw, &p); err != nil {
		return &Preference{}
	}
	return &p
}

// savePreference overwrites the on-disk preference for openId.
//
// Uses the "effectively never expire" TTL (noExpire) so we can reuse
// filecache's cross-process locking + atomic rename semantics without
// patching the core package to understand a native "no TTL" flag.
func savePreference(openId, value string) error {
	fc, err := newCache(openId)
	if err != nil {
		return err
	}
	data, err := json.Marshal(&Preference{
		Value:     value,
		UpdatedAt: time.Now().Unix(),
	})
	if err != nil {
		return err
	}
	return fc.Set(keyPreference, data, noExpire)
}
