package enumerate

import "testing"

func TestMeetingUserRoleName(t *testing.T) {
	tests := []struct {
		id   int
		want string
	}{
		{0, "普通成员角色"},
		{1, "创建者角色"},
		{2, "主持人"},
		{3, "创建者+主持人"},
		{4, "游客"},
		{5, "游客+主持人"},
		{6, "联席主持人"},
		{7, "创建者+联席主持人"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := MeetingUserRoleName(tt.id)
			if got != tt.want {
				t.Errorf("MeetingUserRoleName(%d) = %q, want %q", tt.id, got, tt.want)
			}
		})
	}
}

func TestMeetingUserRoleName_Unknown(t *testing.T) {
	unknownIDs := []int{-1, 8, 9, 99, 1000}
	for _, id := range unknownIDs {
		t.Run("unknown", func(t *testing.T) {
			got := MeetingUserRoleName(id)
			if got != "Unknown" {
				t.Errorf("MeetingUserRoleName(%d) = %q, want %q", id, got, "Unknown")
			}
		})
	}
}
