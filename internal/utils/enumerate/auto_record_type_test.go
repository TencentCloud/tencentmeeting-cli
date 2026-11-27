package enumerate

import "testing"

func TestAutoRecordTypeName(t *testing.T) {
	tests := []struct {
		value string
		want  string
	}{
		{"none", "关闭"},
		{"cloud", "云录制"},
	}

	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			got := AutoRecordTypeName(tt.value)
			if got != tt.want {
				t.Errorf("AutoRecordTypeName(%q) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}

func TestAutoRecordTypeName_Unknown(t *testing.T) {
	unknownValues := []string{"", "NONE", "Cloud", "local", "random"}
	for _, v := range unknownValues {
		t.Run(v, func(t *testing.T) {
			if got := AutoRecordTypeName(v); got != "Unknown" {
				t.Errorf("AutoRecordTypeName(%q) = %q, want %q", v, got, "Unknown")
			}
		})
	}
}

func TestAutoRecordTypeValues(t *testing.T) {
	want := []string{"none", "cloud"}
	got := AutoRecordTypeValues()
	if len(got) != len(want) {
		t.Fatalf("AutoRecordTypeValues() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("AutoRecordTypeValues()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
