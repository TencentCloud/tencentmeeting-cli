package utils

import (
	"encoding/json"
	"fmt"
)

// FilterMeetingsByTimeRange filters items in a JSON array field by a time range.
// data is the raw JSON bytes; listField is the array field name (e.g. "meeting_info_list").
// startTimeField/endTimeField are the timestamp field names inside each item (second-level, may be float64 or string).
// filterStart/filterEnd are the boundary Unix timestamps in seconds; 0 means no limit on that side.
// Filter logic: keep items where start_time >= filterStart AND end_time <= filterEnd (when the corresponding bound is non-zero).
func FilterMeetingsByTimeRange(data []byte, listField, startTimeField, endTimeField string, filterStart, filterEnd int64) []byte {
	var root map[string]interface{}
	if err := json.Unmarshal(data, &root); err != nil {
		return data
	}

	list, ok := root[listField].([]interface{})
	if !ok {
		return data
	}

	filtered := make([]interface{}, 0, len(list))
	for _, item := range list {
		m, ok := item.(map[string]interface{})
		if !ok {
			filtered = append(filtered, item)
			continue
		}

		meetingStart := extractTimestamp(m[startTimeField])
		meetingEnd := extractTimestamp(m[endTimeField])

		// Filter: meeting start_time must be >= filterStart (if filterStart > 0)
		if filterStart > 0 && meetingStart > 0 && meetingStart < filterStart {
			continue
		}
		// Filter: meeting end_time must be <= filterEnd (if filterEnd > 0)
		if filterEnd > 0 && meetingEnd > 0 && meetingEnd > filterEnd {
			continue
		}

		filtered = append(filtered, item)
	}

	root[listField] = filtered
	// Update meeting_number if it exists.
	if _, exists := root["meeting_number"]; exists {
		root["meeting_number"] = len(filtered)
	}

	result, err := json.Marshal(root)
	if err != nil {
		return data
	}
	return result
}

// extractTimestamp extracts a second-level Unix timestamp from an interface{} value.
// Supports float64 and string types, and automatically normalizes millisecond-level timestamps.
func extractTimestamp(v interface{}) int64 {
	var ts int64
	switch val := v.(type) {
	case float64:
		ts = int64(val)
	case string:
		if _, err := fmt.Sscanf(val, "%d", &ts); err != nil {
			return 0
		}
	default:
		return 0
	}
	// Automatically detect millisecond-level timestamps and convert to seconds.
	const millisecondThreshold = int64(1e11)
	if ts > millisecondThreshold {
		ts = ts / 1000
	}
	return ts
}
