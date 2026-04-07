package utils

import "testing"

// TestISO8601ToTimeStamp 测试ISO8601ToTimeStamp
func TestISO8601ToTimeStamp(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantTs    int64
		wantError bool
	}{
		{
			name:   "正常解析 UTC+8 时区（有秒）",
			input:  "2026-03-12T14:00:00+08:00",
			wantTs: 1773295200,
		},
		{
			name:   "正常解析 UTC+8 时区（无秒）",
			input:  "2026-03-12T14:00+08:00",
			wantTs: 1773295200,
		},
		{
			name:   "正常解析 UTC 时区",
			input:  "2026-03-12T06:00:00Z",
			wantTs: 1773295200,
		},
		{
			name:   "正常解析 UTC-5 时区",
			input:  "2026-03-12T01:00:00-05:00",
			wantTs: 1773295200,
		},
		{
			name:      "无效格式：缺少时区",
			input:     "2026-03-12T14:00:00",
			wantError: true,
		},
		{
			name:      "无效格式：日期格式错误",
			input:     "2026/03/12 14:00:00",
			wantError: true,
		},
		{
			name:      "空字符串",
			input:     "",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ISO8601ToTimeStamp(tt.input)
			if tt.wantError {
				if err == nil {
					t.Errorf("期望返回错误，但未返回错误，got=%d", got)
				}
				return
			}
			if err != nil {
				t.Errorf("不期望返回错误，但返回了错误：%v", err)
				return
			}
			if got != tt.wantTs {
				t.Errorf("时间戳不匹配：got=%d, want=%d", got, tt.wantTs)
			}
		})
	}
}

// TestTimeStampToISO8601 测试时间戳转ISO8601格式
func TestTimeStampToISO8601(t *testing.T) {
	tests := []struct {
		name      string
		input     int64
		wantError bool
	}{
		{
			name:  "正常时间戳",
			input: 1773295200, // 2026-03-12T14:00:00+08:00 或 2026-03-12T06:00:00Z
		},
		{
			name:  "零值时间戳",
			input: 0, // 1970-01-01T00:00:00Z
		},
		{
			name:  "负数时间戳（1970年之前）",
			input: -86400, // 1969-12-31T00:00:00Z
		},
		{
			name:  "大时间戳（未来时间）",
			input: 2147483647, // 2038-01-19T03:14:07Z (32位有符号整数最大值)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TimeStampToISO8601(tt.input)

			// 验证输出不为空
			if got == "" {
				t.Errorf("返回空字符串")
				return
			}

			// 验证输出可以被解析回时间戳（证明格式正确）
			parsed, err := ISO8601ToTimeStamp(got)
			if err != nil {
				t.Errorf("输出格式无效，无法解析：%s, error: %v", got, err)
				return
			}

			// 验证解析后的时间戳与输入一致
			if parsed != tt.input {
				t.Errorf("时间戳不匹配：parsed=%d, input=%d", parsed, tt.input)
			}
		})
	}
}

// TestDurationSecondsToHMS 测试秒数转时间格式字符串
func TestDurationSecondsToHMS(t *testing.T) {
	tests := []struct {
		name  string
		input int64
		want  string
	}{
		{
			name:  "零秒",
			input: 0,
			want:  "00:00",
		},
		{
			name:  "不足一分钟",
			input: 45,
			want:  "00:45",
		},
		{
			name:  "整一分钟",
			input: 60,
			want:  "01:00",
		},
		{
			name:  "不足一小时",
			input: 3599,
			want:  "59:59",
		},
		{
			name:  "整一小时",
			input: 3600,
			want:  "01:00:00",
		},
		{
			name:  "超过一小时",
			input: 3661,
			want:  "01:01:01",
		},
		{
			name:  "多小时",
			input: 7322,
			want:  "02:02:02",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := durationSecondsToHMS(tt.input)
			if got != tt.want {
				t.Errorf("durationSecondsToHMS(%d) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
