package enumerate

// MeetingRecurringUntilType represents the until type of a recurring meeting.
type MeetingRecurringUntilType int

const (
	MeetingRecurringUntilTypeDate  MeetingRecurringUntilType = 0 // 按日期结束重复
	MeetingRecurringUntilTypeCount MeetingRecurringUntilType = 1 // 按次数结束重复
)

var meetingRecurringUntilTypeNames = map[MeetingRecurringUntilType]string{
	MeetingRecurringUntilTypeDate:  "按日期结束重复",
	MeetingRecurringUntilTypeCount: "按次数结束重复",
}

// MeetingRecurringUntilTypeName returns the until type name for the given type value, or "Unknown" for unrecognized types.
func MeetingRecurringUntilTypeName(t int) string {
	if name, ok := meetingRecurringUntilTypeNames[MeetingRecurringUntilType(t)]; ok {
		return name
	}
	return "Unknown"
}
