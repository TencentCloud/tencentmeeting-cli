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
			name: "会议列表嵌套结构_字符串时间戳",
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
			name: "数字类型时间戳",
			input: `{
  "data": {
    "start_time": 1775203200,
    "end_time": 1775206800
  }
}`,
			wantErr: false,
		},
		{
			name: "混合类型时间戳",
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
			name: "毫秒级字符串时间戳",
			input: `{
  "data": {
    "start_time": "1775203200000",
    "end_time": "1775206800000"
  }
}`,
			wantErr: false,
		},
		{
			name: "毫秒级数字时间戳",
			input: `{
  "data": {
    "start_time": 1775203200000,
    "end_time": 1775206800000
  }
}`,
			wantErr: false,
		},
		{
			name: "秒级与毫秒级时间戳混合",
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
			name:    "无效JSON返回原数据",
			input:   `{invalid json}`,
			wantErr: true,
		},
		{
			name: "深度限制测试",
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

			// 验证输出是否为有效 JSON
			var resultMap map[string]interface{}
			err := json.Unmarshal(result, &resultMap)
			if tt.wantErr {
				// 无效 JSON 应该返回原数据
				if string(result) != tt.input {
					t.Errorf("无效 JSON 应返回原数据，got: %s", string(result))
				}
				return
			}
			if err != nil {
				t.Errorf("结果不是有效 JSON: %v", err)
				return
			}

			t.Logf("输入: %s", tt.input)
			t.Logf("输出: %s", string(result))

			// 检查时间戳字段是否被转换
			checkTimeStampConverted(t, resultMap)
		})
	}
}

func TestNormalizeAndConvertTimestamp(t *testing.T) {
	tests := []struct {
		name     string
		input    int64
		wantYear string // 期望结果中包含的年份字符串
	}{
		{
			name:     "秒级时间戳",
			input:    1775203200, // 2026-03-12T14:00:00+08:00
			wantYear: "2026",
		},
		{
			name:     "毫秒级时间戳",
			input:    1775203200000, // 同上，毫秒级
			wantYear: "2026",
		},
		{
			name:     "秒级与毫秒级结果一致",
			input:    1743600000,
			wantYear: "2025",
		},
		{
			name:     "毫秒级与对应秒级结果一致",
			input:    1743600000000,
			wantYear: "2025",
		},
		{
			name:     "边界值：恰好等于阈值1e11（视为秒级）",
			input:    int64(1e11),
			wantYear: "5138", // 1e11 秒对应遥远的未来年份
		},
		{
			name:     "边界值：超过阈值1e11+1（视为毫秒级）",
			input:    int64(1e11) + 1,
			wantYear: "1973", // (1e11+1)/1000 ≈ 1e8 秒，对应 1973 年
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeAndConvertTimestamp(tt.input)
			// 验证结果是 ISO8601 格式（包含 'T'）
			if len(got) == 0 || !containsISO8601Markers(got) {
				t.Errorf("结果不是 ISO8601 格式: %s", got)
			}
			// 验证年份
			if len(got) < 4 || got[:4] != tt.wantYear {
				t.Errorf("期望年份 %s，实际结果: %s", tt.wantYear, got)
			}
			t.Logf("输入: %d -> 输出: %s", tt.input, got)
		})
	}

	// 额外验证：秒级和毫秒级同一时刻的转换结果应相同
	t.Run("秒级与毫秒级转换结果一致性", func(t *testing.T) {
		secTs := int64(1775203200)
		msTs := int64(1775203200000)
		if normalizeAndConvertTimestamp(secTs) != normalizeAndConvertTimestamp(msTs) {
			t.Errorf("秒级(%d)与毫秒级(%d)转换结果不一致: %s vs %s",
				secTs, msTs,
				normalizeAndConvertTimestamp(secTs),
				normalizeAndConvertTimestamp(msTs))
		}
	})
}

// checkTimeStampConverted 递归检查时间戳字段是否已转换为 ISO8601 格式
func checkTimeStampConverted(t *testing.T, data interface{}) {
	switch v := data.(type) {
	case map[string]interface{}:
		for key, value := range v {
			if key == "start_time" || key == "end_time" {
				if strVal, ok := value.(string); ok {
					// 如果是字符串，检查是否为 ISO8601 格式（包含 'T' 或 '-'）
					if len(strVal) > 0 && strVal[0] >= '0' && strVal[0] <= '9' && !containsISO8601Markers(strVal) {
						t.Errorf("字段 %s 未转换，仍为时间戳: %s", key, strVal)
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

// containsISO8601Markers 检查字符串是否包含 ISO8601 格式的特征
func containsISO8601Markers(s string) bool {
	// ISO8601 格式包含 'T' 或日期分隔符 '-'
	return len(s) > 10 && (s[4] == '-' || s[10] == 'T')
}

func TestTimestampConverter(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		wantISO  bool   // 是否期望返回 ISO8601 字符串
		wantYear string // 期望年份前缀
	}{
		{
			name:     "float64 秒级时间戳",
			input:    float64(1775203200),
			wantISO:  true,
			wantYear: "2026",
		},
		{
			name:     "float64 毫秒级时间戳",
			input:    float64(1775203200000),
			wantISO:  true,
			wantYear: "2026",
		},
		{
			name:     "string 秒级时间戳",
			input:    "1775203200",
			wantISO:  true,
			wantYear: "2026",
		},
		{
			name:     "string 毫秒级时间戳",
			input:    "1775203200000",
			wantISO:  true,
			wantYear: "2026",
		},
		{
			name:    "非数字字符串原样返回",
			input:   "not-a-timestamp",
			wantISO: false,
		},
		{
			name:    "nil 原样返回",
			input:   nil,
			wantISO: false,
		},
		{
			name:    "bool 类型原样返回",
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
					t.Fatalf("期望返回 string，实际返回 %T: %v", got, got)
				}
				if !containsISO8601Markers(str) {
					t.Errorf("期望 ISO8601 格式，实际: %s", str)
				}
				if tt.wantYear != "" && (len(str) < 4 || str[:4] != tt.wantYear) {
					t.Errorf("期望年份 %s，实际: %s", tt.wantYear, str)
				}
				t.Logf("输入: %v -> 输出: %s", tt.input, str)
			} else {
				if got != tt.input {
					t.Errorf("期望原样返回 %v，实际: %v", tt.input, got)
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
		{name: "float64 未知类型", input: float64(99), want: "Unknown"},
		{name: "string PC", input: "1", want: "PC"},
		{name: "string Mac", input: "2", want: "Mac"},
		{name: "string 未知类型", input: "999", want: "Unknown"},
		{name: "非数字字符串原样返回", input: "abc", want: "abc"},
		{name: "nil 原样返回", input: nil, want: nil},
		{name: "bool 原样返回", input: false, want: false},
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
			name:  "float64 零毫秒",
			input: float64(0),
			want:  "00:00",
		},
		{
			name:  "float64 不足一分钟（45秒）",
			input: float64(45000),
			want:  "00:45",
		},
		{
			name:  "float64 整一分钟",
			input: float64(60000),
			want:  "01:00",
		},
		{
			name:  "float64 不足一小时（59分59秒）",
			input: float64(3599000),
			want:  "59:59",
		},
		{
			name:  "float64 整一小时",
			input: float64(3600000),
			want:  "01:00:00",
		},
		{
			name:  "float64 超过一小时（1小时1分1秒）",
			input: float64(3661000),
			want:  "01:01:01",
		},
		{
			name:  "string 零毫秒",
			input: "0",
			want:  "00:00",
		},
		{
			name:  "string 不足一分钟（30秒）",
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
