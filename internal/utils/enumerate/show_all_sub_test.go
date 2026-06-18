package enumerate

import "testing"

func TestShowAllSubMeetingsName(t *testing.T) {
	tests := []struct {
		id   int
		want string
	}{
		{0, "no"},
		{1, "yes"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := ShowAllSubMeetingsName(tt.id)
			if got != tt.want {
				t.Errorf("ShowAllSubMeetingsName(%d) = %q, want %q", tt.id, got, tt.want)
			}
		})
	}
}

func TestShowAllSubMeetingsName_Unknown(t *testing.T) {
	unknownIDs := []int{-1, 2, 3, 99, 1000}
	for _, id := range unknownIDs {
		t.Run("unknown", func(t *testing.T) {
			got := ShowAllSubMeetingsName(id)
			if got != "unknown" {
				t.Errorf("ShowAllSubMeetingsName(%d) = %q, want %q", id, got, "unknown")
			}
		})
	}
}
