package enumerate

// RecordSettingType represents a generic on/off switch value used by
// record-related settings (e.g. subtitle switch, smart recording switch).
// 录制设置开关状态
type RecordSettingType int

const (
	RecordSettingTypeOff RecordSettingType = 1 // 关闭
	RecordSettingTypeOn  RecordSettingType = 2 // 开启
)

var recordSettingTypeNames = map[RecordSettingType]string{
	RecordSettingTypeOff: "关闭",
	RecordSettingTypeOn:  "开启",
}

// RecordSettingTypeName returns the setting name for the given value, or "Unknown" for unrecognized values.
func RecordSettingTypeName(t int) string {
	if name, ok := recordSettingTypeNames[RecordSettingType(t)]; ok {
		return name
	}
	return "Unknown"
}
