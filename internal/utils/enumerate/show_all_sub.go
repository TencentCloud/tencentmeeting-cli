package enumerate

// ShowAllSubMeetings represents the flag of whether to show all sub meetings.
type ShowAllSubMeetings int

const (
	ShowAllSubMeetingsNo  ShowAllSubMeetings = 0 // 不展示
	ShowAllSubMeetingsYes ShowAllSubMeetings = 1 // 展示
)

var showAllSubMeetingsNames = map[ShowAllSubMeetings]string{
	ShowAllSubMeetingsNo:  "no",
	ShowAllSubMeetingsYes: "yes",
}

// ShowAllSubMeetingsName returns the human-readable name for the given show all sub meetings flag, or "unknown" for unrecognized values.
func ShowAllSubMeetingsName(flag int) string {
	if name, ok := showAllSubMeetingsNames[ShowAllSubMeetings(flag)]; ok {
		return name
	}
	return "unknown"
}
