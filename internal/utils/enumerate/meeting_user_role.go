package enumerate

// MeetingUserRole represents the user's role in a meeting.
type MeetingUserRole int

const (
	MeetingUserRoleMember           MeetingUserRole = 0 // 普通成员角色
	MeetingUserRoleCreator          MeetingUserRole = 1 // 创建者角色
	MeetingUserRoleHost             MeetingUserRole = 2 // 主持人
	MeetingUserRoleCreatorAndHost   MeetingUserRole = 3 // 创建者+主持人
	MeetingUserRoleGuest            MeetingUserRole = 4 // 游客
	MeetingUserRoleGuestAndHost     MeetingUserRole = 5 // 游客+主持人
	MeetingUserRoleCoHost           MeetingUserRole = 6 // 联席主持人
	MeetingUserRoleCreatorAndCoHost MeetingUserRole = 7 // 创建者+联席主持人
)

var meetingUserRoleNames = map[MeetingUserRole]string{
	MeetingUserRoleMember:           "普通成员角色",
	MeetingUserRoleCreator:          "创建者角色",
	MeetingUserRoleHost:             "主持人",
	MeetingUserRoleCreatorAndHost:   "创建者+主持人",
	MeetingUserRoleGuest:            "游客",
	MeetingUserRoleGuestAndHost:     "游客+主持人",
	MeetingUserRoleCoHost:           "联席主持人",
	MeetingUserRoleCreatorAndCoHost: "创建者+联席主持人",
}

// MeetingUserRoleName returns the user role name for the given role value, or "Unknown" for unrecognized roles.
func MeetingUserRoleName(r int) string {
	if name, ok := meetingUserRoleNames[MeetingUserRole(r)]; ok {
		return name
	}
	return "Unknown"
}
