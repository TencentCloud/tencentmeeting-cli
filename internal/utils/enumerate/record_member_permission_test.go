package enumerate

import "testing"

func TestRecordMemberPermissionName(t *testing.T) {
	tests := []struct {
		id   int
		want string
	}{
		{0, "可查看"},
		{1, "可管理"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := RecordMemberPermissionName(tt.id)
			if got != tt.want {
				t.Errorf("RecordMemberPermissionName(%d) = %q, want %q", tt.id, got, tt.want)
			}
		})
	}
}

func TestRecordMemberPermissionName_Unknown(t *testing.T) {
	unknownIDs := []int{-1, 2, 99, 1000}
	for _, id := range unknownIDs {
		t.Run("unknown", func(t *testing.T) {
			got := RecordMemberPermissionName(id)
			if got != "Unknown" {
				t.Errorf("RecordMemberPermissionName(%d) = %q, want %q", id, got, "Unknown")
			}
		})
	}
}
