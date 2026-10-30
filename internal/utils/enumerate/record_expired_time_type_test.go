package enumerate

import "testing"

func TestRecordExpiredTimeTypeName(t *testing.T) {
	tests := []struct {
		id   int
		want string
	}{
		{0, "永久有效"},
		{1, "7天"},
		{2, "15天"},
		{3, "30天"},
		{4, "自定义"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := RecordExpiredTimeTypeName(tt.id)
			if got != tt.want {
				t.Errorf("RecordExpiredTimeTypeName(%d) = %q, want %q", tt.id, got, tt.want)
			}
		})
	}
}

func TestRecordExpiredTimeTypeName_Unknown(t *testing.T) {
	unknownIDs := []int{-1, 5, 6, 8, 14, 16, 29, 31, 99, 1000}
	for _, id := range unknownIDs {
		t.Run("unknown", func(t *testing.T) {
			got := RecordExpiredTimeTypeName(id)
			if got != "Unknown" {
				t.Errorf("RecordExpiredTimeTypeName(%d) = %q, want %q", id, got, "Unknown")
			}
		})
	}
}
