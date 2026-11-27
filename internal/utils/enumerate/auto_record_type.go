package enumerate

// AutoRecordType represents the auto-record type accepted by
// --auto-record-type. The value is forwarded to settings.auto_record_type
// unchanged.
type AutoRecordType string

const (
	AutoRecordTypeNone  AutoRecordType = "none"  // 关闭
	AutoRecordTypeCloud AutoRecordType = "cloud" // 云录制
)

var autoRecordTypeNames = map[AutoRecordType]string{
	AutoRecordTypeNone:  "关闭",
	AutoRecordTypeCloud: "云录制",
}

// AutoRecordTypeName returns the name for the given value, or "Unknown" for
// unrecognized values.
func AutoRecordTypeName(v string) string {
	if name, ok := autoRecordTypeNames[AutoRecordType(v)]; ok {
		return name
	}
	return "Unknown"
}

// AutoRecordTypeValues returns every valid value as strings, in a stable
// order. It feeds the Allowed list of cmdutil.EnumValue so the valid values
// are defined in one place.
func AutoRecordTypeValues() []string {
	return []string{string(AutoRecordTypeNone), string(AutoRecordTypeCloud)}
}
