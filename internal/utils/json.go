package utils

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
)

// FieldConverter is the field converter function type that accepts a raw value and returns the converted value.
type FieldConverter func(value interface{}) interface{}

// ConvertFields recursively parses JSON data and processes specified fields with custom converters.
// maxDepth limits the maximum recursion depth to prevent infinite recursion.
// fields specifies the field names to convert and their corresponding converter functions.
func ConvertFields(data []byte, maxDepth int, fields map[string]FieldConverter) []byte {
	if maxDepth <= 0 {
		return data
	}
	if fields == nil {
		return data
	}

	var result interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return data
	}

	result = convertRecursive(result, maxDepth, fields)

	updatedData, err := json.Marshal(result)
	if err != nil {
		return data
	}
	return updatedData
}

// convertRecursive recursively processes the data structure.
func convertRecursive(data interface{}, depth int, fields map[string]FieldConverter) interface{} {
	if depth <= 0 {
		return data
	}

	switch v := data.(type) {
	case map[string]interface{}:
		result := make(map[string]interface{})
		for key, value := range v {
			// Check if there is a corresponding converter.
			if converter, ok := fields[key]; ok {
				result[key] = converter(value)
				continue
			}
			// Recursively process the value.
			result[key] = convertRecursive(value, depth-1, fields)
		}
		return result

	case []interface{}:
		result := make([]interface{}, len(v))
		for i, item := range v {
			result[i] = convertRecursive(item, depth-1, fields)
		}
		return result

	default:
		return data
	}
}

// normalizeAndConvertTimestamp automatically detects second-level or millisecond-level timestamps and converts them to ISO8601 format.
// Detection rule: millisecond-level timestamps are usually greater than 1e11 (approximately the millisecond value for year 2001).
func normalizeAndConvertTimestamp(ts int64) string {
	const millisecondThreshold = int64(1e11)
	if ts > millisecondThreshold {
		// Millisecond-level timestamp, convert to second-level.
		ts = ts / 1000
	}
	return TimeStampToISO8601(ts)
}

// TimestampConverter converts float64 or string timestamps to ISO8601 format.
// It can be passed directly to ConvertFields as a FieldConverter.
func TimestampConverter(value interface{}) interface{} {
	switch ts := value.(type) {
	case float64:
		return normalizeAndConvertTimestamp(int64(ts))
	case string:
		var n int64
		if _, err := fmt.Sscanf(ts, "%d", &n); err == nil {
			return normalizeAndConvertTimestamp(n)
		}
	}
	return value
}

// InstanceIdConverter converts instance IDs to their human-readable device type names.
// It can be passed directly to ConvertFields as a FieldConverter.
func InstanceIdConverter(value interface{}) interface{} {
	switch ts := value.(type) {
	case float64:
		return InstanceTypeName(int(ts))
	case string:
		var n int
		if _, err := fmt.Sscanf(ts, "%d", &n); err == nil {
			return InstanceTypeName(n)
		}
	}
	return value
}

// HHMMSSConverter converts float64 or string duration values to HH:MM:SS / MM:SS format.
func HHMMSSConverter(value interface{}) interface{} {
	switch ts := value.(type) {
	case float64:
		return DurationToHMS(int64(ts))
	case string:
		var n int64
		if _, err := fmt.Sscanf(ts, "%d", &n); err == nil {
			return DurationToHMS(n)
		}
	}
	return value
}

// Base64DecodeConverter decodes a Base64-encoded string value to its original string.
// Returns the original value unchanged if decoding fails.
func Base64DecodeConverter(value interface{}) interface{} {
	str, ok := value.(string)
	if !ok {
		return value
	}
	decoded, err := base64.StdEncoding.DecodeString(str)
	if err != nil {
		// Fall back to URL-safe Base64.
		decoded, err = base64.URLEncoding.DecodeString(str)
		if err != nil {
			return value
		}
	}
	return string(decoded)
}

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
