package enumerate

// MeetingStatus represents the meeting status.
type MeetingStatus string

const (
	MeetingStatusInvalid   MeetingStatus = "MEETING_STATE_INVALID"   // 非法或未知的会议状态，错误状态
	MeetingStatusInit      MeetingStatus = "MEETING_STATE_INIT"      // 待开始，会议预定到预定结束时间前，会议中无人
	MeetingStatusCancelled MeetingStatus = "MEETING_STATE_CANCELLED" // 已取消，主持人主动取消会议，待开始的会议才能取消，取消的会议无法再进入
	MeetingStatusStarted   MeetingStatus = "MEETING_STATE_STARTED"   // 会议中，只要会议中有人即表示进行中
	MeetingStatusEnded     MeetingStatus = "MEETING_STATE_ENDED"     // 已删除，结束时间后且会议中无人时，被主持人删除，已删除的会议无法再进入
	MeetingStatusNull      MeetingStatus = "MEETING_STATE_NULL"      // 无状态，过了预定结束时间，会议中无人
	MeetingStatusRecycled  MeetingStatus = "MEETING_STATE_RECYCLED"  // 已回收，过了预定开始时间30天，会议号被后台回收，无法再进入
)

var meetingStatusNames = map[MeetingStatus]string{
	MeetingStatusInvalid:   "非法状态",
	MeetingStatusInit:      "待开始",
	MeetingStatusCancelled: "已取消",
	MeetingStatusStarted:   "会议中",
	MeetingStatusEnded:     "已删除",
	MeetingStatusNull:      "无状态",
	MeetingStatusRecycled:  "已回收",
}

// MeetingStatusName returns the meeting status name for the given status value, or "Unknown" for unrecognized status.
func MeetingStatusName(s string) string {
	if name, ok := meetingStatusNames[MeetingStatus(s)]; ok {
		return name
	}
	return "Unknown"
}
