package enumerate

// MeetingRecurringType represents the recurring type of a meeting.
type MeetingRecurringType int

const (
	MeetingRecurringTypeDaily    MeetingRecurringType = 0 // 每天
	MeetingRecurringTypeWeekdays MeetingRecurringType = 1 // 每周一至周五
	MeetingRecurringTypeWeekly   MeetingRecurringType = 2 // 每周
	MeetingRecurringTypeBiweekly MeetingRecurringType = 3 // 每两周
	MeetingRecurringTypeMonthly  MeetingRecurringType = 4 // 每月
)

var meetingRecurringTypeNames = map[MeetingRecurringType]string{
	MeetingRecurringTypeDaily:    "每天",
	MeetingRecurringTypeWeekdays: "每周一至周五",
	MeetingRecurringTypeWeekly:   "每周",
	MeetingRecurringTypeBiweekly: "每两周",
	MeetingRecurringTypeMonthly:  "每月",
}

// MeetingRecurringTypeName returns the recurring type name for the given type value, or "Unknown" for unrecognized types.
func MeetingRecurringTypeName(t int) string {
	if name, ok := meetingRecurringTypeNames[MeetingRecurringType(t)]; ok {
		return name
	}
	return "Unknown"
}
