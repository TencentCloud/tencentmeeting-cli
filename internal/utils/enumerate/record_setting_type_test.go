package enumerate

import "testing"

func TestRecordSettingTypeName(t *testing.T) {
	tests := []struct {
		id   int
		want string
	}{
		{1, "关闭"},
		{2, "开启"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := RecordSettingTypeName(tt.id)
			if got != tt.want {
				t.Errorf("RecordSettingTypeName(%d) = %q, want %q", tt.id, got, tt.want)
			}
		})
	}
}

func TestRecordSettingTypeName_Unknown(t *testing.T) {
	unknownIDs := []int{-1, 0, 3, 99, 1000}
	for _, id := range unknownIDs {
		t.Run("unknown", func(t *testing.T) {
			got := RecordSettingTypeName(id)
			if got != "Unknown" {
				t.Errorf("RecordSettingTypeName(%d) = %q, want %q", id, got, "Unknown")
			}
		})
	}
}
