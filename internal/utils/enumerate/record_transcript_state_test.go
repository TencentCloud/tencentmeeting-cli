package enumerate

import "testing"

func TestRecordTranscriptStateName(t *testing.T) {
	tests := []struct {
		id   int
		want string
	}{
		{0, "未生成逐字稿"},
		{1, "逐字稿生成中"},
		{2, "逐字稿生成完成"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := RecordTranscriptStateName(tt.id)
			if got != tt.want {
				t.Errorf("RecordTranscriptStateName(%d) = %q, want %q", tt.id, got, tt.want)
			}
		})
	}
}

func TestRecordTranscriptStateName_Unknown(t *testing.T) {
	unknownIDs := []int{-1, 3, 99, 1000}
	for _, id := range unknownIDs {
		t.Run("unknown", func(t *testing.T) {
			got := RecordTranscriptStateName(id)
			if got != "Unknown" {
				t.Errorf("RecordTranscriptStateName(%d) = %q, want %q", id, got, "Unknown")
			}
		})
	}
}
