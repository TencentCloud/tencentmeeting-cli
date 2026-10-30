package enumerate

// TranscriptType represents the transcript type.
type TranscriptType int

const (
	TranscriptTypeOriginal TranscriptType = 0 // 原文版
	TranscriptTypeSmart    TranscriptType = 1 // 智能优化版
)

var transcriptTypeNames = map[TranscriptType]string{
	TranscriptTypeOriginal: "原文版",
	TranscriptTypeSmart:    "智能优化版",
}

// TranscriptTypeName returns the transcript type name for the given type value, or "Unknown" for unrecognized types.
func TranscriptTypeName(t int) string {
	if name, ok := transcriptTypeNames[TranscriptType(t)]; ok {
		return name
	}
	return "Unknown"
}
