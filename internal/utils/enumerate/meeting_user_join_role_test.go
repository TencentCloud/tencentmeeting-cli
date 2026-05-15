package enumerate

import "testing"

func TestMeetingUserJoinRoleName(t *testing.T) {
	tests := []struct {
		id   string
		want string
	}{
		{"creator", "创建者"},
		{"hoster", "主持人"},
		{"invitee", "被邀请者"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := MeetingUserJoinRoleName(tt.id)
			if got != tt.want {
				t.Errorf("MeetingUserJoinRoleName(%q) = %q, want %q", tt.id, got, tt.want)
			}
		})
	}
}

func TestMeetingUserJoinRoleName_Unknown(t *testing.T) {
	unknownIDs := []string{"", "unknown", "Creator", "HOSTER", "guest", "member"}
	for _, id := range unknownIDs {
		t.Run("unknown", func(t *testing.T) {
			got := MeetingUserJoinRoleName(id)
			if got != "Unknown" {
				t.Errorf("MeetingUserJoinRoleName(%q) = %q, want %q", id, got, "Unknown")
			}
		})
	}
}
