package cmdutil

import (
	"encoding/json"
	"fmt"

	"tmeet/internal/utils/enumerate"
)

// GenerateMeetingSettingsHints generates hints purely based on the corp_lock_mask
func GenerateMeetingSettingsHints(responseData string) []string {
	if responseData == "" {
		return nil
	}

	var resp map[string]interface{}
	if err := json.Unmarshal([]byte(responseData), &resp); err != nil {
		return nil
	}

	// Collect every settings map present in the response.
	settingsList := extractSettingsList(resp)
	if len(settingsList) == 0 {
		return nil
	}

	seen := make(map[string]struct{})
	var result []string
	for _, settings := range settingsList {
		mask, ok := toUint32(settings["corp_lock_mask"])
		if !ok || mask == 0 {
			continue
		}
		for _, label := range enumerate.CorpLockMaskNames(mask) {
			line := fmt.Sprintf("Enterprise has locked the [%s] setting", label)
			if _, dup := seen[line]; dup {
				continue
			}
			seen[line] = struct{}{}
			result = append(result, line)
		}
	}
	return result
}

// extractSettingsList collects all `settings` maps from the response, supporting
// both the top-level shape and the meeting_info_list[].settings shape.
func extractSettingsList(resp map[string]interface{}) []map[string]interface{} {
	var out []map[string]interface{}

	if s, ok := resp["settings"].(map[string]interface{}); ok {
		out = append(out, s)
	}

	if list, ok := resp["meeting_info_list"].([]interface{}); ok {
		for _, item := range list {
			m, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			if s, ok := m["settings"].(map[string]interface{}); ok {
				out = append(out, s)
			}
		}
	}

	return out
}

// toUint32 converts numeric values that may come from JSON unmarshaling to uint32.
func toUint32(v interface{}) (uint32, bool) {
	switch n := v.(type) {
	case float64:
		return uint32(n), true
	case int:
		return uint32(n), true
	case int32:
		return uint32(n), true
	case uint32:
		return n, true
	default:
		return 0, false
	}
}
