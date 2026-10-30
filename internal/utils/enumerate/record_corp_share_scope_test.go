package enumerate

import "testing"

func TestRecordCorpShareScopeName(t *testing.T) {
	tests := []struct {
		id   int
		want string
	}{
		{1, "不限制"},
		{2, "企业已关闭分享"},
		{3, "仅允许同企业范围"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := RecordCorpShareScopeName(tt.id)
			if got != tt.want {
				t.Errorf("RecordCorpShareScopeName(%d) = %q, want %q", tt.id, got, tt.want)
			}
		})
	}
}

func TestRecordCorpShareScopeName_Unknown(t *testing.T) {
	unknownIDs := []int{-1, 0, 4, 5, 99, 1000}
	for _, id := range unknownIDs {
		t.Run("unknown", func(t *testing.T) {
			got := RecordCorpShareScopeName(id)
			if got != "Unknown" {
				t.Errorf("RecordCorpShareScopeName(%d) = %q, want %q", id, got, "Unknown")
			}
		})
	}
}
