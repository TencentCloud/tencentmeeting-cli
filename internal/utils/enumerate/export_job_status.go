package enumerate

// ExportJobStatus represents the export job status.
type ExportJobStatus int

const (
	ExportJobStatusSuccess    ExportJobStatus = 1 // 成功
	ExportJobStatusFailed     ExportJobStatus = 2 // 失败
	ExportJobStatusProcessing ExportJobStatus = 3 // 处理中
)

var exportJobStatusNames = map[ExportJobStatus]string{
	ExportJobStatusSuccess:    "成功",
	ExportJobStatusFailed:     "失败",
	ExportJobStatusProcessing: "处理中",
}

// ExportJobStatusName returns the export job status name for the given status value, or "Unknown" for unrecognized statuses.
func ExportJobStatusName(s int) string {
	if name, ok := exportJobStatusNames[ExportJobStatus(s)]; ok {
		return name
	}
	return "Unknown"
}
