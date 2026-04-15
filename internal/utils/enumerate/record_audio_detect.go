package enumerate

// RecordAudioDetect represents the audio detect (voiceprint recognition) status.
// 声纹识别状态
type RecordAudioDetect int

const (
	RecordAudioDetectIncomplete RecordAudioDetect = 0 // 未完成
	RecordAudioDetectComplete   RecordAudioDetect = 1 // 已完成
)

var recordAudioDetectNames = map[RecordAudioDetect]string{
	RecordAudioDetectIncomplete: "未完成",
	RecordAudioDetectComplete:   "已完成",
}

// RecordAudioDetectName returns the audio detect status name for the given value, or "Unknown" for unrecognized values.
func RecordAudioDetectName(t int) string {
	if name, ok := recordAudioDetectNames[RecordAudioDetect(t)]; ok {
		return name
	}
	return "Unknown"
}
