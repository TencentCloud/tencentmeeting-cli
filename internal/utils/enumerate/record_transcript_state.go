package enumerate

type RecordTranscriptState int

const (
	RecordTranscriptStateNotGenerated RecordTranscriptState = 0
	RecordTranscriptStateGenerating   RecordTranscriptState = 1
	RecordTranscriptStateCompleted    RecordTranscriptState = 2
)

var recordTranscriptStateNames = map[RecordTranscriptState]string{
	RecordTranscriptStateNotGenerated: "未生成逐字稿",
	RecordTranscriptStateGenerating:   "逐字稿生成中",
	RecordTranscriptStateCompleted:    "逐字稿生成完成",
}

// RecordTranscriptStateName returns the transcript state name for the given state value, or "Unknown" for unrecognized states.
func RecordTranscriptStateName(s int) string {
	if name, ok := recordTranscriptStateNames[RecordTranscriptState(s)]; ok {
		return name
	}
	return "Unknown"
}
