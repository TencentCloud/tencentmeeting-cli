package enumerate

// RecordExpiredTimeType represents the recording share link expiration type
// used by the record-SetRecordPermissionConfigMcp API.
type RecordExpiredTimeType int

const (
	RecordExpiredTimeTypeForever RecordExpiredTimeType = 0 // 永久有效
	RecordExpiredTimeType7Days   RecordExpiredTimeType = 1 // 7 天
	RecordExpiredTimeType15Days  RecordExpiredTimeType = 2 // 15 天
	RecordExpiredTimeType30Days  RecordExpiredTimeType = 3 // 30 天
	RecordExpiredTimeTypeCustom  RecordExpiredTimeType = 4 // 自定义（须同时传 expired_time）
)

var recordExpiredTimeTypeNames = map[RecordExpiredTimeType]string{
	RecordExpiredTimeTypeForever: "永久有效",
	RecordExpiredTimeTypeCustom:  "自定义",
	RecordExpiredTimeType7Days:   "7天",
	RecordExpiredTimeType15Days:  "15天",
	RecordExpiredTimeType30Days:  "30天",
}

// RecordExpiredTimeTypeName returns the recording share link expiration type name for the given id,
// or "Unknown" for unrecognized ids.
func RecordExpiredTimeTypeName(id int) string {
	if name, ok := recordExpiredTimeTypeNames[RecordExpiredTimeType(id)]; ok {
		return name
	}
	return "Unknown"
}
