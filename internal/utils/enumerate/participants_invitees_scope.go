package enumerate

// ParticipantsInviteesScope represents the viewing scope granted to participants
// and invitees after the meeting, returned by record-GetRecordPermissionConfigMcp
// (field: participants_invitees_scope).
type ParticipantsInviteesScope int

const (
	ParticipantsInviteesScopeAttendedTranscript ParticipantsInviteesScope = 0 // 可查看参会期间的逐字稿
	ParticipantsInviteesScopeFullTranscript     ParticipantsInviteesScope = 1 // 可查看整场会议的逐字稿
	ParticipantsInviteesScopeFullReview         ParticipantsInviteesScope = 2 // 可查看整场会议回顾
)

var participantsInviteesScopeNames = map[ParticipantsInviteesScope]string{
	ParticipantsInviteesScopeAttendedTranscript: "可查看参会期间的逐字稿",
	ParticipantsInviteesScopeFullTranscript:     "可查看整场会议的逐字稿",
	ParticipantsInviteesScopeFullReview:         "可查看整场会议回顾",
}

// ParticipantsInviteesScopeName returns the participants-invitees scope name for
// the given id, or an empty string for unrecognized ids.
func ParticipantsInviteesScopeName(id int) string {
	if name, ok := participantsInviteesScopeNames[ParticipantsInviteesScope(id)]; ok {
		return name
	}
	return ""
}
