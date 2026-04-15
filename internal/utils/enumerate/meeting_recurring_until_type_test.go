package enumerate

import "testing"

func TestMeetingRecurringUntilTypeName(t *testing.T) {
	tests := []struct {
		id   int
		want string
	}{
		{0, "按日期结束重复"},
		{1, "按次数结束重复"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := MeetingRecurringUntilTypeName(tt.id)
			if got != tt.want {
				t.Errorf("MeetingRecurringUntilTypeName(%d) = %q, want %q", tt.id, got, tt.want)
			}
		})
	}
}

func TestMeetingRecurringUntilTypeName_Unknown(t *testing.T) {
	unknownIDs := []int{-1, 2, 3, 99, 1000}
	for _, id := range unknownIDs {
		t.Run("unknown", func(t *testing.T) {
			got := MeetingRecurringUntilTypeName(id)
			if got != "Unknown" {
				t.Errorf("MeetingRecurringUntilTypeName(%d) = %q, want %q", id, got, "Unknown")
			}
		})
	}
}
