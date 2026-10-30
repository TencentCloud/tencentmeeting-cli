package enumerate

// RecordCorpShareScope represents the enterprise-level share-scope control
// returned by record-GetRecordPermissionConfigMcp (field: corp_share_scope).
type RecordCorpShareScope int

const (
	RecordCorpShareScopeUnlimited RecordCorpShareScope = 1 // 不限制
	RecordCorpShareScopeDisabled  RecordCorpShareScope = 2 // 企业已关闭分享（禁止修改）
	RecordCorpShareScopeCorpOnly  RecordCorpShareScope = 3 // 仅允许同企业范围
)

var recordCorpShareScopeNames = map[RecordCorpShareScope]string{
	RecordCorpShareScopeUnlimited: "不限制",
	RecordCorpShareScopeDisabled:  "企业已关闭分享",
	RecordCorpShareScopeCorpOnly:  "仅允许同企业范围",
}

// RecordCorpShareScopeName returns the enterprise-level share-scope control name
// for the given id, or "Unknown" for unrecognized ids.
func RecordCorpShareScopeName(id int) string {
	if name, ok := recordCorpShareScopeNames[RecordCorpShareScope(id)]; ok {
		return name
	}
	return "Unknown"
}
