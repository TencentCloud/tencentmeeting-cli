package utils

import (
	"encoding/json"
	"testing"
)

// ---- Helper functions ----

// parseFilteredList parses the result JSON and returns a slice of maps for the given listField array
func parseFilteredList(t *testing.T, result []byte, listField string) []map[string]interface{} {
	t.Helper()
	var root map[string]interface{}
	if err := json.Unmarshal(result, &root); err != nil {
		t.Fatalf("result is not valid JSON: %v, raw: %s", err, string(result))
	}
	list, ok := root[listField].([]interface{})
	if !ok {
		return nil
	}
	out := make([]map[string]interface{}, 0, len(list))
	for _, item := range list {
		if m, ok := item.(map[string]interface{}); ok {
			out = append(out, m)
		}
	}
	return out
}

// meetingNumber reads the meeting_number field from the result JSON
func meetingNumber(t *testing.T, result []byte) int {
	t.Helper()
	var root map[string]interface{}
	if err := json.Unmarshal(result, &root); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}
	v, ok := root["meeting_number"]
	if !ok {
		return -1
	}
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	}
	return -1
}

// ---- TestFilterMeetingsByTimeRange ----

func TestFilterMeetingsByTimeRange(t *testing.T) {
	// Base time: 2026-03-12T14:00:00+08:00 = 1741766400
	const (
		t1 = int64(1741766400) // meeting1 start
		t2 = int64(1741770000) // meeting1 end (+1h)
		t3 = int64(1741780000) // meeting2 start
		t4 = int64(1741790000) // meeting2 end
		t5 = int64(1741800000) // meeting3 start
		t6 = int64(1741810000) // meeting3 end
	)

	// Build JSON with 3 meetings (start_time/end_time as string)
	makeJSON := func(items []map[string]interface{}) []byte {
		root := map[string]interface{}{
			"meeting_info_list": items,
			"meeting_number":    len(items),
		}
		b, _ := json.Marshal(root)
		return b
	}

	threeItems := []map[string]interface{}{
		{"meeting_id": "1", "start_time": "1741766400", "end_time": "1741770000"},
		{"meeting_id": "2", "start_time": "1741780000", "end_time": "1741790000"},
		{"meeting_id": "3", "start_time": "1741800000", "end_time": "1741810000"},
	}

	tests := []struct {
		name        string
		data        []byte
		listField   string
		startField  string
		endField    string
		filterStart int64
		filterEnd   int64
		wantCount   int
		wantIDs     []string // expected meeting_ids to be retained
	}{
		{
			name:        "no filter condition (keep all)",
			data:        makeJSON(threeItems),
			listField:   "meeting_info_list",
			startField:  "start_time",
			endField:    "end_time",
			filterStart: 0,
			filterEnd:   0,
			wantCount:   3,
			wantIDs:     []string{"1", "2", "3"},
		},
		{
			name:        "filterStart only (drop meetings before t3)",
			data:        makeJSON(threeItems),
			listField:   "meeting_info_list",
			startField:  "start_time",
			endField:    "end_time",
			filterStart: t3,
			filterEnd:   0,
			wantCount:   2,
			wantIDs:     []string{"2", "3"},
		},
		{
			name:        "filterEnd only (drop meetings ending after t4)",
			data:        makeJSON(threeItems),
			listField:   "meeting_info_list",
			startField:  "start_time",
			endField:    "end_time",
			filterStart: 0,
			filterEnd:   t4,
			wantCount:   2,
			wantIDs:     []string{"1", "2"},
		},
		{
			name:        "both filterStart and filterEnd (keep middle meeting only)",
			data:        makeJSON(threeItems),
			listField:   "meeting_info_list",
			startField:  "start_time",
			endField:    "end_time",
			filterStart: t3,
			filterEnd:   t4,
			wantCount:   1,
			wantIDs:     []string{"2"},
		},
		{
			name:        "filter range covers all (keep all)",
			data:        makeJSON(threeItems),
			listField:   "meeting_info_list",
			startField:  "start_time",
			endField:    "end_time",
			filterStart: t1,
			filterEnd:   t6,
			wantCount:   3,
			wantIDs:     []string{"1", "2", "3"},
		},
		{
			name:        "filter range matches no meeting (drop all)",
			data:        makeJSON(threeItems),
			listField:   "meeting_info_list",
			startField:  "start_time",
			endField:    "end_time",
			filterStart: t6 + 1,
			filterEnd:   t6 + 3600,
			wantCount:   0,
			wantIDs:     []string{},
		},
		{
			name: "start_time as float64 (numeric JSON)",
			data: func() []byte {
				items := []map[string]interface{}{
					{"meeting_id": "A", "start_time": float64(t1), "end_time": float64(t2)},
					{"meeting_id": "B", "start_time": float64(t3), "end_time": float64(t4)},
				}
				return makeJSON(items)
			}(),
			listField:   "meeting_info_list",
			startField:  "start_time",
			endField:    "end_time",
			filterStart: t3,
			filterEnd:   0,
			wantCount:   1,
			wantIDs:     []string{"B"},
		},
		{
			name: "millisecond timestamp auto-normalization",
			data: func() []byte {
				items := []map[string]interface{}{
					// millisecond: t1*1000 and t2*1000
					{"meeting_id": "ms1", "start_time": float64(t1 * 1000), "end_time": float64(t2 * 1000)},
					{"meeting_id": "ms2", "start_time": float64(t3 * 1000), "end_time": float64(t4 * 1000)},
				}
				return makeJSON(items)
			}(),
			listField:   "meeting_info_list",
			startField:  "start_time",
			endField:    "end_time",
			filterStart: t3, // second-level filter condition
			filterEnd:   0,
			wantCount:   1,
			wantIDs:     []string{"ms2"},
		},
		{
			name:        "meeting_number field synced after filter",
			data:        makeJSON(threeItems),
			listField:   "meeting_info_list",
			startField:  "start_time",
			endField:    "end_time",
			filterStart: t3,
			filterEnd:   0,
			wantCount:   2,
			wantIDs:     []string{"2", "3"},
		},
		{
			name: "listField not found returns original data",
			data: func() []byte {
				b, _ := json.Marshal(map[string]interface{}{"other_field": "value"})
				return b
			}(),
			listField:   "meeting_info_list",
			startField:  "start_time",
			endField:    "end_time",
			filterStart: t1,
			filterEnd:   t6,
			wantCount:   -1, // marker: skip list check, verify original data returned
		},
		{
			name:        "invalid JSON returns original data",
			data:        []byte(`{invalid json}`),
			listField:   "meeting_info_list",
			startField:  "start_time",
			endField:    "end_time",
			filterStart: t1,
			filterEnd:   t6,
			wantCount:   -2, // marker: verify original data returned
		},
		{
			name: "item with missing timestamp fields is retained",
			data: func() []byte {
				items := []map[string]interface{}{
					{"meeting_id": "no_time"}, // no start_time / end_time
				}
				return makeJSON(items)
			}(),
			listField:   "meeting_info_list",
			startField:  "start_time",
			endField:    "end_time",
			filterStart: t1,
			filterEnd:   t6,
			wantCount:   1,
			wantIDs:     []string{"no_time"},
		},
		{
			name: "filterStart boundary: start_time == filterStart is retained",
			data: func() []byte {
				items := []map[string]interface{}{
					{"meeting_id": "eq", "start_time": float64(t3), "end_time": float64(t4)},
				}
				return makeJSON(items)
			}(),
			listField:   "meeting_info_list",
			startField:  "start_time",
			endField:    "end_time",
			filterStart: t3,
			filterEnd:   0,
			wantCount:   1,
			wantIDs:     []string{"eq"},
		},
		{
			name: "filterEnd boundary: end_time == filterEnd is retained",
			data: func() []byte {
				items := []map[string]interface{}{
					{"meeting_id": "eq_end", "start_time": float64(t3), "end_time": float64(t4)},
				}
				return makeJSON(items)
			}(),
			listField:   "meeting_info_list",
			startField:  "start_time",
			endField:    "end_time",
			filterStart: 0,
			filterEnd:   t4,
			wantCount:   1,
			wantIDs:     []string{"eq_end"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FilterMeetingsByTimeRange(tt.data, tt.listField, tt.startField, tt.endField, tt.filterStart, tt.filterEnd)

			// invalid JSON: return original data
			if tt.wantCount == -2 {
				if string(result) != string(tt.data) {
					t.Errorf("invalid JSON should return original data, got: %s", string(result))
				}
				return
			}

			// listField not found: return original data
			if tt.wantCount == -1 {
				if string(result) != string(tt.data) {
					t.Errorf("listField not found should return original data, got: %s", string(result))
				}
				return
			}

			items := parseFilteredList(t, result, tt.listField)
			if len(items) != tt.wantCount {
				t.Errorf("expected %d items after filter, got %d, result: %s", tt.wantCount, len(items), string(result))
				return
			}

			// verify retained meeting_ids
			for i, wantID := range tt.wantIDs {
				if i >= len(items) {
					t.Errorf("items[%d] not found, expected meeting_id=%s", i, wantID)
					continue
				}
				gotID, _ := items[i]["meeting_id"].(string)
				if gotID != wantID {
					t.Errorf("items[%d].meeting_id expected %s, got %s", i, wantID, gotID)
				}
			}

			// verify meeting_number is synced (if field exists)
			if n := meetingNumber(t, result); n != -1 && n != tt.wantCount {
				t.Errorf("meeting_number expected %d, got %d", tt.wantCount, n)
			}

			t.Logf("filterStart=%d filterEnd=%d -> 保留 %d 条", tt.filterStart, tt.filterEnd, len(items))
		})
	}
}

