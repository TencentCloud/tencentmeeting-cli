package enumerate

import "testing"

func TestMeetingStatusInSearchName(t *testing.T) {
	tests := []struct {
		status string
		want   string
	}{
		{"unknown", "未知状态"},
		{"upcoming", "待开始"},
		{"in_progress", "进行中"},
		{"closed", "已结束"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := MeetingStatusInSearchName(tt.status)
			if got != tt.want {
				t.Errorf("MeetingStatusInSearchName(%q) = %q, want %q", tt.status, got, tt.want)
			}
		})
	}
}

func TestMeetingStatusInSearchName_Unknown(t *testing.T) {
	unknownStatuses := []string{"", "INVALID", "UNKNOWN", "Upcoming", "IN_PROGRESS", "random"}
	for _, s := range unknownStatuses {
		t.Run("unknown", func(t *testing.T) {
			got := MeetingStatusInSearchName(s)
			if got != "Unknown" {
				t.Errorf("MeetingStatusInSearchName(%q) = %q, want %q", s, got, "Unknown")
			}
		})
	}
}
