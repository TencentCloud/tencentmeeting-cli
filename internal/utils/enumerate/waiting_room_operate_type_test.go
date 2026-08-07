package enumerate

import "testing"

func TestWaitingRoomOperateTypeName(t *testing.T) {
	tests := []struct {
		id   int
		want string
	}{
		{1, "enter-meeting"},
		{2, "back-to-waiting"},
		{3, "expel"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := WaitingRoomOperateTypeName(tt.id)
			if got != tt.want {
				t.Errorf("WaitingRoomOperateTypeName(%d) = %q, want %q", tt.id, got, tt.want)
			}
		})
	}
}

func TestWaitingRoomOperateTypeName_Unknown(t *testing.T) {
	unknownIDs := []int{-1, 0, 4, 99, 1000}
	for _, id := range unknownIDs {
		t.Run("unknown", func(t *testing.T) {
			got := WaitingRoomOperateTypeName(id)
			if got != "Unknown" {
				t.Errorf("WaitingRoomOperateTypeName(%d) = %q, want %q", id, got, "Unknown")
			}
		})
	}
}

func TestWaitingRoomOperateTypeValue(t *testing.T) {
	tests := []struct {
		name   string
		want   WaitingRoomOperateType
		wantOk bool
	}{
		{"enter-meeting", WaitingRoomOperateEnterMeeting, true},
		{"back-to-waiting", WaitingRoomOperateBackToWaiting, true},
		{"expel", WaitingRoomOperateExpel, true},
		{"unknown", 0, false},
		{"", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := WaitingRoomOperateTypeValue(tt.name)
			if ok != tt.wantOk || (ok && got != tt.want) {
				t.Errorf("WaitingRoomOperateTypeValue(%q) = (%d, %v), want (%d, %v)",
					tt.name, got, ok, tt.want, tt.wantOk)
			}
		})
	}
}
