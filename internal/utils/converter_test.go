package utils

import (
	"encoding/json"
	"testing"
)

func TestConvertFields(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name: "meeting list nested structure with string timestamp",
			input: `{
  "data": {
    "meeting_info_list": [
      {
        "enable_live": false,
        "end_time": "1775206800",
        "hosts": [
          {
            "userid": "SzfsIRRnsj01m4kjfsWqRgvspyul"
          }
        ],
        "join_url": "https://test-592.cicd.tencentmeeting.com/dm/a3YWqvkC96R5",
        "location": "",
        "media_set_type": 0,
        "meeting_code": "931945029",
        "meeting_id": "6953553464429888300",
        "meeting_type": 0,
        "settings": {
          "allow_multi_device": true,
          "allow_unmute_self": true,
          "audio_watermark": false,
          "mute_all": true,
          "mute_enable_join": false,
          "mute_enable_type_join": 2
        },
        "start_time": "1775203200",
        "subject": "自测会议",
        "type": 0
      }
    ],
    "meeting_number": 1
  },
  "trace_id": "4862dd82e6c231ff93cba4c020b1013a"
}`,
			wantErr: false,
		},
		{
			name: "numeric timestamp",
			input: `{
  "data": {
    "start_time": 1775203200,
    "end_time": 1775206800
  }
}`,
			wantErr: false,
		},
		{
			name: "mixed timestamp types",
			input: `{
  "data": {
    "meeting_info_list": [
      {
        "start_time": "1775203200",
        "end_time": 1775206800
      }
    ]
  }
}`,
			wantErr: false,
		},
		{
			name: "millisecond string timestamp",
			input: `{
  "data": {
    "start_time": "1775203200000",
    "end_time": "1775206800000"
  }
}`,
			wantErr: false,
		},
		{
			name: "millisecond numeric timestamp",
			input: `{
  "data": {
    "start_time": 1775203200000,
    "end_time": 1775206800000
  }
}`,
			wantErr: false,
		},
		{
			name: "mixed second and millisecond timestamps",
			input: `{
  "data": {
    "meeting_info_list": [
      {
        "start_time": "1775203200",
        "end_time": 1775206800000
      }
    ]
  }
}`,
			wantErr: false,
		},
		{
			name:    "invalid JSON returns original data",
			input:   `{invalid json}`,
			wantErr: true,
		},
		{
			name: "depth limit test",
			input: `{
  "level1": {
    "level2": {
      "level3": {
        "start_time": "1775203200"
      }
    }
  }
}`,
			wantErr: false,
		},
	}

	fields := map[string]FieldConverter{
		"start_time":        TimestampConverter,
		"end_time":          TimestampConverter,
		"media_start_time":  TimestampConverter,
		"record_start_time": TimestampConverter,
		"record_end_time":   TimestampConverter,
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertFields([]byte(tt.input), 10, fields)

			// verify output is valid JSON
			var resultMap map[string]interface{}
			err := json.Unmarshal(result, &resultMap)
			if tt.wantErr {
				// invalid JSON should return original data
				if string(result) != tt.input {
					t.Errorf("invalid JSON should return original data, got: %s", string(result))
				}
				return
			}
			if err != nil {
				t.Errorf("result is not valid JSON: %v", err)
				return
			}

			t.Logf("input: %s", tt.input)
			t.Logf("output: %s", string(result))

			// check whether timestamp fields are converted
			checkTimeStampConverted(t, resultMap)
		})
	}
}

