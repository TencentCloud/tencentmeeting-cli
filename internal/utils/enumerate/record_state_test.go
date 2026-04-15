package enumerate

import "testing"

func TestRecordStateName(t *testing.T) {
	tests := []struct {
		id   int
		want string
	}{
		{1, "录制中"},
		{2, "转码中"},
		{3, "转码完成"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := RecordStateName(tt.id)
			if got != tt.want {
				t.Errorf("RecordStateName(%d) = %q, want %q", tt.id, got, tt.want)
			}
		})
	}
}

func TestRecordStateName_Unknown(t *testing.T) {
	unknownIDs := []int{-1, 0, 4, 99, 1000}
	for _, id := range unknownIDs {
		t.Run("unknown", func(t *testing.T) {
			got := RecordStateName(id)
			if got != "Unknown" {
				t.Errorf("RecordStateName(%d) = %q, want %q", id, got, "Unknown")
			}
		})
	}
}
