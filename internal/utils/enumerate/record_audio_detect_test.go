package enumerate

import "testing"

func TestRecordAudioDetectName(t *testing.T) {
	tests := []struct {
		id   int
		want string
	}{
		{0, "未完成"},
		{1, "已完成"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := RecordAudioDetectName(tt.id)
			if got != tt.want {
				t.Errorf("RecordAudioDetectName(%d) = %q, want %q", tt.id, got, tt.want)
			}
		})
	}
}

func TestRecordAudioDetectName_Unknown(t *testing.T) {
	unknownIDs := []int{-1, 2, 3, 99, 1000}
	for _, id := range unknownIDs {
		t.Run("unknown", func(t *testing.T) {
			got := RecordAudioDetectName(id)
			if got != "Unknown" {
				t.Errorf("RecordAudioDetectName(%d) = %q, want %q", id, got, "Unknown")
			}
		})
	}
}
