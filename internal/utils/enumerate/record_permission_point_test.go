package enumerate

import "testing"

func TestRecordPermissionPointName(t *testing.T) {
	tests := []struct {
		id   int
		want string
	}{
		{1, "所有人可查看"},
		{2, "所有人需输入密码查看"},
		{3, "所有人需申请权限查看"},
		{4, "同企业可查看"},
		{5, "同企业需输入密码查看"},
		{6, "同企业需申请权限查看"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := RecordPermissionPointName(tt.id)
			if got != tt.want {
				t.Errorf("RecordPermissionPointName(%d) = %q, want %q", tt.id, got, tt.want)
			}
		})
	}
}

func TestRecordPermissionPointName_Unknown(t *testing.T) {
	unknownIDs := []int{-1, 0, 7, 99, 1000}
	for _, id := range unknownIDs {
		t.Run("unknown", func(t *testing.T) {
			got := RecordPermissionPointName(id)
			if got != "Unknown" {
				t.Errorf("RecordPermissionPointName(%d) = %q, want %q", id, got, "Unknown")
			}
		})
	}
}
