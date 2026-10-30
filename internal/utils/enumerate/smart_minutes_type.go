package enumerate

// SmartMinutesMinuteType represents the meeting minute return category.
type SmartMinutesMinuteType int

const (
	SmartMinutesMinuteTypeChapter SmartMinutesMinuteType = 1 // 按章节
	SmartMinutesMinuteTypeTopic   SmartMinutesMinuteType = 2 // 按主题
	SmartMinutesMinuteTypeSpeaker SmartMinutesMinuteType = 3 // 按发言人
)

var smartMinutesMinuteTypeNames = map[SmartMinutesMinuteType]string{
	SmartMinutesMinuteTypeChapter: "按章节",
	SmartMinutesMinuteTypeTopic:   "按主题",
	SmartMinutesMinuteTypeSpeaker: "按发言人",
}

// SmartMinutesMinuteTypeName returns the minute type name for the given value, or "Unknown" for unrecognized values.
func SmartMinutesMinuteTypeName(t int) string {
	if name, ok := smartMinutesMinuteTypeNames[SmartMinutesMinuteType(t)]; ok {
		return name
	}
	return "Unknown"
}

// SmartMinutesTextType represents the returned text type.
type SmartMinutesTextType int

const (
	SmartMinutesTextTypePlain    SmartMinutesTextType = 1 // 纯文本
	SmartMinutesTextTypeMarkdown SmartMinutesTextType = 2 // markdown
)

var smartMinutesTextTypeNames = map[SmartMinutesTextType]string{
	SmartMinutesTextTypePlain:    "纯文本",
	SmartMinutesTextTypeMarkdown: "markdown",
}

// SmartMinutesTextTypeName returns the text type name for the given value, or "Unknown" for unrecognized values.
func SmartMinutesTextTypeName(t int) string {
	if name, ok := smartMinutesTextTypeNames[SmartMinutesTextType(t)]; ok {
		return name
	}
	return "Unknown"
}

// SmartMinutesIndustry represents the industry model.
type SmartMinutesIndustry int

const (
	SmartMinutesIndustryGeneral   SmartMinutesIndustry = 0 // 通用模型
	SmartMinutesIndustryFinance   SmartMinutesIndustry = 1 // 金融模型
	SmartMinutesIndustryInterview SmartMinutesIndustry = 2 // 面试
	SmartMinutesIndustryEducation SmartMinutesIndustry = 3 // 教育
)

var smartMinutesIndustryNames = map[SmartMinutesIndustry]string{
	SmartMinutesIndustryGeneral:   "通用模型",
	SmartMinutesIndustryFinance:   "金融模型",
	SmartMinutesIndustryInterview: "面试",
	SmartMinutesIndustryEducation: "教育",
}

// SmartMinutesIndustryName returns the industry name for the given value, or "Unknown" for unrecognized values.
func SmartMinutesIndustryName(t int) string {
	if name, ok := smartMinutesIndustryNames[SmartMinutesIndustry(t)]; ok {
		return name
	}
	return "Unknown"
}

// SmartMinutesLLM represents the LLM model selection.
type SmartMinutesLLM int

const (
	SmartMinutesLLMHunyuan  SmartMinutesLLM = 1 // 混元
	SmartMinutesLLMDeepSeek SmartMinutesLLM = 2 // deepseek
)

var smartMinutesLLMNames = map[SmartMinutesLLM]string{
	SmartMinutesLLMHunyuan:  "混元",
	SmartMinutesLLMDeepSeek: "deepseek",
}

// SmartMinutesLLMName returns the LLM name for the given value, or "Unknown" for unrecognized values.
func SmartMinutesLLMName(t int) string {
	if name, ok := smartMinutesLLMNames[SmartMinutesLLM(t)]; ok {
		return name
	}
	return "Unknown"
}
