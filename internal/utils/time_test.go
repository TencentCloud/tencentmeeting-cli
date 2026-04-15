package utils

import "testing"

// TestISO8601ToTimeStamp tests ISO8601ToTimeStamp
func TestISO8601ToTimeStamp(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantTs    int64
		wantError bool
	}{
		{
			name:   "parse UTC+8 with seconds",
			input:  "2026-03-12T14:00:00+08:00",
			wantTs: 1773295200,
		},
		{
			name:   "parse UTC+8 without seconds",
			input:  "2026-03-12T14:00+08:00",
			wantTs: 1773295200,
		},
		{
			name:   "parse UTC timezone",
			input:  "2026-03-12T06:00:00Z",
			wantTs: 1773295200,
		},
		{
			name:   "parse UTC-5 timezone",
			input:  "2026-03-12T01:00:00-05:00",
			wantTs: 1773295200,
		},
		{
			name:      "invalid format: missing timezone",
			input:     "2026-03-12T14:00:00",
			wantError: true,
		},
		{
			name:      "invalid format: wrong date format",
			input:     "2026/03/12 14:00:00",
			wantError: true,
		},
		{
			name:      "empty string",
			input:     "",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ISO8601ToTimeStamp(tt.input)
			if tt.wantError {
				if err == nil {
					t.Errorf("expected error but got none, got=%d", got)
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if got != tt.wantTs {
				t.Errorf("timestamp mismatch: got=%d, want=%d", got, tt.wantTs)
			}
		})
	}
}

// TestTimeStampToISO8601 tests timestamp to ISO8601 conversion
func TestTimeStampToISO8601(t *testing.T) {
	tests := []struct {
		name      string
		input     int64
		wantError bool
	}{
		{
			name:  "normal timestamp",
			input: 1773295200, // 2026-03-12T14:00:00+08:00 or 2026-03-12T06:00:00Z
		},
		{
			name:  "negative timestamp (before 1970)",
			input: -86400, // 1969-12-31T00:00:00Z
		},
		{
			name:  "large timestamp (future)",
			input: 2147483647, // 2038-01-19T03:14:07Z (max int32)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TimeStampToISO8601(tt.input)

			// verify output is not empty
			if got == "" {
				t.Errorf("returned empty string")
				return
			}

			// verify output can be parsed back to timestamp (proves format is correct)
			parsed, err := ISO8601ToTimeStamp(got)
			if err != nil {
				t.Errorf("output format invalid, cannot parse: %s, error: %v", got, err)
				return
			}

			// verify parsed timestamp matches input
			if parsed != tt.input {
				t.Errorf("timestamp mismatch: parsed=%d, input=%d", parsed, tt.input)
			}
		})
	}
}

// TestDurationSecondsToHMS tests seconds to HH:MM:SS format conversion
func TestDurationSecondsToHMS(t *testing.T) {
	tests := []struct {
		name  string
		input int64
		want  string
	}{
		{
			name:  "zero seconds",
			input: 0,
			want:  "00:00",
		},
		{
			name:  "less than one minute",
			input: 45,
			want:  "00:45",
		},
		{
			name:  "exactly one minute",
			input: 60,
			want:  "01:00",
		},
		{
			name:  "less than one hour",
			input: 3599,
			want:  "59:59",
		},
		{
			name:  "exactly one hour",
			input: 3600,
			want:  "01:00:00",
		},
		{
			name:  "more than one hour",
			input: 3661,
			want:  "01:01:01",
		},
		{
			name:  "multiple hours",
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
