package utils

import (
	"testing"
	"tmeet/internal/exception"
)

// TestSplitAndTrim tests SplitAndTrim function with various scenarios
func TestSplitAndTrim(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: []string{},
		},
		{
			name:     "single element",
			input:    "hello",
			expected: []string{"hello"},
		},
		{
			name:     "multiple elements without spaces",
			input:    "a,b,c,d",
			expected: []string{"a", "b", "c", "d"},
		},
		{
			name:     "multiple elements with spaces",
			input:    " a , b , c , d ",
			expected: []string{"a", "b", "c", "d"},
		},
		{
			name:     "elements with empty strings",
			input:    "a,,b,,c,",
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "only spaces and commas",
			input:    " , , , ",
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SplitAndTrim(tt.input)
			if len(got) != len(tt.expected) {
				t.Errorf("SplitAndTrim(%q) length mismatch: got %d, want %d", tt.input, len(got), len(tt.expected))
				return
			}
			for i := range got {
				if got[i] != tt.expected[i] {
					t.Errorf("SplitAndTrim(%q) element %d mismatch: got %q, want %q", tt.input, i, got[i], tt.expected[i])
				}
			}
		})
	}
}

// TestValidatePhone tests ValidatePhone function with various phone number formats
func TestValidatePhone(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantError bool
	}{
		{
			name:      "valid phone number starting with 13",
			input:     "13800138000",
			wantError: false,
		},
		{
			name:      "valid phone number starting with 18",
			input:     "18812345678",
			wantError: false,
		},
		{
			name:      "valid phone number starting with 19",
			input:     "19987654321",
			wantError: false,
		},
		{
			name:      "invalid phone number starting with 12",
			input:     "12800138000",
			wantError: true,
		},
		{
			name:      "invalid phone number with 10 digits",
			input:     "1380013800",
			wantError: true,
		},
		{
			name:      "invalid phone number with 12 digits",
			input:     "138001380000",
			wantError: true,
		},
		{
			name:      "invalid phone number with letters",
			input:     "1380013800a",
			wantError: true,
		},
		{
			name:      "empty string",
			input:     "",
			wantError: true,
		},
		{
			name:      "phone number with spaces",
			input:     "138 0013 8000",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePhone(tt.input)
			if tt.wantError {
				if err == nil {
					t.Errorf("ValidatePhone(%q) expected error but got none", tt.input)
					return
				}
				if !exception.Is(err, exception.InvalidArgsError) {
					t.Errorf("ValidatePhone(%q) expected InvalidArgsError, but got: %v", tt.input, err)
				}
			} else if err != nil {
				t.Errorf("ValidatePhone(%q) unexpected error: %v", tt.input, err)
			}
		})
	}
}

// TestValidateEmail tests ValidateEmail function with various email formats
func TestValidateEmail(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantError bool
	}{
		{
			name:      "valid standard email",
			input:     "test@example.com",
			wantError: false,
		},
		{
			name:      "valid email with numbers",
			input:     "user123@domain.com",
			wantError: false,
		},
		{
			name:      "valid email with dots",
			input:     "first.last@company.co.uk",
			wantError: false,
		},
		{
			name:      "valid email with underscore",
			input:     "user_name@domain.org",
			wantError: false,
		},
		{
			name:      "valid email with hyphen",
			input:     "user-name@sub.domain.com",
			wantError: false,
		},
		{
			name:      "email exceeds 100 characters",
			input:     "verylongemailaddresswithlotsofcharactersandnumbers1234567890123456789012345678901234567890@example.com",
			wantError: true,
		},
		{
			name:      "email missing @ symbol",
			input:     "invalidemail.com",
			wantError: true,
		},
		{
			name:      "email with @ at beginning",
			input:     "@example.com",
			wantError: true,
		},
		{
			name:      "email with @ at end",
			input:     "user@",
			wantError: true,
		},
		{
			name:      "email with quoted local part",
			input:     "\"user\"@example.com",
			wantError: true,
		},
		{
			name:      "email with consecutive dots",
			input:     "user..name@example.com",
			wantError: true,
		},
		{
			name:      "email with IP address domain",
			input:     "user@[192.168.1.1]",
			wantError: true,
		},
		{
			name:      "email with invalid characters",
			input:     "user*name@example.com",
			wantError: true,
		},
		{
			name:      "email with domain missing dot",
			input:     "user@examplecom",
			wantError: true,
		},
		{
			name:      "email with domain ending with dot",
			input:     "user@example.",
			wantError: true,
		},
		{
			name:      "email with domain starting with dot",
			input:     "user@.example.com",
			wantError: true,
		},
		{
			name:      "empty string",
			input:     "",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateEmail(tt.input)
			if tt.wantError {
				if err == nil {
					t.Errorf("ValidateEmail(%q) expected error but got none", tt.input)
					return
				}
				if !exception.Is(err, exception.InvalidArgsError) {
					t.Errorf("ValidateEmail(%q) expected InvalidArgsError, but got: %v", tt.input, err)
				}
			} else if err != nil {
				t.Errorf("ValidateEmail(%q) unexpected error: %v", tt.input, err)
			}
		})
	}
}
