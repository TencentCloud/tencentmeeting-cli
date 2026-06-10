package cmdutil

import (
	"context"
	"encoding/json"
	"net/http"

	"tmeet/internal"
	"tmeet/internal/core/thttp"
	"tmeet/internal/log"
	restProxy "tmeet/internal/proxy/rest-proxy"
)

// ApiCmd defines the api cmd.
const (
	// ApiCmdMeetingCancel apiCmd meeting_cancel
	ApiCmdMeetingCancel = "meeting_cancel"
	// ApiCmdMeetingCreate apiCmd meeting_create
	ApiCmdMeetingCreate = "meeting_create"
	// ApiCmdMeetingGetById apiCmd meeting_get_by_id
	ApiCmdMeetingGetById = "meeting_get_by_id"
	// ApiCmdMeetingGetByCode apiCmd meeting_get_by_code
	ApiCmdMeetingGetByCode = "meeting_get_by_code"
	// ApiCmdMeetingInviteList apiCmd meeting_invite_list
	ApiCmdMeetingInviteList = "meeting_invite_list"
	// ApiCmdMeetingList apiCmd meeting_list
	ApiCmdMeetingList = "meeting_list"
	// ApiCmdMeetingListEnded apiCmd meeting_list_ended
	ApiCmdMeetingListEnded = "meeting_list_ended"
	// ApiCmdMeetingUpdate apiCmd meeting_update
	ApiCmdMeetingUpdate = "meeting_update"

	// ApiCmdRecordAddress apiCmd record_address
	ApiCmdRecordAddress = "record_address"
	// ApiCmdRecordList apiCmd record_list
	ApiCmdRecordList = "record_list"
	// ApiCmdRecordSmartMinutes apiCmd record_smart_minutes
	ApiCmdRecordSmartMinutes = "record_smart_minutes"
	// ApiCmdRecordTranscriptGet apiCmd record_transcript_get
	ApiCmdRecordTranscriptGet = "record_transcript_get"
	// ApiCmdRecordTranscriptParagraphs apiCmd record_transcript_paragraphs
	ApiCmdRecordTranscriptParagraphs = "record_transcript_paragraphs"
	// ApiCmdRecordTranscriptSearch apiCmd record_transcript_search
	ApiCmdRecordTranscriptSearch = "record_transcript_search"
	// ApiCmdRecordPermissionApplyPrepare apiCmd record_permission_apply_prepare
	ApiCmdRecordPermissionApplyPrepare = "record_permission_apply_prepare"
	// ApiCmdRecordPermissionApplyCommit apiCmd record_permission_apply_commit
	ApiCmdRecordPermissionApplyCommit = "record_permission_apply_commit"

	// ApiCmdReportParticipants apiCmd report_participants
	ApiCmdReportParticipants = "report_participants"
	// ApiCmdReportWaitingRoomLog apiCmd report_waiting_room_log
	ApiCmdReportWaitingRoomLog = "report_waiting_room_log"
)

// APISchema defines the api schema.
type APISchema struct {
	CompactFields []string              `json:"compact_fields,omitempty"` // compact fields
	CompactSchema []*APICompactSchema   `json:"compact_schema,omitempty"` // compact schema
	CacheConfig   *APISchemaCacheConfig `json:"cache_config,omitempty"`   // cache config
}

// APICompactSchema defines the api compact schema.
type APICompactSchema struct {
	FieldName    string `json:"field_name,omitempty"`    // file name
	FieldType    string `json:"field_type,omitempty"`    // file type
	FieldDesc    string `json:"field_desc,omitempty"`    // file desc
	IsRequired   bool   `json:"is_required,omitempty"`   // is required
	DefaultValue string `json:"default_value,omitempty"` // default value
}

// APISchemaCacheConfig defines the api schema cache config.
type APISchemaCacheConfig struct {
	TTL    int32 `json:"ttl,omitempty"`    // ttl
	Switch bool  `json:"switch,omitempty"` // switch
}

// GetAPISchema gets the api schema.
//
// The result is cached on local disk under <config-dir>/cache/schema/ when
// the server-side CacheConfig explicitly enables caching. A fresh hit is
// returned directly without any network I/O; on miss / expiration / a
// corrupted cache file, the request falls back to the remote endpoint. Any
// cache-layer failure (read error, write error, lock contention, etc.) is
// logged but never surfaces to the caller: the caller's contract is still
// "return a valid APISchema or a concrete error from the remote call".
func GetAPISchema(cxt context.Context, apiCmd string, tmeet *internal.Tmeet) (*APISchema, error) {
	if schema, ok := loadSchemaCache(cxt, apiCmd); ok {
		return schema, nil
	}

	schema, err := fetchAndCacheSchema(cxt, apiCmd, tmeet)
	if err != nil {
		return nil, err
	}
	return schema, nil
}

// fetchSchemaFromRemote issues the actual HTTP call that backs
// GetAPISchema. It deliberately contains zero cache logic so that callers
// which need a guaranteed remote refresh (e.g. unit tests, future admin
// commands) can target it directly without disturbing the cache layer.
func fetchSchemaFromRemote(ctx context.Context, apiCmd string, tmeet *internal.Tmeet) (*APISchema, error) {
	queryParams := thttp.QueryParams{}
	queryParams.Set("operator_id", tmeet.UserConfig.OpenId)
	queryParams.Set("operator_id_type", "2") // OpenId
	queryParams.Set("cmd", apiCmd)
	queryParams.Set("source", "CLI") // fixed

	req := &thttp.Request{
		ApiURI:      "/v1/api/compact-schema",
		QueryParams: queryParams,
	}
	rsp, err := restProxy.RequestProxy(ctx, http.MethodGet, tmeet, req)
	if err != nil {
		return nil, err
	}

	apiSchema := &APISchema{}
	err = json.Unmarshal([]byte(rsp.Data), apiSchema)
	if err != nil {
		log.Errorf(ctx, "unmarshal api schema failed, err: %v", err)
		return nil, err
	}
	return apiSchema, nil
}