func TestNormalizeAndConvertTimestamp(t *testing.T) {
	tests := []struct {
		name     string
		input    int64
		wantYear string // expected year string in result
	}{
		{
			name:     "second-level timestamp",
			input:    1775203200, // 2026-03-12T14:00:00+08:00
			wantYear: "2026",
		},
		{
			name:     "millisecond timestamp",
			input:    1775203200000, // same as above, millisecond
			wantYear: "2026",
		},
		{
			name:     "second and millisecond results are consistent",
			input:    1743600000,
			wantYear: "2025",
		},
		{
			name:     "millisecond and corresponding second results are consistent",
			input:    1743600000000,
			wantYear: "2025",
		},
		{
			name:     "boundary: exactly 1e11 (treated as second-level)",
			input:    int64(1e11),
			wantYear: "5138", // 1e11 seconds corresponds to far future
		},
		{
			name:     "boundary: 1e11+1 exceeds threshold (treated as millisecond-level)",
			input:    int64(1e11) + 1,
			wantYear: "1973", // (1e11+1)/1000 ≈ 1e8 seconds, corresponds to 1973
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeAndConvertTimestamp(tt.input)
			// verify result is ISO8601 format (contains 'T')
			if len(got) == 0 || !containsISO8601Markers(got) {
				t.Errorf("result is not ISO8601 format: %s", got)
			}
			// verify year
			if len(got) < 4 || got[:4] != tt.wantYear {
				t.Errorf("expected year %s, got: %s", tt.wantYear, got)
			}
			t.Logf("input: %d -> output: %s", tt.input, got)
		})
	}

	// extra check: second-level and millisecond-level at the same moment should produce the same result
	t.Run("second and millisecond conversion consistency", func(t *testing.T) {
		secTs := int64(1775203200)
		msTs := int64(1775203200000)
		if normalizeAndConvertTimestamp(secTs) != normalizeAndConvertTimestamp(msTs) {
			t.Errorf("second(%d) and millisecond(%d) conversion results differ: %s vs %s",
				secTs, msTs,
				normalizeAndConvertTimestamp(secTs),
				normalizeAndConvertTimestamp(msTs))
		}
	})
}

// checkTimeStampConverted recursively checks whether timestamp fields have been converted to ISO8601 format
func checkTimeStampConverted(t *testing.T, data interface{}) {
	switch v := data.(type) {
	case map[string]interface{}:
		for key, value := range v {
			if key == "start_time" || key == "end_time" {
				if strVal, ok := value.(string); ok {
					// if it's a string, check whether it's ISO8601 format (contains 'T' or '-')
					if len(strVal) > 0 && strVal[0] >= '0' && strVal[0] <= '9' && !containsISO8601Markers(strVal) {
						t.Errorf("field %s not converted, still a timestamp: %s", key, strVal)
					}
				}
			} else {
				checkTimeStampConverted(t, value)
			}
		}
	case []interface{}:
		for _, item := range v {
			checkTimeStampConverted(t, item)
		}
	}
}

// containsISO8601Markers checks whether a string contains ISO8601 format markers
func containsISO8601Markers(s string) bool {
	// ISO8601 format contains 'T' or date separator '-'
	return len(s) > 10 && (s[4] == '-' || s[10] == 'T')
}

func TestTimestampConverter(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		wantISO  bool   // whether ISO8601 string is expected
		wantYear string // expected year prefix
	}{
		{
			name:     "float64 second-level timestamp",
			input:    float64(1775203200),
			wantISO:  true,
			wantYear: "2026",
		},
		{
			name:     "float64 millisecond timestamp",
			input:    float64(1775203200000),
			wantISO:  true,
			wantYear: "2026",
		},
		{
			name:     "string second-level timestamp",
			input:    "1775203200",
			wantISO:  true,
			wantYear: "2026",
		},
		{
			name:     "string millisecond timestamp",
			input:    "1775203200000",
			wantISO:  true,
			wantYear: "2026",
		},
		{
			name:    "non-numeric string returns as-is",
			input:   "not-a-timestamp",
			wantISO: false,
		},
		{
			name:    "nil returns as-is",
			input:   nil,
			wantISO: false,
		},
		{
			name:    "bool type returns as-is",
			input:   true,
			wantISO: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TimestampConverter(tt.input)
			if tt.wantISO {
				str, ok := got.(string)
				if !ok {
					t.Fatalf("expected string return, got %T: %v", got, got)
				}
				if !containsISO8601Markers(str) {
					t.Errorf("expected ISO8601 format, got: %s", str)
				}
				if tt.wantYear != "" && (len(str) < 4 || str[:4] != tt.wantYear) {
					t.Errorf("expected year %s, got: %s", tt.wantYear, str)
				}
				t.Logf("input: %v -> output: %s", tt.input, str)
			} else {
				if got != tt.input {
					t.Errorf("expected as-is return %v, got: %v", tt.input, got)
				}
			}
		})
	}
}

