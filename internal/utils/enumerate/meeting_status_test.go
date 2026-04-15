package enumerate

import "testing"

func TestMeetingStatusName(t *testing.T) {
	tests := []struct {
		status string
		want   string
	}{
		{"MEETING_STATE_INVALID", "非法状态"},
		{"MEETING_STATE_INIT", "待开始"},
		{"MEETING_STATE_CANCELLED", "已取消"},
		{"MEETING_STATE_STARTED", "会议中"},
		{"MEETING_STATE_ENDED", "已删除"},
		{"MEETING_STATE_NULL", "无状态"},
		{"MEETING_STATE_RECYCLED", "已回收"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := MeetingStatusName(tt.status)
			if got != tt.want {
				t.Errorf("MeetingStatusName(%q) = %q, want %q", tt.status, got, tt.want)
			}
		})
	}
}

func TestMeetingStatusName_Unknown(t *testing.T) {
	unknownStatuses := []string{"", "INVALID", "MEETING_STATE_UNKNOWN", "meeting_state_init", "random"}
	for _, s := range unknownStatuses {
		t.Run("unknown", func(t *testing.T) {
			got := MeetingStatusName(s)
			if got != "Unknown" {
				t.Errorf("MeetingStatusName(%q) = %q, want %q", s, got, "Unknown")
			}
		})
	}
}
