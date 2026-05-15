package enumerate

// MeetingUserJoinRole represents the user's join role in a meeting.
type MeetingUserJoinRole string

const (
	MeetingUserJoinRoleCreator MeetingUserJoinRole = "creator" // 创建者
	MeetingUserJoinRoleHoster  MeetingUserJoinRole = "hoster"  // 主持人
	MeetingUserJoinRoleInvitee MeetingUserJoinRole = "invitee" // 被邀请者
)

var meetingUserJoinRoleNames = map[MeetingUserJoinRole]string{
	MeetingUserJoinRoleCreator: "创建者",
	MeetingUserJoinRoleHoster:  "主持人",
	MeetingUserJoinRoleInvitee: "被邀请者",
}

// MeetingUserJoinRoleName returns the join role name for the given role value, or "Unknown" for unrecognized roles.
func MeetingUserJoinRoleName(r string) string {
	if name, ok := meetingUserJoinRoleNames[MeetingUserJoinRole(r)]; ok {
		return name
	}
	return "Unknown"
}
