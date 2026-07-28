package cmdutil

import "strings"

// FormatMeetingCode formats the meetingCode.
// Raw inputs:
// 1. #腾讯会议：295-150-176
// 2. 295-150-176
// 3. 295 150 176
// 4. 295150176
// After conversion:
// 1. 295150176
func FormatMeetingCode(meetingCode string) string {
	// Trim leading/trailing whitespace first to avoid TrimPrefix failing
	// when the prefix is preceded by spaces.
	meetingCode = strings.TrimSpace(meetingCode)
	// Strip the "#腾讯会议：" prefix (also accepts the half-width colon).
	meetingCode = strings.TrimPrefix(meetingCode, "#腾讯会议：")
	meetingCode = strings.TrimPrefix(meetingCode, "#腾讯会议:")
	// Remove hyphens, spaces, tabs and single quotes.
	replacer := strings.NewReplacer("-", "", " ", "", "\t", "", "'", "")
	return strings.TrimSpace(replacer.Replace(meetingCode))
}
