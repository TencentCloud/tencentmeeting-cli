package enumerate

import "testing"

func TestMeetingRecurringTypeName(t *testing.T) {
	tests := []struct {
		id   int
		want string
	}{
		{0, "每天"},
		{1, "每周一至周五"},
		{2, "每周"},
		{3, "每两周"},
		{4, "每月"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := MeetingRecurringTypeName(tt.id)
			if got != tt.want {
				t.Errorf("MeetingRecurringTypeName(%d) = %q, want %q", tt.id, got, tt.want)
			}
		})
	}
}

func TestMeetingRecurringTypeName_Unknown(t *testing.T) {
	unknownIDs := []int{-1, 5, 6, 99, 1000}
	for _, id := range unknownIDs {
		t.Run("unknown", func(t *testing.T) {
			got := MeetingRecurringTypeName(id)
			if got != "Unknown" {
				t.Errorf("MeetingRecurringTypeName(%d) = %q, want %q", id, got, "Unknown")
			}
		})
	}
}
