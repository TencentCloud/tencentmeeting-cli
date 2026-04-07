package utils

import (
	"fmt"
	"time"
	"tmeet/internal/exception"
)

// ISO8601ToTimeStamp converts an ISO8601 formatted time string to a Unix timestamp.
// Example input: 2026-03-12T14:00+08:00 or 2026-03-12T14:00:00+08:00
func ISO8601ToTimeStamp(iso8601 string) (int64, error) {
	// Supported formats (tried in order of priority):
	formats := []string{
		time.RFC3339,             // 2006-01-02T15:04:05Z07:00
		"2006-01-02T15:04Z07:00", // 2026-03-12T14:00+08:00 (no seconds)
	}
	for _, layout := range formats {
		t, err := time.Parse(layout, iso8601)
		if err == nil {
			return t.Unix(), nil
		}
	}
	return 0, exception.InvalidArgsError.With("failed to parse time format: %s, supported formats: 2006-01-02T15:04:05Z07:00 or 2006-01-02T15:04Z07:00", iso8601)
}

// TimeStampToISO8601 converts a Unix timestamp to an ISO8601 formatted time string.
func TimeStampToISO8601(timestamp int64) string {
	if timestamp == 0 {
		return ""
	}
	t := time.Unix(timestamp, 0)
	return t.Format(time.RFC3339)
}

// durationSecondsToHMS is an internal function that converts seconds to a time format string.
// Outputs "MM:SS" if less than one hour, otherwise "HH:MM:SS".
func durationSecondsToHMS(seconds int64) string {
	h := seconds / 3600
	m := (seconds % 3600) / 60
	s := seconds % 60
	if h > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%02d:%02d", m, s)
}

// DurationToHMS converts milliseconds to a time format string.
// Outputs "MM:SS" if less than one hour, otherwise "HH:MM:SS".
func DurationToHMS(milliseconds int64) string {
	return durationSecondsToHMS(milliseconds / 1000)
}

// DurationSecondsToHMS converts seconds to a time format string.
// Outputs "MM:SS" if less than one hour, otherwise "HH:MM:SS".
func DurationSecondsToHMS(seconds int64) string {
	return durationSecondsToHMS(seconds)
}
