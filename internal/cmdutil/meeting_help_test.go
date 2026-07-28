package cmdutil

import "testing"

// TestFormatMeetingCode cover FormatMeetingCode input.
func TestFormatMeetingCode(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "with #腾讯会议 prefix and dashes (full-width colon)",
			in:   "#腾讯会议：295-150-176",
			want: "295150176",
		},
		{
			name: "with #腾讯会议 prefix (half-width colon)",
			in:   "#腾讯会议:295-150-176",
			want: "295150176",
		},
		{
			name: "with dashes only",
			in:   "295-150-176",
			want: "295150176",
		},
		{
			name: "with spaces only",
			in:   "295 150 176",
			want: "295150176",
		},
		{
			name: "pure digits",
			in:   "295150176",
			want: "295150176",
		},
		{
			name: "leading and trailing spaces",
			in:   "  295150176  ",
			want: "295150176",
		},
		{
			name: "with tabs",
			in:   "295\t150\t176",
			want: "295150176",
		},
		{
			name: "mixed dashes spaces and tabs",
			in:   "295- 150\t176",
			want: "295150176",
		},
		{
			name: "with single quotes",
			in:   "'295-150-176'",
			want: "295150176",
		},
		{
			name: "prefix with leading whitespace",
			in:   "  #腾讯会议：295-150-176",
			want: "295150176",
		},
		{
			name: "empty string",
			in:   "",
			want: "",
		},
		{
			name: "only separators",
			in:   "- - ",
			want: "",
		},
		{
			name: "prefix only",
			in:   "#腾讯会议：",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatMeetingCode(tt.in)
			if got != tt.want {
				t.Errorf("FormatMeetingCode(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
