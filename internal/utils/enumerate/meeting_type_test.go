package enumerate

import "testing"

func TestMeetingTypeName(t *testing.T) {
	tests := []struct {
		id   int
		want string
	}{
		{0, "普通会议"},
		{1, "周期性会议"},
		{2, "微信专属会议"},
		{4, "Rooms 投屏会议"},
		{5, "个人会议号会议"},
		{6, "网络研讨会"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := MeetingTypeName(tt.id)
			if got != tt.want {
				t.Errorf("MeetingTypeName(%d) = %q, want %q", tt.id, got, tt.want)
			}
		})
	}
}

func TestMeetingTypeName_Unknown(t *testing.T) {
	unknownIDs := []int{-1, 3, 7, 99, 1000}
	for _, id := range unknownIDs {
		t.Run("unknown", func(t *testing.T) {
			got := MeetingTypeName(id)
			if got != "Unknown" {
				t.Errorf("MeetingTypeName(%d) = %q, want %q", id, got, "Unknown")
			}
		})
	}
}