// ---- TestExtractTimestamp ----

func TestExtractTimestamp(t *testing.T) {
	tests := []struct {
		name  string
		input interface{}
		want  int64
	}{
		{
			name:  "float64 second-level timestamp",
			input: float64(1741766400),
			want:  1741766400,
		},
		{
			name:  "float64 millisecond timestamp (auto-normalized)",
			input: float64(1741766400000),
			want:  1741766400,
		},
		{
			name:  "string second-level timestamp",
			input: "1741766400",
			want:  1741766400,
		},
		{
			name:  "string millisecond timestamp (auto-normalized)",
			input: "1741766400000",
			want:  1741766400,
		},
		{
			name:  "float64 zero value",
			input: float64(0),
			want:  0,
		},
		{
			name:  "string zero value",
			input: "0",
			want:  0,
		},
		{
			name:  "non-numeric string returns 0",
			input: "not-a-number",
			want:  0,
		},
		{
			name:  "nil returns 0",
			input: nil,
			want:  0,
		},
		{
			name:  "bool type returns 0",
			input: true,
			want:  0,
		},
		{
			name:  "boundary: exactly 1e11 (treated as second-level)",
			input: float64(1e11),
			want:  int64(1e11),
		},
		{
			name:  "boundary: 1e11+1 exceeds threshold (treated as millisecond-level)",
			input: float64(int64(1e11) + 1),
			want:  (int64(1e11) + 1) / 1000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractTimestamp(tt.input)
			if got != tt.want {
				t.Errorf("extractTimestamp(%v) = %d, want %d", tt.input, got, tt.want) // nolint
			}
		})
	}
}

