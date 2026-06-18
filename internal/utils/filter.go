package utils

import (
	"encoding/json"
	"fmt"
	"strings"
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

// KeepFields recursively parses JSON data and keeps only the specified fields (whitelist mode);
// all other fields will be removed.
// maxDepth limits the maximum recursion depth to prevent infinite recursion.
//
// Each entry in fields supports two modes:
//   - Field name mode (without "."): e.g. "start_time", any field with the same name anywhere
//     in the JSON tree will be kept (together with its whole subtree), limited by maxDepth.
//   - Path mode (with "."): e.g. "meeting.recurring_rule.recurring_type", keeps the target
//     field by exact path. Parent objects along the path are preserved, but their sibling keys
//     that are not in the whitelist will be removed. Arrays are automatically expanded (each
//     element continues matching the remaining path).
//
// When both modes are mixed, a field is kept if it matches either rule.
func KeepFields(data []byte, maxDepth int, fields []string) []byte {
	if maxDepth <= 0 || len(fields) == 0 {
		return data
	}

	// Split fields into field-name mode and path mode groups.
	nameFields := make(map[string]struct{})
	pathTree := make(map[string]interface{})
	for _, key := range fields {
		if key == "" {
			continue
		}
		if strings.Contains(key, ".") {
			addPathToTree(pathTree, strings.Split(key, "."))
		} else {
			nameFields[key] = struct{}{}
		}
	}

	var result interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return data
	}

	result = keepRecursive(result, maxDepth, nameFields, pathTree)

	updatedData, err := json.Marshal(result)
	if err != nil {
		return data
	}
	return updatedData
}

// addPathToTree inserts a path (as segments) into the keep-tree. A leaf is marked with a nil
// value, meaning "keep this node and its entire subtree".
func addPathToTree(tree map[string]interface{}, parts []string) {
	current := tree
	for i, part := range parts {
		if i == len(parts)-1 {
			// Leaf node: mark as nil so the whole subtree is kept.
			current[part] = nil
			return
		}
		next, ok := current[part].(map[string]interface{})
		if !ok {
			// If an earlier rule already marked this as a leaf (nil), keep it as a leaf
			// because a shorter whitelist path takes precedence (keeps the whole subtree).
			if _, isLeaf := current[part]; isLeaf && current[part] == nil {
				return
			}
			next = make(map[string]interface{})
			current[part] = next
		}
		current = next
	}
}

// keepRecursive walks the data tree and keeps only the keys that match the whitelist.
// nameFields: recursive name-based whitelist.
// pathTree:   path-based whitelist rooted at the current position.
func keepRecursive(data interface{}, depth int, nameFields map[string]struct{},
	pathTree map[string]interface{}) interface{} {
	if depth <= 0 {
		return data
	}

	switch v := data.(type) {
	case map[string]interface{}:
		for key, value := range v {
			// Rule 1: matched by name-based whitelist, keep the whole subtree as-is.
			if _, ok := nameFields[key]; ok {
				continue
			}

			// Rule 2: matched by path-based whitelist.
			if sub, ok := pathTree[key]; ok {
				if sub == nil {
					// Leaf in the keep-tree: keep the whole subtree as-is.
					continue
				}
				subTree, _ := sub.(map[string]interface{})
				v[key] = keepRecursive(value, depth-1, nameFields, subTree)
				continue
			}

			// Rule 3: neither matched; recurse so that deeper name-based matches can still
			// preserve descendant fields. If nothing is preserved inside, remove the key.
			if hasNameMatchInside(value, depth-1, nameFields) {
				v[key] = keepRecursive(value, depth-1, nameFields, nil)
				continue
			}
			delete(v, key)
		}
		return v

	case []interface{}:
		for i, item := range v {
			v[i] = keepRecursive(item, depth-1, nameFields, pathTree)
		}
		return v

	default:
		return data
	}
}

// hasNameMatchInside reports whether the given subtree contains any key matching the
// name-based whitelist within the remaining depth. It avoids needlessly keeping empty
// parent objects that have no whitelisted descendants.
func hasNameMatchInside(data interface{}, depth int, nameFields map[string]struct{}) bool {
	if depth <= 0 || len(nameFields) == 0 {
		return false
	}
	switch v := data.(type) {
	case map[string]interface{}:
		for key, value := range v {
			if _, ok := nameFields[key]; ok {
				return true
			}
			if hasNameMatchInside(value, depth-1, nameFields) {
				return true
			}
		}
	case []interface{}:
		for _, item := range v {
			if hasNameMatchInside(item, depth-1, nameFields) {
				return true
			}
		}
	}
	return false
}

// DeleteFields recursively parses JSON data and removes the specified fields (blacklist mode);
// all other fields will be preserved. It is the dual of KeepFields.
// maxDepth limits the maximum recursion depth to prevent infinite recursion.
//
// Each entry in fields supports two modes:
//   - Field name mode (without "."): e.g. "secret", any field with the same name anywhere
//     in the JSON tree will be removed (together with its whole subtree), limited by maxDepth.
//   - Path mode (with "."): e.g. "meeting.recurring_rule.recurring_type", removes the target
//     field by exact path. Parent objects along the path are preserved as-is (only the target
//     leaf is removed). Arrays are automatically expanded (each element continues matching the
//     remaining path).
//
// When both modes are mixed, a field is removed if it matches either rule.
func DeleteFields(data []byte, maxDepth int, fields []string) []byte {
	if maxDepth <= 0 || len(fields) == 0 {
		return data
	}

	// Split fields into field-name mode and path mode groups.
	nameFields := make(map[string]struct{})
	pathTree := make(map[string]interface{})
	for _, key := range fields {
		if key == "" {
			continue
		}
		if strings.Contains(key, ".") {
			addPathToTree(pathTree, strings.Split(key, "."))
		} else {
			nameFields[key] = struct{}{}
		}
	}

	if len(nameFields) == 0 && len(pathTree) == 0 {
		return data
	}

	var result interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return data
	}

	result = deleteRecursive(result, maxDepth, nameFields, pathTree)

	updatedData, err := json.Marshal(result)
	if err != nil {
		return data
	}
	return updatedData
}

// deleteRecursive walks the data tree and removes the keys that match the blacklist.
// nameFields: recursive name-based blacklist.
// pathTree:   path-based blacklist rooted at the current position. A leaf (nil value) means
// "delete this node entirely".
func deleteRecursive(data interface{}, depth int, nameFields map[string]struct{},
	pathTree map[string]interface{}) interface{} {
	if depth <= 0 {
		return data
	}

	switch v := data.(type) {
	case map[string]interface{}:
		for key, value := range v {
			// Rule 1: matched by name-based blacklist, drop the whole subtree.
			if _, ok := nameFields[key]; ok {
				delete(v, key)
				continue
			}

			// Rule 2: matched by path-based blacklist.
			if sub, ok := pathTree[key]; ok {
				if sub == nil {
					// Leaf in the delete-tree: drop this node entirely.
					delete(v, key)
					continue
				}
				subTree, _ := sub.(map[string]interface{})
				v[key] = deleteRecursive(value, depth-1, nameFields, subTree)
				continue
			}

			// Rule 3: not in path-tree at this level; still recurse so deeper name-based
			// matches can take effect on descendants.
			v[key] = deleteRecursive(value, depth-1, nameFields, nil)
		}
		return v

	case []interface{}:
		for i, item := range v {
			// Path-tree applies to each array element when arrays are auto-expanded.
			v[i] = deleteRecursive(item, depth-1, nameFields, pathTree)
		}
		return v

	default:
		return data
	}
}
