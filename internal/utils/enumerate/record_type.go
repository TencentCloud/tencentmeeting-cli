package enumerate

// RecordType represents the recording type.
type RecordType int

const (
	RecordTypeCloud      RecordType = 0 // 云录制
	RecordTypeUpload     RecordType = 2 // 上传录制
	RecordTypeTranscript RecordType = 3 // 文字转写
	RecordTypeVideo      RecordType = 4 // 视频录制
	RecordTypeAudio      RecordType = 5 // 音频录制
)

var recordTypeNames = map[RecordType]string{
	RecordTypeCloud:      "云录制",
	RecordTypeUpload:     "上传录制",
	RecordTypeTranscript: "文字转写",
	RecordTypeVideo:      "视频录制",
	RecordTypeAudio:      "音频录制",
}

// RecordTypeName returns the recording type name for the given type value, or "Unknown" for unrecognized types.
func RecordTypeName(t int) string {
	if name, ok := recordTypeNames[RecordType(t)]; ok {
		return name
	}
	return "Unknown"
}
