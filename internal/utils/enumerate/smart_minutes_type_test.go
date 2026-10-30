package enumerate

import "testing"

func TestSmartMinutesMinuteTypeName(t *testing.T) {
	tests := []struct {
		id   int
		want string
	}{
		{1, "按章节"},
		{2, "按主题"},
		{3, "按发言人"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := SmartMinutesMinuteTypeName(tt.id)
			if got != tt.want {
				t.Errorf("SmartMinutesMinuteTypeName(%d) = %q, want %q", tt.id, got, tt.want)
			}
		})
	}
}

func TestSmartMinutesMinuteTypeName_Unknown(t *testing.T) {
	unknownIDs := []int{-1, 0, 4, 99, 1000}
	for _, id := range unknownIDs {
		t.Run("unknown", func(t *testing.T) {
			got := SmartMinutesMinuteTypeName(id)
			if got != "Unknown" {
				t.Errorf("SmartMinutesMinuteTypeName(%d) = %q, want %q", id, got, "Unknown")
			}
		})
	}
}

func TestSmartMinutesTextTypeName(t *testing.T) {
	tests := []struct {
		id   int
		want string
	}{
		{1, "纯文本"},
		{2, "markdown"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := SmartMinutesTextTypeName(tt.id)
			if got != tt.want {
				t.Errorf("SmartMinutesTextTypeName(%d) = %q, want %q", tt.id, got, tt.want)
			}
		})
	}
}

func TestSmartMinutesTextTypeName_Unknown(t *testing.T) {
	unknownIDs := []int{-1, 0, 3, 99, 1000}
	for _, id := range unknownIDs {
		t.Run("unknown", func(t *testing.T) {
			got := SmartMinutesTextTypeName(id)
			if got != "Unknown" {
				t.Errorf("SmartMinutesTextTypeName(%d) = %q, want %q", id, got, "Unknown")
			}
		})
	}
}

func TestSmartMinutesIndustryName(t *testing.T) {
	tests := []struct {
		id   int
		want string
	}{
		{0, "通用模型"},
		{1, "金融模型"},
		{2, "面试"},
		{3, "教育"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := SmartMinutesIndustryName(tt.id)
			if got != tt.want {
				t.Errorf("SmartMinutesIndustryName(%d) = %q, want %q", tt.id, got, tt.want)
			}
		})
	}
}

func TestSmartMinutesIndustryName_Unknown(t *testing.T) {
	unknownIDs := []int{-1, 4, 5, 99, 1000}
	for _, id := range unknownIDs {
		t.Run("unknown", func(t *testing.T) {
			got := SmartMinutesIndustryName(id)
			if got != "Unknown" {
				t.Errorf("SmartMinutesIndustryName(%d) = %q, want %q", id, got, "Unknown")
			}
		})
	}
}

func TestSmartMinutesLLMName(t *testing.T) {
	tests := []struct {
		id   int
		want string
	}{
		{1, "混元"},
		{2, "deepseek"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := SmartMinutesLLMName(tt.id)
			if got != tt.want {
				t.Errorf("SmartMinutesLLMName(%d) = %q, want %q", tt.id, got, tt.want)
			}
		})
	}
}

func TestSmartMinutesLLMName_Unknown(t *testing.T) {
	unknownIDs := []int{-1, 0, 3, 99, 1000}
	for _, id := range unknownIDs {
		t.Run("unknown", func(t *testing.T) {
			got := SmartMinutesLLMName(id)
			if got != "Unknown" {
				t.Errorf("SmartMinutesLLMName(%d) = %q, want %q", id, got, "Unknown")
			}
		})
	}
}
