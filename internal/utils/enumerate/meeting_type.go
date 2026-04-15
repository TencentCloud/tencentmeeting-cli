package enumerate

// MeetingType represents the meeting type.
type MeetingType int

const (
	MeetingTypeNormal    MeetingType = 0 // 普通会议
	MeetingTypeRecurring MeetingType = 1 // 周期性会议
	MeetingTypeWeChat    MeetingType = 2 // 微信专属会议
	MeetingTypeRooms     MeetingType = 4 // Rooms 投屏会议
	MeetingTypePersonal  MeetingType = 5 // 个人会议号会议
	MeetingTypeWebinar   MeetingType = 6 // 网络研讨会
)

var meetingTypeNames = map[MeetingType]string{
	MeetingTypeNormal:    "普通会议",
	MeetingTypeRecurring: "周期性会议",
	MeetingTypeWeChat:    "微信专属会议",
	MeetingTypeRooms:     "Rooms 投屏会议",
	MeetingTypePersonal:  "个人会议号会议",
	MeetingTypeWebinar:   "网络研讨会",
}

// MeetingTypeName returns the meeting type name for the given type value, or "Unknown" for unrecognized types.
func MeetingTypeName(t int) string {
	if name, ok := meetingTypeNames[MeetingType(t)]; ok {
		return name
	}
	return "Unknown"
}
