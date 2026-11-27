package enumerate

import "testing"

func TestRecordParticipantPermName(t *testing.T) {
	tests := []struct {
		id   int
		want string
	}{
		{1, "可查看参会期间的逐字稿"},
		{2, "可查看整场会议的逐字稿和纪要"},
		{3, "需指定分享或者链接分享"},
		{4, "可查看全部回顾内容"},
		{5, "可查看整场会议的纪要"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := RecordParticipantPermName(tt.id)
			if got != tt.want {
				t.Errorf("RecordParticipantPermName(%d) = %q, want %q", tt.id, got, tt.want)
			}
		})
	}
}

func TestRecordParticipantPermName_Unknown(t *testing.T) {
	unknownIDs := []int{-1, 0, 6, 99, 1000}
	for _, id := range unknownIDs {
		t.Run("unknown", func(t *testing.T) {
			got := RecordParticipantPermName(id)
			if got != "Unknown" {
				t.Errorf("RecordParticipantPermName(%d) = %q, want %q", id, got, "Unknown")
			}
		})
	}
}
