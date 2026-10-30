package enumerate

// RecordMemberPermission represents the permission level of a member granted
// to a recording, returned by the record-GetGrantedUserList (access members) API.
type RecordMemberPermission int

const (
	RecordMemberPermissionView   RecordMemberPermission = 0 // 可查看
	RecordMemberPermissionManage RecordMemberPermission = 1 // 可管理
)

var recordMemberPermissionNames = map[RecordMemberPermission]string{
	RecordMemberPermissionView:   "可查看",
	RecordMemberPermissionManage: "可管理",
}

// RecordMemberPermissionName returns the recording member permission name
// for the given id, or "Unknown" for unrecognized ids.
func RecordMemberPermissionName(id int) string {
	if name, ok := recordMemberPermissionNames[RecordMemberPermission(id)]; ok {
		return name
	}
	return "Unknown"
}
