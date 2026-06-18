package enumerate

import "testing"

func TestMeetingJoinTypeName(t *testing.T) {
	tests := []struct {
		id   int
		want string
	}{
		{1, "all"},
		{2, "invited"},
		{3, "internal"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := MeetingJoinTypeName(tt.id)
			if got != tt.want {
				t.Errorf("MeetingJoinTypeName(%d) = %q, want %q", tt.id, got, tt.want)
			}
		})
	}
}

func TestMeetingJoinTypeName_Unknown(t *testing.T) {
	unknownIDs := []int{-1, 0, 4, 99, 1000}
	for _, id := range unknownIDs {
		t.Run("unknown", func(t *testing.T) {
			got := MeetingJoinTypeName(id)
			if got != "unknown" {
				t.Errorf("MeetingJoinTypeName(%d) = %q, want %q", id, got, "unknown")
			}
		})
	}
}
