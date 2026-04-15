package enumerate

import "testing"

func TestRecordTypeName(t *testing.T) {
	tests := []struct {
		id   int
		want string
	}{
		{0, "云录制"},
		{2, "上传录制"},
		{3, "文字转写"},
		{4, "视频录制"},
		{5, "音频录制"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := RecordTypeName(tt.id)
			if got != tt.want {
				t.Errorf("RecordTypeName(%d) = %q, want %q", tt.id, got, tt.want)
			}
		})
	}
}

func TestRecordTypeName_Unknown(t *testing.T) {
	unknownIDs := []int{-1, 1, 6, 99, 1000}
	for _, id := range unknownIDs {
		t.Run("unknown", func(t *testing.T) {
			got := RecordTypeName(id)
			if got != "Unknown" {
				t.Errorf("RecordTypeName(%d) = %q, want %q", id, got, "Unknown")
			}
		})
	}
}
