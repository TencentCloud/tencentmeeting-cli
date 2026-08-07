package enumerate

// WaitingRoomOperateType represents the host operation type on waiting room members.
type WaitingRoomOperateType int

const (
	WaitingRoomOperateEnterMeeting  WaitingRoomOperateType = 1 // enter-meeting: admit waiting room members into the meeting
	WaitingRoomOperateBackToWaiting WaitingRoomOperateType = 2 // back-to-waiting: move in-meeting members back to the waiting room
	WaitingRoomOperateExpel         WaitingRoomOperateType = 3 // expel: expel waiting room members from the meeting
)

// waitingRoomOperateTypeNames maps WaitingRoomOperateType to its CLI flag name (also used as display name).
var waitingRoomOperateTypeNames = map[WaitingRoomOperateType]string{
	WaitingRoomOperateEnterMeeting:  "enter-meeting",
	WaitingRoomOperateBackToWaiting: "back-to-waiting",
	WaitingRoomOperateExpel:         "expel",
}

// waitingRoomOperateTypeValues is the reverse lookup table built from waitingRoomOperateTypeNames.
var waitingRoomOperateTypeValues = func() map[string]WaitingRoomOperateType {
	m := make(map[string]WaitingRoomOperateType, len(waitingRoomOperateTypeNames))
	for k, v := range waitingRoomOperateTypeNames {
		m[v] = k
	}
	return m
}()

// WaitingRoomOperateTypeName returns the operate type name for the given numeric value, or "Unknown" for unrecognized types.
func WaitingRoomOperateTypeName(t int) string {
	if name, ok := waitingRoomOperateTypeNames[WaitingRoomOperateType(t)]; ok {
		return name
	}
	return "Unknown"
}

// WaitingRoomOperateTypeValue resolves a CLI flag string (e.g. "enter-meeting") to its
// downstream API numeric enum. The second return value indicates whether the name is recognized.
func WaitingRoomOperateTypeValue(name string) (WaitingRoomOperateType, bool) {
	v, ok := waitingRoomOperateTypeValues[name]
	return v, ok
}
