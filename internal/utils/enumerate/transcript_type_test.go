package enumerate

import "testing"

func TestTranscriptTypeName(t *testing.T) {
	tests := []struct {
		id   int
		want string
	}{
		{0, "原文版"},
		{1, "智能优化版"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := TranscriptTypeName(tt.id)
			if got != tt.want {
				t.Errorf("TranscriptTypeName(%d) = %q, want %q", tt.id, got, tt.want)
			}
		})
	}
}

func TestTranscriptTypeName_Unknown(t *testing.T) {
	unknownIDs := []int{-1, 2, 3, 99, 1000}
	for _, id := range unknownIDs {
		t.Run("unknown", func(t *testing.T) {
			got := TranscriptTypeName(id)
			if got != "Unknown" {
				t.Errorf("TranscriptTypeName(%d) = %q, want %q", id, got, "Unknown")
			}
		})
	}
}
