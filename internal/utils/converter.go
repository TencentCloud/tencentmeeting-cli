package utils

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"tmeet/internal/utils/enumerate"
)

// FieldConverter is the field converter function type that accepts a raw value and returns the converted value.
type FieldConverter func(value interface{}) interface{}

// ConvertFields recursively parses JSON data and processes specified fields with custom converters.
// maxDepth limits the maximum recursion depth to prevent infinite recursion.
//
// The key of fields supports two modes:
//   - Field name mode (without "."): e.g. "start_time", recursively matches all fields with the same name
//     in the entire JSON tree, limited by maxDepth.
//   - Path mode (with "."): e.g. "meeting.recurring_rule.recurring_type", matches by exact path,
//     not limited by maxDepth. Each segment in the path corresponds to a JSON object key,
//     and arrays are automatically expanded (each element continues matching the remaining path).
func ConvertFields(data []byte, maxDepth int, fields map[string]FieldConverter) []byte {
	if maxDepth <= 0 {
		return data
	}
	if fields == nil {
		return data
	}

	// Split fields into field-name mode and path mode groups.
	nameFields := make(map[string]FieldConverter)
	pathFields := make(map[string]FieldConverter)
	for key, converter := range fields {
		if strings.Contains(key, ".") {
			pathFields[key] = converter
		} else {
			nameFields[key] = converter
		}
	}

	var result interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return data
	}

	// First, perform recursive conversion for field-name mode.
	if len(nameFields) > 0 {
		result = convertRecursive(result, maxDepth, nameFields)
	}

	// Then, perform exact conversion for path mode.
	for path, converter := range pathFields {
		parts := strings.Split(path, ".")
		result = convertByPath(result, parts, converter)
	}

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

// convertByPath locates and converts the target field by exact path, automatically expanding arrays
// to continue matching each element.
func convertByPath(data interface{}, pathParts []string, converter FieldConverter) interface{} {
	if len(pathParts) == 0 {
		return data
	}

	switch v := data.(type) {
	case map[string]interface{}:
		currentKey := pathParts[0]
		value, exists := v[currentKey]
		if !exists {
			return data
		}

		if len(pathParts) == 1 {
			// Reached the end of the path, apply the converter.
			v[currentKey] = converter(value)
		} else {
			// Continue traversing along the path.
			v[currentKey] = convertByPath(value, pathParts[1:], converter)
		}
		return v

	case []interface{}:
		// Auto-expand array: continue matching the current path for each element.
		for i, item := range v {
			v[i] = convertByPath(item, pathParts, converter)
		}
		return v

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
var InstanceIdConverter = intEnumConverter(func(n int) string { return enumerate.InstanceTypeName(n) })

// HHMMSSConverter converts float64 or string duration values to HH:MM:SS / MM:SS format.
var HHMMSSConverter FieldConverter = func(value interface{}) interface{} {
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

// intEnumConverter is a helper that builds a FieldConverter for integer-keyed enums.
// nameFunc maps an int ID to its human-readable name.
func intEnumConverter(nameFunc func(int) string) FieldConverter {
	return func(value interface{}) interface{} {
		switch ts := value.(type) {
		case float64:
			return nameFunc(int(ts))
		case string:
			var n int
			if _, err := fmt.Sscanf(ts, "%d", &n); err == nil {
				return nameFunc(n)
			}
		}
		return value
	}
}

// stringEnumConverter is a helper that builds a FieldConverter for string-keyed enums.
// nameFunc maps a string key to its human-readable name.
func stringEnumConverter(nameFunc func(string) string) FieldConverter {
	return func(value interface{}) interface{} {
		if ts, ok := value.(string); ok {
			return nameFunc(ts)
		}
		return value
	}
}

// RecordTypeConverter converts record type IDs to their human-readable record type names.
var RecordTypeConverter = intEnumConverter(enumerate.RecordTypeName)

// RecordStateConverter converts record state IDs to their human-readable record state names.
var RecordStateConverter = intEnumConverter(enumerate.RecordStateName)

// RecordAudioDetectConverter converts record audio detect IDs to their human-readable record audio detect names.
var RecordAudioDetectConverter = intEnumConverter(enumerate.RecordAudioDetectName)

// MeetingTypeConverter converts meeting type IDs to their human-readable meeting type names.
var MeetingTypeConverter = intEnumConverter(enumerate.MeetingTypeName)

// MeetingRecurringTypeConverter converts meeting recurring type IDs to their human-readable meeting recurring type names.
var MeetingRecurringTypeConverter = intEnumConverter(enumerate.MeetingRecurringTypeName)

// MeetingRecurringUntilTypeConverter converts meeting recurring until type IDs to their human-readable meeting recurring until type names.
var MeetingRecurringUntilTypeConverter = intEnumConverter(enumerate.MeetingRecurringUntilTypeName)

// MeetingUserRoleConverter converts meeting user role IDs to their human-readable meeting user role names.
var MeetingUserRoleConverter = intEnumConverter(enumerate.MeetingUserRoleName)

// MeetingStatusConverter converts meeting status strings to their human-readable meeting status names.
var MeetingStatusConverter = stringEnumConverter(enumerate.MeetingStatusName)
