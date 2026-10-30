package enumerate

import "testing"

func TestRecordingTimelineStatusName(t *testing.T) {
	tests := []struct {
		id   int
		want string
	}{
		{0, "not generated"},
		{1, "generating"},
		{2, "generated"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := RecordingTimelineStatusName(tt.id)
			if got != tt.want {
				t.Errorf("RecordingTimelineStatusName(%d) = %q, want %q", tt.id, got, tt.want)
			}
		})
	}
}

func TestRecordingTimelineStatusName_Unknown(t *testing.T) {
	unknownIDs := []int{-1, 3, 99, 1000}
	for _, id := range unknownIDs {
		t.Run("unknown", func(t *testing.T) {
			got := RecordingTimelineStatusName(id)
			if got != "Unknown" {
				t.Errorf("RecordingTimelineStatusName(%d) = %q, want %q", id, got, "Unknown")
			}
		})
	}
}
