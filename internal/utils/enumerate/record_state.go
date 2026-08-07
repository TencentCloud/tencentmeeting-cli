package enumerate

// RecordState represents the recording state.
type RecordState int

const (
	RecordStateRecording   RecordState = 1 // 录制中
	RecordStateTranscoding RecordState = 2 // 转码中
	RecordStateDone        RecordState = 3 // 转码完成
)

var recordStateNames = map[RecordState]string{
	RecordStateRecording:   "录制中，不可查看或申请",
	RecordStateTranscoding: "转码中，不可查看或申请",
	RecordStateDone:        "转码完成，可根据录制文件权限进行下一步",
}

// RecordStateName returns the recording state name for the given state value, or "Unknown" for unrecognized states.
func RecordStateName(s int) string {
	if name, ok := recordStateNames[RecordState(s)]; ok {
		return name
	}
	return "Unknown"
}
