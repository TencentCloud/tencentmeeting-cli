package utils

import (
	"regexp"
	"strings"
	"tmeet/internal/exception"
)

// phoneRegex is a pre-compiled regular expression for validating phone numbers.
var phoneRegex = regexp.MustCompile(`^1[3-9]\d{9}$`)

// emailRegex is a pre-compiled regular expression for validating email addresses.
var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9][a-zA-Z0-9.\-]*\.[a-zA-Z]{2,}$`)

// SplitAndTrim splits a comma-separated string, trims each element, and filters empty elements.
func SplitAndTrim(s string) []string {
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// ValidatePhone validates phone number format: 11-digit number, first digit is 1, second digit is 3-9.
func ValidatePhone(phone string) error {
	if !phoneRegex.MatchString(phone) {
		return exception.InvalidArgsError.With(
			"invalid phone number format: %q, phone number must be 11 digits starting with 1 and second digit 3-9", phone)
	}
	return nil
}

// ValidateEmail validates email address format.
// Rules: total length not exceeding 100 characters; must contain @ and cannot be empty before/after @;
// does not support quoted local part, consecutive dots, or IP address format domains.
func ValidateEmail(email string) error {
	// Length limit
	if len(email) > 100 {
		return exception.InvalidArgsError.With("invalid email format: %q, email length cannot exceed 100 characters", email)
	}
	// Must contain @, and cannot be empty before/after @
	atIdx := strings.Index(email, "@")
	if atIdx <= 0 || atIdx == len(email)-1 {
		return exception.InvalidArgsError.With(
			"invalid email format: %q, must contain @ symbol and cannot be empty before/after @", email)
	}
	localPart := email[:atIdx]
	domain := email[atIdx+1:]
	// Does not support quoted local part
	if strings.HasPrefix(localPart, "\"") {
		return exception.InvalidArgsError.With("invalid email format: %q, quoted local part is not supported", email)
	}
	// Does not support consecutive dots
	if strings.Contains(localPart, "..") {
		return exception.InvalidArgsError.With("invalid email format: %q, consecutive dots are not supported", email)
	}
	// Does not support pure IP address domain
	if strings.HasPrefix(domain, "[") {
		return exception.InvalidArgsError.With("invalid email format: %q, IP address format domain is not supported", email)
	}
	// Basic format validation: local part allows letters, numbers, dots, underscores, hyphens;
	// domain must contain dot and cannot start/end with dot
	if !emailRegex.MatchString(email) {
		return exception.InvalidArgsError.With("invalid email format: %q", email)
	}
	return nil
}
