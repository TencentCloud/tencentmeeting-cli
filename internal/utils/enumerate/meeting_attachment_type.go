package enumerate

// MeetingAttachmentType represents the source type of a meeting attachment.
// 附件来源类型
type MeetingAttachmentType int

const (
	MeetingAttachmentTypeTencentDocs MeetingAttachmentType = 1 // 腾讯文档
	MeetingAttachmentTypeLocalUpload MeetingAttachmentType = 2 // 本地上传文件
)

var meetingAttachmentTypeNames = map[MeetingAttachmentType]string{
	MeetingAttachmentTypeTencentDocs: "腾讯文档",
	MeetingAttachmentTypeLocalUpload: "本地上传文件",
}

// MeetingAttachmentTypeName returns the attachment source name for the given value,
// or "Unknown" for unrecognized values.
func MeetingAttachmentTypeName(t int) string {
	if name, ok := meetingAttachmentTypeNames[MeetingAttachmentType(t)]; ok {
		return name
	}
	return "Unknown"
}
