package enumerate

// MeetingStatusInSearch represents the meeting status used in search results.
type MeetingStatusInSearch string

const (
	MeetingStatusInSearchUnknown    MeetingStatusInSearch = "unknown"     // 未知状态
	MeetingStatusInSearchUpcoming   MeetingStatusInSearch = "upcoming"    // 待开始
	MeetingStatusInSearchInProgress MeetingStatusInSearch = "in_progress" // 进行中
	MeetingStatusInSearchClosed     MeetingStatusInSearch = "closed"      // 已结束
)

var meetingStatusInSearchNames = map[MeetingStatusInSearch]string{
	MeetingStatusInSearchUnknown:    "未知状态",
	MeetingStatusInSearchUpcoming:   "待开始",
	MeetingStatusInSearchInProgress: "进行中",
	MeetingStatusInSearchClosed:     "已结束",
}

// MeetingStatusInSearchName returns the meeting status name for the given status value, or "Unknown" for unrecognized status.
func MeetingStatusInSearchName(s string) string {
	if name, ok := meetingStatusInSearchNames[MeetingStatusInSearch(s)]; ok {
		return name
	}
	return "Unknown"
}
