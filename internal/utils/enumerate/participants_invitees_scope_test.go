package enumerate

import "testing"

func TestParticipantsInviteesScopeName(t *testing.T) {
	tests := []struct {
		id   int
		want string
	}{
		{0, "可查看参会期间的逐字稿"},
		{1, "可查看整场会议的逐字稿"},
		{2, "可查看整场会议回顾"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := ParticipantsInviteesScopeName(tt.id)
			if got != tt.want {
				t.Errorf("ParticipantsInviteesScopeName(%d) = %q, want %q", tt.id, got, tt.want)
			}
		})
	}
}

func TestParticipantsInviteesScopeName_Unknown(t *testing.T) {
	unknownIDs := []int{-1, 3, 99, 1000}
	for _, id := range unknownIDs {
		t.Run("unknown", func(t *testing.T) {
			got := ParticipantsInviteesScopeName(id)
			if got != "" {
				t.Errorf("ParticipantsInviteesScopeName(%d) = %q, want %q", id, got, "")
			}
		})
	}
}