func TestInstanceIdConverter(t *testing.T) {
	tests := []struct {
		name  string
		input interface{}
		want  interface{}
	}{
		{name: "float64 PC", input: float64(1), want: "PC"},
		{name: "float64 Mac", input: float64(2), want: "Mac"},
		{name: "float64 iOS", input: float64(4), want: "iOS"},
		{name: "float64 HarmonyOS Phone", input: float64(81), want: "HarmonyOS Phone"},
		{name: "float64 unknown type", input: float64(99), want: "Unknown"},
		{name: "string PC", input: "1", want: "PC"},
		{name: "string Mac", input: "2", want: "Mac"},
		{name: "string unknown type", input: "999", want: "Unknown"},
		{name: "non-numeric string returns as-is", input: "abc", want: "abc"},
		{name: "nil returns as-is", input: nil, want: nil},
		{name: "bool returns as-is", input: false, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := InstanceIdConverter(tt.input)
			if got != tt.want {
				t.Errorf("InstanceIdConverter(%v) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestHHMMSSConverter(t *testing.T) {
	tests := []struct {
		name  string
		input interface{}
		want  interface{}
	}{
		{
			name:  "float64 zero milliseconds",
			input: float64(0),
			want:  "00:00",
		},
		{
			name:  "float64 less than one minute (45s)",
			input: float64(45000),
			want:  "00:45",
		},
		{
			name:  "float64 exactly one minute",
			input: float64(60000),
			want:  "01:00",
		},
		{
			name:  "float64 less than one hour (59m59s)",
			input: float64(3599000),
			want:  "59:59",
		},
		{
			name:  "float64 exactly one hour",
			input: float64(3600000),
			want:  "01:00:00",
		},
		{
			name:  "float64 more than one hour (1h1m1s)",
			input: float64(3661000),
			want:  "01:01:01",
		},
		{
			name:  "string zero milliseconds",
			input: "0",
			want:  "00:00",
		},
		{
			name:  "string less than one minute (30s)",
			input: "30000",
			want:  "00:30",
		},
		{
			name:  "string 超过一小时（2小时2分2秒）",
			input: "7322000",
			want:  "02:02:02",
		},
		{
			name:  "非数字字符串原样返回",
			input: "not-a-number",
			want:  "not-a-number",
		},
		{
			name:  "nil 原样返回",
			input: nil,
			want:  nil,
		},
		{
			name:  "bool 类型原样返回",
			input: true,
			want:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HHMMSSConverter(tt.input)
			if got != tt.want {
				t.Errorf("HHMMSSConverter(%v) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestConvertFields_WithConverters(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		fields map[string]FieldConverter
		checks func(t *testing.T, result map[string]interface{})
	}{
		{
			name: "时间戳字段转换",
			input: `{
  "start_time": "1775203200",
  "end_time": 1775206800,
  "subject": "测试会议"
}`,
			fields: map[string]FieldConverter{
				"start_time": TimestampConverter,
				"end_time":   TimestampConverter,
			},
			checks: func(t *testing.T, result map[string]interface{}) {
				for _, key := range []string{"start_time", "end_time"} {
					val, ok := result[key].(string)
					if !ok || !containsISO8601Markers(val) {
						t.Errorf("字段 %s 未转换为 ISO8601，实际: %v", key, result[key])
					}
				}
				// 非转换字段保持不变
				if result["subject"] != "测试会议" {
					t.Errorf("subject 字段被意外修改: %v", result["subject"])
				}
			},
		},
		{
			name: "instanceid 字段转换",
			input: `{
  "instance_id": 2,
  "user": "test"
}`,
			fields: map[string]FieldConverter{
				"instance_id": InstanceIdConverter,
			},
			checks: func(t *testing.T, result map[string]interface{}) {
				if result["instance_id"] != "Mac" {
					t.Errorf("instance_id 转换错误，期望 Mac，实际: %v", result["instance_id"])
				}
				if result["user"] != "test" {
					t.Errorf("user 字段被意外修改: %v", result["user"])
				}
			},
		},
		{
			name: "嵌套结构中的字段转换",
			input: `{
  "participants": [
    {"instance_id": 4, "join_time": "1775203200"},
    {"instance_id": 81, "join_time": 1775206800}
  ]
}`,
			fields: map[string]FieldConverter{
				"instance_id": InstanceIdConverter,
				"join_time":   TimestampConverter,
			},
			checks: func(t *testing.T, result map[string]interface{}) {
				list, ok := result["participants"].([]interface{})
				if !ok || len(list) != 2 {
					t.Fatal("participants 结构异常")
				}
				p0 := list[0].(map[string]interface{})
				if p0["instance_id"] != "iOS" {
					t.Errorf("participants[0].instance_id 期望 iOS，实际: %v", p0["instance_id"])
				}
				if str, ok := p0["join_time"].(string); !ok || !containsISO8601Markers(str) {
					t.Errorf("participants[0].join_time 未转换为 ISO8601，实际: %v", p0["join_time"])
				}
				p1 := list[1].(map[string]interface{})
				if p1["instance_id"] != "HarmonyOS Phone" {
					t.Errorf("participants[1].instance_id 期望 HarmonyOS Phone，实际: %v", p1["instance_id"])
				}
			},
		},
		{
			name:   "fields 为 nil 返回原数据",
			input:  `{"start_time": "1775203200"}`,
			fields: nil,
			checks: func(t *testing.T, result map[string]interface{}) {
				if result["start_time"] != "1775203200" {
					t.Errorf("fields 为 nil 时数据被意外修改: %v", result["start_time"])
				}
			},
		},
		{
			name:   "无效 JSON 返回原数据",
			input:  `{invalid}`,
			fields: map[string]FieldConverter{"start_time": TimestampConverter},
			checks: nil, // 特殊处理
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertFields([]byte(tt.input), 10, tt.fields)

			if tt.checks == nil {
				// 无效 JSON 应原样返回
				if string(result) != tt.input {
					t.Errorf("无效 JSON 应原样返回，实际: %s", string(result))
				}
				return
			}

			var resultMap map[string]interface{}
			if err := json.Unmarshal(result, &resultMap); err != nil {
				t.Fatalf("结果不是有效 JSON: %v, raw: %s", err, string(result))
			}
			tt.checks(t, resultMap)
			t.Logf("输出: %s", string(result))
		})
	}
}

// TestBase64DecodeConverter 测试 Base64 解码转换器
func TestBase64DecodeConverter(t *testing.T) {
	tests := []struct {
		name  string
		input interface{}
		want  interface{}
	}{
		{
			name:  "标准 Base64 解码",
			input: "aGVsbG8gd29ybGQ=", // "hello world"
			want:  "hello world",
		},
		{
			name:  "中文内容解码",
			input: "5L2g5aW9", // "你好"
			want:  "你好",
		},
		{
			name:  "空字符串解码",
			input: "",
			want:  "",
		},
		{
			name:  "URL-safe Base64 解码",
			input: "aGVsbG8-d29ybGQ=", // URL-safe 编码
			want:  "hello>world",
		},
		{
			name:  "非 Base64 字符串原样返回",
			input: "not-base64-!!!",
			want:  "not-base64-!!!",
		},
		{
			name:  "nil 原样返回",
			input: nil,
			want:  nil,
		},
		{
			name:  "float64 类型原样返回",
			input: float64(123),
			want:  float64(123),
		},
		{
			name:  "bool 类型原样返回",
			input: true,
			want:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Base64DecodeConverter(tt.input)
			if got != tt.want {
				t.Errorf("Base64DecodeConverter(%v) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// TestConvertFields_PathMode 测试路径模式的字段转换
func TestConvertFields_PathMode(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		fields map[string]FieldConverter
		checks func(t *testing.T, result map[string]interface{})
	}{
		{
			name: "单层路径精确匹配",
			input: `{
  "meeting": {
    "meeting_type": 1,
    "subject": "测试会议"
  }
}`,
			fields: map[string]FieldConverter{
				"meeting.meeting_type": MeetingTypeConverter,
			},
			checks: func(t *testing.T, result map[string]interface{}) {
				meeting := result["meeting"].(map[string]interface{})
				if meeting["meeting_type"] != "周期性会议" {
					t.Errorf("meeting.meeting_type 期望 '周期性会议'，实际: %v", meeting["meeting_type"])
				}
				if meeting["subject"] != "测试会议" {
					t.Errorf("subject 字段被意外修改: %v", meeting["subject"])
				}
			},
		},
		{
			name: "多层路径精确匹配",
			input: `{
  "meeting": {
    "recurring_rule": {
      "recurring_type": 2,
      "until_count": 10
    }
  }
}`,
			fields: map[string]FieldConverter{
				"meeting.recurring_rule.recurring_type": MeetingRecurringTypeConverter,
			},
			checks: func(t *testing.T, result map[string]interface{}) {
				meeting := result["meeting"].(map[string]interface{})
				rule := meeting["recurring_rule"].(map[string]interface{})
				if rule["recurring_type"] != "每周" {
					t.Errorf("recurring_type 期望 '每周'，实际: %v", rule["recurring_type"])
				}
				// until_count 不应被修改
				if rule["until_count"] != float64(10) {
					t.Errorf("until_count 被意外修改: %v", rule["until_count"])
				}
			},
		},
		{
			name: "路径中包含数组自动展开",
			input: `{
  "data": {
    "meeting_info_list": [
      {"meeting_type": 0, "subject": "会议1"},
      {"meeting_type": 1, "subject": "会议2"}
    ]
  }
}`,
			fields: map[string]FieldConverter{
				"data.meeting_info_list.meeting_type": MeetingTypeConverter,
			},
			checks: func(t *testing.T, result map[string]interface{}) {
				data := result["data"].(map[string]interface{})
				list := data["meeting_info_list"].([]interface{})
				m0 := list[0].(map[string]interface{})
				if m0["meeting_type"] != "普通会议" {
					t.Errorf("meeting_info_list[0].meeting_type 期望 '普通会议'，实际: %v", m0["meeting_type"])
				}
				m1 := list[1].(map[string]interface{})
				if m1["meeting_type"] != "周期性会议" {
					t.Errorf("meeting_info_list[1].meeting_type 期望 '周期性会议'，实际: %v", m1["meeting_type"])
				}
			},
		},
		{
			name: "路径模式和字段名模式混合使用",
			input: `{
  "data": {
    "meeting_info_list": [
      {
        "start_time": "1775203200",
        "end_time": "1775206800",
        "meeting_type": 1,
        "recurring_rule": {
          "recurring_type": 3
        }
      }
    ]
  }
}`,
			fields: map[string]FieldConverter{
				"start_time": TimestampConverter,
				"end_time":   TimestampConverter,
				"data.meeting_info_list.recurring_rule.recurring_type": MeetingRecurringTypeConverter,
			},
			checks: func(t *testing.T, result map[string]interface{}) {
				data := result["data"].(map[string]interface{})
				list := data["meeting_info_list"].([]interface{})
				m0 := list[0].(map[string]interface{})
				// 字段名模式：时间戳应被转换
				if str, ok := m0["start_time"].(string); !ok || !containsISO8601Markers(str) {
					t.Errorf("start_time 未转换为 ISO8601，实际: %v", m0["start_time"])
				}
				if str, ok := m0["end_time"].(string); !ok || !containsISO8601Markers(str) {
					t.Errorf("end_time 未转换为 ISO8601，实际: %v", m0["end_time"])
				}
				// 路径模式：recurring_type 应被转换
				rule := m0["recurring_rule"].(map[string]interface{})
				if rule["recurring_type"] != "每两周" {
					t.Errorf("recurring_type 期望 '每两周'，实际: %v", rule["recurring_type"])
				}
				// meeting_type 未指定转换，应保持原值
				if m0["meeting_type"] != float64(1) {
					t.Errorf("meeting_type 被意外修改: %v", m0["meeting_type"])
				}
			},
		},
		{
			name: "路径不存在时不报错",
			input: `{
  "data": {
    "subject": "测试"
  }
}`,
			fields: map[string]FieldConverter{
				"data.nonexistent.field": MeetingTypeConverter,
			},
			checks: func(t *testing.T, result map[string]interface{}) {
				data := result["data"].(map[string]interface{})
				if data["subject"] != "测试" {
					t.Errorf("subject 被意外修改: %v", data["subject"])
				}
			},
		},
		{
			name: "路径模式不受 maxDepth 限制",
			input: `{
  "l1": {
    "l2": {
      "l3": {
        "l4": {
          "l5": {
            "target": 1
          }
        }
      }
    }
  }
}`,
			fields: map[string]FieldConverter{
				"l1.l2.l3.l4.l5.target": MeetingTypeConverter,
			},
			checks: func(t *testing.T, result map[string]interface{}) {
				// 即使 maxDepth=10，路径模式也能精确到达深层字段
				l1 := result["l1"].(map[string]interface{})
				l2 := l1["l2"].(map[string]interface{})
				l3 := l2["l3"].(map[string]interface{})
				l4 := l3["l4"].(map[string]interface{})
				l5 := l4["l5"].(map[string]interface{})
				if l5["target"] != "周期性会议" {
					t.Errorf("深层路径 target 期望 '周期性会议'，实际: %v", l5["target"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertFields([]byte(tt.input), 10, tt.fields)

			var resultMap map[string]interface{}
			if err := json.Unmarshal(result, &resultMap); err != nil {
				t.Fatalf("结果不是有效 JSON: %v, raw: %s", err, string(result))
			}
			tt.checks(t, resultMap)
			t.Logf("输出: %s", string(result))
		})
	}
}

func TestRecordTypeConverter(t *testing.T) {
	tests := []struct {
		name  string
		input interface{}
		want  interface{}
	}{
		{name: "float64 cloud record", input: float64(0), want: "云录制"},
		{name: "float64 upload record", input: float64(2), want: "上传录制"},
		{name: "float64 transcript", input: float64(3), want: "文字转写"},
		{name: "float64 video record", input: float64(4), want: "视频录制"},
		{name: "float64 audio record", input: float64(5), want: "音频录制"},
		{name: "float64 unknown", input: float64(99), want: "Unknown"},
		{name: "string cloud record", input: "0", want: "云录制"},
		{name: "string video record", input: "4", want: "视频录制"},
		{name: "string unknown", input: "999", want: "Unknown"},
		{name: "non-numeric string returns as-is", input: "abc", want: "abc"},
		{name: "nil returns as-is", input: nil, want: nil},
		{name: "bool returns as-is", input: true, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RecordTypeConverter(tt.input)
			if got != tt.want {
				t.Errorf("RecordTypeConverter(%v) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestRecordStateConverter(t *testing.T) {
	tests := []struct {
		name  string
		input interface{}
		want  interface{}
	}{
		{name: "float64 recording", input: float64(1), want: "录制中"},
		{name: "float64 transcoding", input: float64(2), want: "转码中"},
		{name: "float64 done", input: float64(3), want: "转码完成"},
		{name: "float64 unknown", input: float64(99), want: "Unknown"},
		{name: "string recording", input: "1", want: "录制中"},
		{name: "string done", input: "3", want: "转码完成"},
		{name: "string unknown", input: "999", want: "Unknown"},
		{name: "non-numeric string returns as-is", input: "abc", want: "abc"},
		{name: "nil returns as-is", input: nil, want: nil},
		{name: "bool returns as-is", input: false, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RecordStateConverter(tt.input)
			if got != tt.want {
				t.Errorf("RecordStateConverter(%v) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestRecordAudioDetectConverter(t *testing.T) {
	tests := []struct {
		name  string
		input interface{}
		want  interface{}
	}{
		{name: "float64 incomplete", input: float64(0), want: "未完成"},
		{name: "float64 complete", input: float64(1), want: "已完成"},
		{name: "float64 unknown", input: float64(99), want: "Unknown"},
		{name: "string incomplete", input: "0", want: "未完成"},
		{name: "string complete", input: "1", want: "已完成"},
		{name: "string unknown", input: "999", want: "Unknown"},
		{name: "non-numeric string returns as-is", input: "abc", want: "abc"},
		{name: "nil returns as-is", input: nil, want: nil},
		{name: "bool returns as-is", input: true, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RecordAudioDetectConverter(tt.input)
			if got != tt.want {
				t.Errorf("RecordAudioDetectConverter(%v) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestMeetingTypeConverter(t *testing.T) {
	tests := []struct {
		name  string
		input interface{}
		want  interface{}
	}{
		{name: "float64 normal", input: float64(0), want: "普通会议"},
		{name: "float64 recurring", input: float64(1), want: "周期性会议"},
		{name: "float64 wechat", input: float64(2), want: "微信专属会议"},
		{name: "float64 rooms", input: float64(4), want: "Rooms 投屏会议"},
		{name: "float64 personal", input: float64(5), want: "个人会议号会议"},
		{name: "float64 webinar", input: float64(6), want: "网络研讨会"},
		{name: "float64 unknown", input: float64(99), want: "Unknown"},
		{name: "string normal", input: "0", want: "普通会议"},
		{name: "string recurring", input: "1", want: "周期性会议"},
		{name: "string unknown", input: "999", want: "Unknown"},
		{name: "non-numeric string returns as-is", input: "abc", want: "abc"},
		{name: "nil returns as-is", input: nil, want: nil},
		{name: "bool returns as-is", input: false, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MeetingTypeConverter(tt.input)
			if got != tt.want {
				t.Errorf("MeetingTypeConverter(%v) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestMeetingRecurringTypeConverter(t *testing.T) {
	tests := []struct {
		name  string
		input interface{}
		want  interface{}
	}{
		{name: "float64 daily", input: float64(0), want: "每天"},
		{name: "float64 weekdays", input: float64(1), want: "每周一至周五"},
		{name: "float64 weekly", input: float64(2), want: "每周"},
		{name: "float64 biweekly", input: float64(3), want: "每两周"},
		{name: "float64 monthly", input: float64(4), want: "每月"},
		{name: "float64 unknown", input: float64(99), want: "Unknown"},
		{name: "string daily", input: "0", want: "每天"},
		{name: "string weekly", input: "2", want: "每周"},
		{name: "string unknown", input: "999", want: "Unknown"},
		{name: "non-numeric string returns as-is", input: "abc", want: "abc"},
		{name: "nil returns as-is", input: nil, want: nil},
		{name: "bool returns as-is", input: true, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MeetingRecurringTypeConverter(tt.input)
			if got != tt.want {
				t.Errorf("MeetingRecurringTypeConverter(%v) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestMeetingRecurringUntilTypeConverter(t *testing.T) {
	tests := []struct {
		name  string
		input interface{}
		want  interface{}
	}{
		{name: "float64 by date", input: float64(0), want: "按日期结束重复"},
		{name: "float64 by count", input: float64(1), want: "按次数结束重复"},
		{name: "float64 unknown", input: float64(99), want: "Unknown"},
		{name: "string by date", input: "0", want: "按日期结束重复"},
		{name: "string by count", input: "1", want: "按次数结束重复"},
		{name: "string unknown", input: "999", want: "Unknown"},
		{name: "non-numeric string returns as-is", input: "abc", want: "abc"},
		{name: "nil returns as-is", input: nil, want: nil},
		{name: "bool returns as-is", input: false, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MeetingRecurringUntilTypeConverter(tt.input)
			if got != tt.want {
				t.Errorf("MeetingRecurringUntilTypeConverter(%v) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestMeetingUserRoleConverter(t *testing.T) {
	tests := []struct {
		name  string
		input interface{}
		want  interface{}
	}{
		{name: "float64 member", input: float64(0), want: "普通成员角色"},
		{name: "float64 creator", input: float64(1), want: "创建者角色"},
		{name: "float64 host", input: float64(2), want: "主持人"},
		{name: "float64 creator+host", input: float64(3), want: "创建者+主持人"},
		{name: "float64 guest", input: float64(4), want: "游客"},
		{name: "float64 guest+host", input: float64(5), want: "游客+主持人"},
		{name: "float64 co-host", input: float64(6), want: "联席主持人"},
		{name: "float64 creator+co-host", input: float64(7), want: "创建者+联席主持人"},
		{name: "float64 unknown", input: float64(99), want: "Unknown"},
		{name: "string host", input: "2", want: "主持人"},
		{name: "string co-host", input: "6", want: "联席主持人"},
		{name: "string unknown", input: "999", want: "Unknown"},
		{name: "non-numeric string returns as-is", input: "abc", want: "abc"},
		{name: "nil returns as-is", input: nil, want: nil},
		{name: "bool returns as-is", input: true, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MeetingUserRoleConverter(tt.input)
			if got != tt.want {
				t.Errorf("MeetingUserRoleConverter(%v) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestMeetingStatusConverter(t *testing.T) {
	tests := []struct {
		name  string
		input interface{}
		want  interface{}
	}{
		{name: "invalid status", input: "MEETING_STATE_INVALID", want: "非法状态"},
		{name: "init status", input: "MEETING_STATE_INIT", want: "待开始"},
		{name: "cancelled status", input: "MEETING_STATE_CANCELLED", want: "已取消"},
		{name: "started status", input: "MEETING_STATE_STARTED", want: "会议中"},
		{name: "ended status", input: "MEETING_STATE_ENDED", want: "已删除"},
		{name: "null status", input: "MEETING_STATE_NULL", want: "无状态"},
		{name: "recycled status", input: "MEETING_STATE_RECYCLED", want: "已回收"},
		{name: "unknown string", input: "MEETING_STATE_UNKNOWN", want: "Unknown"},
		{name: "empty string", input: "", want: "Unknown"},
		{name: "float64 returns as-is", input: float64(1), want: float64(1)},
		{name: "nil returns as-is", input: nil, want: nil},
		{name: "bool returns as-is", input: false, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MeetingStatusConverter(tt.input)
			if got != tt.want {
				t.Errorf("MeetingStatusConverter(%v) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
