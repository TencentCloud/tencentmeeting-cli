package enumerate

// RecordPermissionPoint represents the recording permission-point ID
// used by the record-SetRecordPermissionConfigMcp / record-GetRecordPermissionConfigMcp APIs.
type RecordPermissionPoint int

const (
	RecordPermissionPointPublicView     RecordPermissionPoint = 1 // 所有人可查看
	RecordPermissionPointPublicPassword RecordPermissionPoint = 2 // 所有人需输入密码查看
	RecordPermissionPointPublicApprove  RecordPermissionPoint = 3 // 所有人需申请权限查看
	RecordPermissionPointCorpView       RecordPermissionPoint = 4 // 同企业可查看
	RecordPermissionPointCorpPassword   RecordPermissionPoint = 5 // 同企业需输入密码查看
	RecordPermissionPointCorpApprove    RecordPermissionPoint = 6 // 同企业需申请权限查看
)

var recordPermissionPointNames = map[RecordPermissionPoint]string{
	RecordPermissionPointPublicView:     "所有人可查看",
	RecordPermissionPointPublicPassword: "所有人需输入密码查看",
	RecordPermissionPointPublicApprove:  "所有人需申请权限查看",
	RecordPermissionPointCorpView:       "同企业可查看",
	RecordPermissionPointCorpPassword:   "同企业需输入密码查看",
	RecordPermissionPointCorpApprove:    "同企业需申请权限查看",
}

// RecordPermissionPointName returns the recording permission-point name for the given id,
// or "Unknown" for unrecognized ids.
func RecordPermissionPointName(id int) string {
	if name, ok := recordPermissionPointNames[RecordPermissionPoint(id)]; ok {
		return name
	}
	return "Unknown"
}