// ---- TestKeepFields ----

// mustMarshal is a helper to build JSON bytes from Go values in tests.
func mustMarshal(t *testing.T, v interface{}) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	return b
}

// unmarshalToMap parses bytes into map[string]interface{} for assertions.
func unmarshalToMap(t *testing.T, data []byte) map[string]interface{} {
	t.Helper()
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal result failed: %v, raw: %s", err, string(data))
	}
	return m
}

func TestKeepFields(t *testing.T) {
	// Build a representative nested JSON as the base sample for multiple cases.
	baseSample := map[string]interface{}{
		"meeting_id": "123",
		"subject":    "team sync",
		"host_info": map[string]interface{}{
			"user_id":  "u1",
			"nickname": "alice",
			"extra":    "secret",
		},
		"meeting_info_list": []interface{}{
			map[string]interface{}{
				"meeting_id": "m1",
				"subject":    "daily",
				"start_time": float64(1741766400),
				"end_time":   float64(1741770000),
				"nested": map[string]interface{}{
					"start_time": float64(1741766400),
					"secret":     "x",
				},
			},
			map[string]interface{}{
				"meeting_id": "m2",
				"subject":    "weekly",
				"start_time": float64(1741780000),
				"end_time":   float64(1741790000),
			},
		},
		"meeting_number": float64(2),
	}

	tests := []struct {
		name     string
		data     []byte
		maxDepth int
		fields   []string
		// validate runs assertions against the returned JSON; for edge cases
		// where original data should be returned, check string equality against
		// expectRaw instead.
		expectRaw []byte
		validate  func(t *testing.T, result []byte)
	}{
		{
			name:     "name mode: existing top-level field kept with its subtree",
			data:     mustMarshal(t, baseSample),
			maxDepth: 10,
			fields:   []string{"host_info"},
			validate: func(t *testing.T, result []byte) {
				m := unmarshalToMap(t, result)
				if _, ok := m["host_info"]; !ok {
					t.Errorf("host_info should be kept, got: %s", string(result))
				}
				// other top-level fields removed
				for _, k := range []string{"meeting_id", "subject", "meeting_info_list", "meeting_number"} {
					if _, ok := m[k]; ok {
						t.Errorf("%s should be removed, got: %s", k, string(result))
					}
				}
				// subtree of host_info kept as-is
				host, _ := m["host_info"].(map[string]interface{})
				if host["user_id"] != "u1" || host["nickname"] != "alice" || host["extra"] != "secret" {
					t.Errorf("host_info subtree should be kept as-is, got: %v", host)
				}
			},
		},
		{
			name:     "name mode: recursive match keeps all same-name fields in tree",
			data:     mustMarshal(t, baseSample),
			maxDepth: 10,
			fields:   []string{"start_time"},
			validate: func(t *testing.T, result []byte) {
				m := unmarshalToMap(t, result)
				// top-level start_time does not exist in baseSample, so top-level should be empty/trimmed
				if _, ok := m["start_time"]; ok {
					t.Errorf("top-level start_time should not appear, got: %s", string(result))
				}
				list, ok := m["meeting_info_list"].([]interface{})
				if !ok {
					t.Fatalf("meeting_info_list should be preserved as parent of deeper matches, got: %s", string(result))
				}
				if len(list) != 2 {
					t.Fatalf("meeting_info_list length expected 2, got %d", len(list))
				}
				for i, it := range list {
					item, _ := it.(map[string]interface{})
					if _, ok := item["start_time"]; !ok {
						t.Errorf("items[%d].start_time should be kept, got: %v", i, item)
					}
					// siblings without match must be removed (when item has no deeper match)
					if _, ok := item["subject"]; ok {
						t.Errorf("items[%d].subject should be removed, got: %v", i, item)
					}
				}
				// nested start_time under items[0].nested should also be kept
				first, _ := list[0].(map[string]interface{})
				nested, _ := first["nested"].(map[string]interface{})
				if nested == nil {
					t.Errorf("items[0].nested should be preserved because it contains start_time, got: %v", first)
				} else if _, ok := nested["start_time"]; !ok {
					t.Errorf("items[0].nested.start_time should be kept, got: %v", nested)
				} else if _, ok := nested["secret"]; ok {
					t.Errorf("items[0].nested.secret should be removed, got: %v", nested)
				}
			},
		},
		{
			name:     "name mode: non-existent field yields empty object",
			data:     mustMarshal(t, baseSample),
			maxDepth: 10,
			fields:   []string{"not_exist_key"},
			validate: func(t *testing.T, result []byte) {
				m := unmarshalToMap(t, result)
				if len(m) != 0 {
					t.Errorf("result should be an empty object when no field matched, got: %s", string(result))
				}
			},
		},
		{
			name:     "name mode: maxDepth=1 keeps only top-level matches",
			data:     mustMarshal(t, baseSample),
			maxDepth: 1,
			fields:   []string{"start_time"},
			validate: func(t *testing.T, result []byte) {
				m := unmarshalToMap(t, result)
				// start_time only exists at deeper levels, maxDepth=1 should strip everything.
				if len(m) != 0 {
					t.Errorf("maxDepth=1 should not find deep start_time, expected empty object, got: %s", string(result))
				}
			},
		},
		{
			name:     "path mode: existing nested path is kept and siblings removed",
			data:     mustMarshal(t, baseSample),
			maxDepth: 10,
			fields:   []string{"host_info.nickname"},
			validate: func(t *testing.T, result []byte) {
				m := unmarshalToMap(t, result)
				host, ok := m["host_info"].(map[string]interface{})
				if !ok {
					t.Fatalf("host_info should be kept as parent, got: %s", string(result))
				}
				if host["nickname"] != "alice" {
					t.Errorf("host_info.nickname should be alice, got: %v", host["nickname"])
				}
				if _, ok := host["user_id"]; ok {
					t.Errorf("host_info.user_id should be removed, got: %v", host)
				}
				if _, ok := host["extra"]; ok {
					t.Errorf("host_info.extra should be removed, got: %v", host)
				}
			},
		},
		{
			name:     "path mode: array auto-expanded, keep subject inside each item",
			data:     mustMarshal(t, baseSample),
			maxDepth: 10,
			fields:   []string{"meeting_info_list.subject"},
			validate: func(t *testing.T, result []byte) {
				m := unmarshalToMap(t, result)
				list, ok := m["meeting_info_list"].([]interface{})
				if !ok {
					t.Fatalf("meeting_info_list should be kept as parent, got: %s", string(result))
				}
				if len(list) != 2 {
					t.Fatalf("list length expected 2, got %d", len(list))
				}
				wantSubjects := []string{"daily", "weekly"}
				for i, it := range list {
					item, _ := it.(map[string]interface{})
					if item["subject"] != wantSubjects[i] {
						t.Errorf("items[%d].subject expected %s, got %v", i, wantSubjects[i], item["subject"])
					}
					for _, sibling := range []string{"meeting_id", "start_time", "end_time"} {
						if _, ok := item[sibling]; ok {
							t.Errorf("items[%d].%s should be removed, got: %v", i, sibling, item)
						}
					}
				}
			},
		},
		{
			name:     "path mode: root-level path not found yields empty object",
			data:     mustMarshal(t, baseSample),
			maxDepth: 10,
			fields:   []string{"not_exist.a.b"},
			validate: func(t *testing.T, result []byte) {
				m := unmarshalToMap(t, result)
				if len(m) != 0 {
					t.Errorf("root-level path not found should yield empty object, got: %s", string(result))
				}
			},
		},
		{
			name:     "path mode: intermediate exists but tail missing keeps empty parent",
			data:     mustMarshal(t, baseSample),
			maxDepth: 10,
			fields:   []string{"host_info.not_exist"},
			validate: func(t *testing.T, result []byte) {
				m := unmarshalToMap(t, result)
				host, ok := m["host_info"].(map[string]interface{})
				if !ok {
					t.Fatalf("host_info should be preserved as parent prefix, got: %s", string(result))
				}
				if len(host) != 0 {
					t.Errorf("host_info should become an empty object when tail not found, got: %v", host)
				}
				// top-level other fields removed
				if _, ok := m["meeting_id"]; ok {
					t.Errorf("meeting_id should be removed, got: %s", string(result))
				}
			},
		},
		{
			name:     "mixed mode: union of name-based and path-based whitelists",
			data:     mustMarshal(t, baseSample),
			maxDepth: 10,
			fields:   []string{"meeting_id", "host_info.nickname"},
			validate: func(t *testing.T, result []byte) {
				m := unmarshalToMap(t, result)
				// name match: top-level meeting_id kept, and also meeting_id under each list item kept
				if m["meeting_id"] != "123" {
					t.Errorf("top-level meeting_id expected 123, got %v", m["meeting_id"])
				}
				// path match: host_info.nickname
				host, ok := m["host_info"].(map[string]interface{})
				if !ok {
					t.Fatalf("host_info should be kept as parent, got: %s", string(result))
				}
				if host["nickname"] != "alice" {
					t.Errorf("host_info.nickname expected alice, got %v", host["nickname"])
				}
				// meeting_info_list should be preserved because items contain meeting_id (name whitelist)
				list, ok := m["meeting_info_list"].([]interface{})
				if !ok {
					t.Fatalf("meeting_info_list should be preserved due to name match inside, got: %s", string(result))
				}
				if len(list) != 2 {
					t.Fatalf("list length expected 2, got %d", len(list))
				}
				for i, it := range list {
					item, _ := it.(map[string]interface{})
					if _, ok := item["meeting_id"]; !ok {
						t.Errorf("items[%d].meeting_id should be kept, got: %v", i, item)
					}
					if _, ok := item["subject"]; ok {
						t.Errorf("items[%d].subject should be removed, got: %v", i, item)
					}
				}
			},
		},
		{
			name:     "partial: some fields exist and some do not",
			data:     mustMarshal(t, baseSample),
			maxDepth: 10,
			fields:   []string{"subject", "ghost_field"},
			validate: func(t *testing.T, result []byte) {
				m := unmarshalToMap(t, result)
				if m["subject"] != "team sync" {
					t.Errorf("subject should be kept as 'team sync', got %v", m["subject"])
				}
				if _, ok := m["ghost_field"]; ok {
					t.Errorf("ghost_field should not appear because it does not exist, got: %s", string(result))
				}
				// meeting_info_list keeps items because they also contain 'subject'
				list, ok := m["meeting_info_list"].([]interface{})
				if !ok {
					t.Fatalf("meeting_info_list should be preserved because items have subject, got: %s", string(result))
				}
				for i, it := range list {
					item, _ := it.(map[string]interface{})
					if _, ok := item["subject"]; !ok {
						t.Errorf("items[%d].subject should be kept, got: %v", i, item)
					}
				}
			},
		},
		{
			name:      "edge: empty fields returns original data",
			data:      mustMarshal(t, baseSample),
			maxDepth:  10,
			fields:    []string{},
			expectRaw: mustMarshal(t, baseSample),
		},
		{
			name:      "edge: maxDepth=0 returns original data",
			data:      mustMarshal(t, baseSample),
			maxDepth:  0,
			fields:    []string{"meeting_id"},
			expectRaw: mustMarshal(t, baseSample),
		},
		{
			name:      "edge: invalid JSON returns original data",
			data:      []byte(`{invalid json}`),
			maxDepth:  10,
			fields:    []string{"meeting_id"},
			expectRaw: []byte(`{invalid json}`),
		},
		{
			name:     "edge: empty string entries in fields are ignored",
			data:     mustMarshal(t, baseSample),
			maxDepth: 10,
			fields:   []string{"", "meeting_id", ""},
			validate: func(t *testing.T, result []byte) {
				m := unmarshalToMap(t, result)
				if m["meeting_id"] != "123" {
					t.Errorf("meeting_id should be kept, got: %s", string(result))
				}
				if _, ok := m["subject"]; ok {
					t.Errorf("subject should be removed, got: %s", string(result))
				}
			},
		},
		{
			name: "name mode: keeps array subtree intact when name matched",
			data: mustMarshal(t, map[string]interface{}{
				"tags":  []interface{}{"a", "b", "c"},
				"other": "drop me",
			}),
			maxDepth: 10,
			fields:   []string{"tags"},
			validate: func(t *testing.T, result []byte) {
				m := unmarshalToMap(t, result)
				tags, ok := m["tags"].([]interface{})
				if !ok || len(tags) != 3 {
					t.Errorf("tags array should be kept intact, got: %s", string(result))
				}
				if _, ok := m["other"]; ok {
					t.Errorf("other should be removed, got: %s", string(result))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := KeepFields(tt.data, tt.maxDepth, tt.fields)

			if tt.expectRaw != nil {
				if string(result) != string(tt.expectRaw) {
					t.Errorf("expected raw result:\n  want: %s\n  got:  %s", string(tt.expectRaw), string(result))
				}
				return
			}

			if tt.validate != nil {
				tt.validate(t, result)
			}
		})
	}
}

// ---- TestDeleteFields ----

func TestDeleteFields(t *testing.T) {
	// Build a representative nested JSON as the base sample for multiple cases.
	baseSample := map[string]interface{}{
		"meeting_id": "123",
		"subject":    "team sync",
		"host_info": map[string]interface{}{
			"user_id":  "u1",
			"nickname": "alice",
			"extra":    "secret",
		},
		"meeting_info_list": []interface{}{
			map[string]interface{}{
				"meeting_id": "m1",
				"subject":    "daily",
				"start_time": float64(1741766400),
				"end_time":   float64(1741770000),
				"nested": map[string]interface{}{
					"start_time": float64(1741766400),
					"secret":     "x",
				},
			},
			map[string]interface{}{
				"meeting_id": "m2",
				"subject":    "weekly",
				"start_time": float64(1741780000),
				"end_time":   float64(1741790000),
			},
		},
		"meeting_number": float64(2),
	}

	tests := []struct {
		name      string
		data      []byte
		maxDepth  int
		fields    []string
		expectRaw []byte
		validate  func(t *testing.T, result []byte)
	}{
		{
			name:     "name mode: top-level field removed with its subtree",
			data:     mustMarshal(t, baseSample),
			maxDepth: 10,
			fields:   []string{"host_info"},
			validate: func(t *testing.T, result []byte) {
				m := unmarshalToMap(t, result)
				if _, ok := m["host_info"]; ok {
					t.Errorf("host_info should be removed, got: %s", string(result))
				}
				// other top-level fields preserved
				if m["meeting_id"] != "123" {
					t.Errorf("meeting_id should be preserved, got: %v", m["meeting_id"])
				}
				if m["subject"] != "team sync" {
					t.Errorf("subject should be preserved, got: %v", m["subject"])
				}
				if _, ok := m["meeting_info_list"]; !ok {
					t.Errorf("meeting_info_list should be preserved, got: %s", string(result))
				}
				if _, ok := m["meeting_number"]; !ok {
					t.Errorf("meeting_number should be preserved, got: %s", string(result))
				}
			},
		},
		{
			name:     "name mode: recursive removal of all same-name fields in tree",
			data:     mustMarshal(t, baseSample),
			maxDepth: 10,
			fields:   []string{"start_time"},
			validate: func(t *testing.T, result []byte) {
				m := unmarshalToMap(t, result)
				list, ok := m["meeting_info_list"].([]interface{})
				if !ok {
					t.Fatalf("meeting_info_list should be preserved, got: %s", string(result))
				}
				if len(list) != 2 {
					t.Fatalf("list length expected 2, got %d", len(list))
				}
				for i, it := range list {
					item, _ := it.(map[string]interface{})
					if _, ok := item["start_time"]; ok {
						t.Errorf("items[%d].start_time should be removed, got: %v", i, item)
					}
					// siblings preserved
					if _, ok := item["end_time"]; !ok {
						t.Errorf("items[%d].end_time should be preserved, got: %v", i, item)
					}
					if _, ok := item["subject"]; !ok {
						t.Errorf("items[%d].subject should be preserved, got: %v", i, item)
					}
				}
				// nested.start_time also removed; nested.secret preserved
				first, _ := list[0].(map[string]interface{})
				nested, _ := first["nested"].(map[string]interface{})
				if nested == nil {
					t.Fatalf("items[0].nested should be preserved, got: %v", first)
				}
				if _, ok := nested["start_time"]; ok {
					t.Errorf("items[0].nested.start_time should be removed, got: %v", nested)
				}
				if nested["secret"] != "x" {
					t.Errorf("items[0].nested.secret should be preserved, got: %v", nested)
				}
			},
		},
		{
			name:     "name mode: non-existent field is a no-op",
			data:     mustMarshal(t, baseSample),
			maxDepth: 10,
			fields:   []string{"not_exist_key"},
			validate: func(t *testing.T, result []byte) {
				m := unmarshalToMap(t, result)
				// All original top-level keys remain present.
				for _, k := range []string{"meeting_id", "subject", "host_info", "meeting_info_list", "meeting_number"} {
					if _, ok := m[k]; !ok {
						t.Errorf("%s should be preserved when delete target does not exist, got: %s", k, string(result))
					}
				}
			},
		},
		{
			name:     "name mode: maxDepth=1 only removes top-level matches",
			data:     mustMarshal(t, baseSample),
			maxDepth: 1,
			fields:   []string{"start_time"},
			validate: func(t *testing.T, result []byte) {
				m := unmarshalToMap(t, result)
				// start_time only exists at deeper levels, so depth=1 should not affect them.
				list, ok := m["meeting_info_list"].([]interface{})
				if !ok {
					t.Fatalf("meeting_info_list should be preserved, got: %s", string(result))
				}
				if len(list) != 2 {
					t.Fatalf("list length expected 2, got %d", len(list))
				}
				for i, it := range list {
					item, _ := it.(map[string]interface{})
					if _, ok := item["start_time"]; !ok {
						t.Errorf("items[%d].start_time should remain due to maxDepth=1, got: %v", i, item)
					}
				}
			},
		},
		{
			name:     "path mode: nested path removed and siblings preserved",
			data:     mustMarshal(t, baseSample),
			maxDepth: 10,
			fields:   []string{"host_info.nickname"},
			validate: func(t *testing.T, result []byte) {
				m := unmarshalToMap(t, result)
				host, ok := m["host_info"].(map[string]interface{})
				if !ok {
					t.Fatalf("host_info should be preserved as parent, got: %s", string(result))
				}
				if _, ok := host["nickname"]; ok {
					t.Errorf("host_info.nickname should be removed, got: %v", host)
				}
				if host["user_id"] != "u1" {
					t.Errorf("host_info.user_id should be preserved, got: %v", host["user_id"])
				}
				if host["extra"] != "secret" {
					t.Errorf("host_info.extra should be preserved, got: %v", host["extra"])
				}
			},
		},
		{
			name:     "path mode: array auto-expanded, remove subject inside each item",
			data:     mustMarshal(t, baseSample),
			maxDepth: 10,
			fields:   []string{"meeting_info_list.subject"},
			validate: func(t *testing.T, result []byte) {
				m := unmarshalToMap(t, result)
				list, ok := m["meeting_info_list"].([]interface{})
				if !ok {
					t.Fatalf("meeting_info_list should be preserved, got: %s", string(result))
				}
				if len(list) != 2 {
					t.Fatalf("list length expected 2, got %d", len(list))
				}
				for i, it := range list {
					item, _ := it.(map[string]interface{})
					if _, ok := item["subject"]; ok {
						t.Errorf("items[%d].subject should be removed, got: %v", i, item)
					}
					// siblings preserved
					for _, sibling := range []string{"meeting_id", "start_time", "end_time"} {
						if _, ok := item[sibling]; !ok {
							t.Errorf("items[%d].%s should be preserved, got: %v", i, sibling, item)
						}
					}
				}
				// top-level siblings preserved
				if m["meeting_id"] != "123" {
					t.Errorf("top-level meeting_id should be preserved, got: %v", m["meeting_id"])
				}
				// path-mode targeting top-level 'subject' was NOT requested, so it remains.
				if m["subject"] != "team sync" {
					t.Errorf("top-level subject should be preserved, got: %v", m["subject"])
				}
			},
		},
		{
			name:     "path mode: non-existent path is a no-op",
			data:     mustMarshal(t, baseSample),
			maxDepth: 10,
			fields:   []string{"not_exist.a.b"},
			validate: func(t *testing.T, result []byte) {
				m := unmarshalToMap(t, result)
				for _, k := range []string{"meeting_id", "subject", "host_info", "meeting_info_list", "meeting_number"} {
					if _, ok := m[k]; !ok {
						t.Errorf("%s should be preserved, got: %s", k, string(result))
					}
				}
			},
		},
		{
			name:     "path mode: intermediate exists but tail missing keeps parent intact",
			data:     mustMarshal(t, baseSample),
			maxDepth: 10,
			fields:   []string{"host_info.not_exist"},
			validate: func(t *testing.T, result []byte) {
				m := unmarshalToMap(t, result)
				host, ok := m["host_info"].(map[string]interface{})
				if !ok {
					t.Fatalf("host_info should be preserved, got: %s", string(result))
				}
				// parent fully preserved because tail does not exist
				if host["user_id"] != "u1" || host["nickname"] != "alice" || host["extra"] != "secret" {
					t.Errorf("host_info subtree should be preserved when tail missing, got: %v", host)
				}
			},
		},
		{
			name:     "mixed mode: union of name-based and path-based blacklists",
			data:     mustMarshal(t, baseSample),
			maxDepth: 10,
			fields:   []string{"meeting_id", "host_info.nickname"},
			validate: func(t *testing.T, result []byte) {
				m := unmarshalToMap(t, result)
				// name match: top-level meeting_id removed
				if _, ok := m["meeting_id"]; ok {
					t.Errorf("top-level meeting_id should be removed, got: %s", string(result))
				}
				// path match: host_info.nickname removed; host_info itself preserved
				host, ok := m["host_info"].(map[string]interface{})
				if !ok {
					t.Fatalf("host_info should be preserved, got: %s", string(result))
				}
				if _, ok := host["nickname"]; ok {
					t.Errorf("host_info.nickname should be removed, got: %v", host)
				}
				if host["user_id"] != "u1" {
					t.Errorf("host_info.user_id should be preserved, got: %v", host)
				}
				// name match also affects deeper meeting_id under each list item
				list, ok := m["meeting_info_list"].([]interface{})
				if !ok {
					t.Fatalf("meeting_info_list should be preserved, got: %s", string(result))
				}
				for i, it := range list {
					item, _ := it.(map[string]interface{})
					if _, ok := item["meeting_id"]; ok {
						t.Errorf("items[%d].meeting_id should be removed, got: %v", i, item)
					}
					if _, ok := item["subject"]; !ok {
						t.Errorf("items[%d].subject should be preserved, got: %v", i, item)
					}
				}
			},
		},
		{
			name:     "partial: some fields exist and some do not",
			data:     mustMarshal(t, baseSample),
			maxDepth: 10,
			fields:   []string{"subject", "ghost_field"},
			validate: func(t *testing.T, result []byte) {
				m := unmarshalToMap(t, result)
				if _, ok := m["subject"]; ok {
					t.Errorf("subject should be removed, got: %s", string(result))
				}
				// other top-level fields preserved
				if m["meeting_id"] != "123" {
					t.Errorf("meeting_id should be preserved, got: %v", m["meeting_id"])
				}
				// items also have 'subject' which should be removed recursively
				list, ok := m["meeting_info_list"].([]interface{})
				if !ok {
					t.Fatalf("meeting_info_list should be preserved, got: %s", string(result))
				}
				for i, it := range list {
					item, _ := it.(map[string]interface{})
					if _, ok := item["subject"]; ok {
						t.Errorf("items[%d].subject should be removed, got: %v", i, item)
					}
				}
			},
		},
		{
			name:      "edge: empty fields returns original data",
			data:      mustMarshal(t, baseSample),
			maxDepth:  10,
			fields:    []string{},
			expectRaw: mustMarshal(t, baseSample),
		},
		{
			name:      "edge: maxDepth=0 returns original data",
			data:      mustMarshal(t, baseSample),
			maxDepth:  0,
			fields:    []string{"meeting_id"},
			expectRaw: mustMarshal(t, baseSample),
		},
		{
			name:      "edge: invalid JSON returns original data",
			data:      []byte(`{invalid json}`),
			maxDepth:  10,
			fields:    []string{"meeting_id"},
			expectRaw: []byte(`{invalid json}`),
		},
		{
			name:      "edge: only empty string entries returns original data",
			data:      mustMarshal(t, baseSample),
			maxDepth:  10,
			fields:    []string{"", ""},
			expectRaw: mustMarshal(t, baseSample),
		},
		{
			name:     "edge: empty string entries in fields are ignored",
			data:     mustMarshal(t, baseSample),
			maxDepth: 10,
			fields:   []string{"", "meeting_id", ""},
			validate: func(t *testing.T, result []byte) {
				m := unmarshalToMap(t, result)
				if _, ok := m["meeting_id"]; ok {
					t.Errorf("meeting_id should be removed, got: %s", string(result))
				}
				if m["subject"] != "team sync" {
					t.Errorf("subject should be preserved, got: %v", m["subject"])
				}
			},
		},
		{
			name: "name mode: removes whole array when name matched",
			data: mustMarshal(t, map[string]interface{}{
				"tags":  []interface{}{"a", "b", "c"},
				"other": "keep me",
			}),
			maxDepth: 10,
			fields:   []string{"tags"},
			validate: func(t *testing.T, result []byte) {
				m := unmarshalToMap(t, result)
				if _, ok := m["tags"]; ok {
					t.Errorf("tags array should be removed, got: %s", string(result))
				}
				if m["other"] != "keep me" {
					t.Errorf("other should be preserved, got: %v", m["other"])
				}
			},
		},
		{
			name: "duality: KeepFields + DeleteFields cover all top-level keys",
			data: mustMarshal(t, map[string]interface{}{
				"a": "1",
				"b": "2",
				"c": "3",
			}),
			maxDepth: 10,
			fields:   []string{"b"},
			validate: func(t *testing.T, result []byte) {
				m := unmarshalToMap(t, result)
				if _, ok := m["b"]; ok {
					t.Errorf("b should be removed, got: %s", string(result))
				}
				if m["a"] != "1" || m["c"] != "3" {
					t.Errorf("a and c should be preserved, got: %s", string(result))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DeleteFields(tt.data, tt.maxDepth, tt.fields)

			if tt.expectRaw != nil {
				if string(result) != string(tt.expectRaw) {
					t.Errorf("expected raw result:\n  want: %s\n  got:  %s", string(tt.expectRaw), string(result))
				}
				return
			}

			if tt.validate != nil {
				tt.validate(t, result)
			}
		})
	}
}
