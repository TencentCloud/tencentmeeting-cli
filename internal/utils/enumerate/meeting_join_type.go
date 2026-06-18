package enumerate

// MeetingJoinType represents the join restriction type of a meeting.
type MeetingJoinType int

const (
	MeetingJoinTypeAll      MeetingJoinType = 1 // 所有人可加入
	MeetingJoinTypeInvited  MeetingJoinType = 2 // 仅受邀者可加入
	MeetingJoinTypeInternal MeetingJoinType = 3 // 仅企业内用户可加入
)

var meetingJoinTypeNames = map[MeetingJoinType]string{
	MeetingJoinTypeAll:      "all",
	MeetingJoinTypeInvited:  "invited",
	MeetingJoinTypeInternal: "internal",
}

// MeetingJoinTypeName returns the human-readable name for the given meeting join type, or "unknown" for unrecognized types.
func MeetingJoinTypeName(joinType int) string {
	if name, ok := meetingJoinTypeNames[MeetingJoinType(joinType)]; ok {
		return name
	}
	return "unknown"
}
