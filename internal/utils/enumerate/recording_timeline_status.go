package enumerate

// RecordingTimelineStatus represents the recording timeline generation status.
type RecordingTimelineStatus int

const (
	RecordingTimelineStatusNotGenerated RecordingTimelineStatus = 0 // not generated
	RecordingTimelineStatusGenerating   RecordingTimelineStatus = 1 // generating
	RecordingTimelineStatusGenerated    RecordingTimelineStatus = 2 // generated
)

var recordingTimelineStatusNames = map[RecordingTimelineStatus]string{
	RecordingTimelineStatusNotGenerated: "not generated",
	RecordingTimelineStatusGenerating:   "generating",
	RecordingTimelineStatusGenerated:    "generated",
}

// RecordingTimelineStatusName returns the human-readable name for the given
// timeline generation status value, or "Unknown" for unrecognized values.
func RecordingTimelineStatusName(s int) string {
	if name, ok := recordingTimelineStatusNames[RecordingTimelineStatus(s)]; ok {
		return name
	}
	return "Unknown"
}
