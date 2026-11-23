package enumerate

import "testing"

func TestMeetingAttachmentTypeName(t *testing.T) {
	tests := []struct {
		id   int
		want string
	}{
		{1, "腾讯文档"},
		{2, "本地上传文件"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := MeetingAttachmentTypeName(tt.id)
			if got != tt.want {
				t.Errorf("MeetingAttachmentTypeName(%d) = %q, want %q", tt.id, got, tt.want)
			}
		})
	}
}

func TestMeetingAttachmentTypeName_Unknown(t *testing.T) {
	unknownIDs := []int{-1, 0, 3, 99, 1000}
	for _, id := range unknownIDs {
		t.Run("unknown", func(t *testing.T) {
			got := MeetingAttachmentTypeName(id)
			if got != "Unknown" {
				t.Errorf("MeetingAttachmentTypeName(%d) = %q, want %q", id, got, "Unknown")
			}
		})
	}
}
