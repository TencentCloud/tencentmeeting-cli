package enumerate

// RecordParticipantPerm represents the viewing permission tier granted to
// meeting participants and invitees for a recording. It is the enum used by
// record-{Get,Set}ParticipantPermissionMcp's `participant_perm` field.
type RecordParticipantPerm int

const (
	// RecordParticipantPermAttendedTranscript allows participants to view only
	// the transcript of the period they attended.
	RecordParticipantPermAttendedTranscript RecordParticipantPerm = 1
	// RecordParticipantPermFullTranscriptAndMinutes allows participants to view
	// the whole-meeting transcript and smart minutes.
	RecordParticipantPermFullTranscriptAndMinutes RecordParticipantPerm = 2
	// RecordParticipantPermNeedExplicitShare requires an explicit share via the
	// authorized-member whitelist or share link; no extra grant is applied.
	RecordParticipantPermNeedExplicitShare RecordParticipantPerm = 3
	// RecordParticipantPermFullReview allows participants to view all recap
	// content of the whole meeting (audio/video, minutes, todos, transcript,
	// timeline).
	RecordParticipantPermFullReview RecordParticipantPerm = 4
	// RecordParticipantPermMinutesOnly allows participants to view the
	// whole-meeting smart minutes only, without the transcript.
	RecordParticipantPermMinutesOnly RecordParticipantPerm = 5
)

var recordParticipantPermNames = map[RecordParticipantPerm]string{
	RecordParticipantPermAttendedTranscript:       "可查看参会期间的逐字稿",
	RecordParticipantPermFullTranscriptAndMinutes: "可查看整场会议的逐字稿和纪要",
	RecordParticipantPermNeedExplicitShare:        "需指定分享或者链接分享",
	RecordParticipantPermFullReview:               "可查看全部回顾内容",
	RecordParticipantPermMinutesOnly:              "可查看整场会议的纪要",
}

// RecordParticipantPermName returns the human-readable name for the given
// participant permission tier value, or "Unknown" for unrecognized values.
func RecordParticipantPermName(t int) string {
	if name, ok := recordParticipantPermNames[RecordParticipantPerm(t)]; ok {
		return name
	}
	return "Unknown"
}
