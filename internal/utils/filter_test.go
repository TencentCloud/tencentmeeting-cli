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
				t.Errorf("extractTimestamp(%v) = %d, want %d", tt.input, got, tt.want) //nolint
			}
		})
	}
}
