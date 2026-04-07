package utils

import "testing"

func TestInstanceTypeName(t *testing.T) {
	tests := []struct {
		id   int
		want string
	}{
		{0, "PSTN"},
		{1, "PC"},
		{2, "Mac"},
		{3, "Android"},
		{4, "iOS"},
		{5, "Web"},
		{6, "iPad"},
		{7, "Android Pad"},
		{8, "小程序"},
		{9, "voip/sip 设备"},
		{10, "Linux"},
		{12, "Vision Pro"},
		{20, "Rooms for Touch Windows"},
		{21, "Rooms for Touch macOS"},
		{22, "Rooms for Touch Android"},
		{30, "Controller for Touch Windows"},
		{32, "Controller for Touch Android"},
		{33, "Controller for Touch iOS"},
		{81, "HarmonyOS Phone"},
		{82, "HarmonyOS Tablet"},
		{83, "HarmonyOS PC"},
		{84, "HarmonyOS Intelligent Cockpit"},
		{86, "HarmonyOS AR/VR"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := InstanceTypeName(tt.id)
			if got != tt.want {
				t.Errorf("InstanceTypeName(%d) = %q, want %q", tt.id, got, tt.want)
			}
		})
	}
}

func TestInstanceTypeName_Unknown(t *testing.T) {
	unknownIDs := []int{-1, 11, 13, 99, 1000}
	for _, id := range unknownIDs {
		t.Run("unknown", func(t *testing.T) {
			got := InstanceTypeName(id)
			if got != "Unknown" {
				t.Errorf("InstanceTypeName(%d) = %q, want %q", id, got, "Unknown")
			}
		})
	}
}
