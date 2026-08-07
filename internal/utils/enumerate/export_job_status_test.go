package enumerate

import "testing"

func TestExportJobStatusName(t *testing.T) {
	tests := []struct {
		status int
		want   string
	}{
		{1, "成功"},
		{2, "失败"},
		{3, "处理中"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := ExportJobStatusName(tt.status)
			if got != tt.want {
				t.Errorf("ExportJobStatusName(%d) = %q, want %q", tt.status, got, tt.want)
			}
		})
	}
}

func TestExportJobStatusName_Unknown(t *testing.T) {
	unknownStatuses := []int{0, -1, 4, 100}
	for _, s := range unknownStatuses {
		t.Run("unknown", func(t *testing.T) {
			got := ExportJobStatusName(s)
			if got != "Unknown" {
				t.Errorf("ExportJobStatusName(%d) = %q, want %q", s, got, "Unknown")
			}
		})
	}
}
