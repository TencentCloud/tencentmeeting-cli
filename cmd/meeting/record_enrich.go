package meeting

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"tmeet/internal"
	middleWare "tmeet/internal/cmdutil/middleware"
	"tmeet/internal/core/thttp"
	"tmeet/internal/log"
	restProxy "tmeet/internal/proxy/rest-proxy"
	"tmeet/internal/utils/enumerate"
)

// meetingIDInfo identifies a single meeting for record-basic-info lookup.
// StartTime / EndTime are unix timestamps in seconds. Pointer types are used
// so that a missing value is omitted from the JSON payload rather than sent
// as a misleading zero.
type meetingIDInfo struct {
	MeetingID    string `json:"meeting_id"`
	SubMeetingID string `json:"sub_meeting_id,omitempty"`
	StartTime    *int64 `json:"start_time,omitempty"`
	EndTime      *int64 `json:"end_time,omitempty"`
}

// recordBasicInfo is one element of the meet_record_basic_infos response array.
type recordBasicInfo struct {
	MeetingID         string        `json:"meeting_id"`
	SubMeetingID      string        `json:"sub_meeting_id,omitempty"`
	RecordsTotalCount int           `json:"records_total_count"`
	Records           []interface{} `json:"records"`
}

// meetRecordBasicInfosRsp is the response body of POST /v1/mcp/records/meet-basic-info.
type meetRecordBasicInfosRsp struct {
	MeetRecordBasicInfos []recordBasicInfo `json:"meet_record_basic_infos"`
}

// meetRecordBasicInfoListRsp is the response body of GET /v1/mcp/records/meet-basic-info-list.
// This paginated API returns one page of records for a single meeting at a time.
type meetRecordBasicInfoListRsp struct {
	TotalCount          int             `json:"total_count"`
	HasMore             bool            `json:"has_more"`
	NextPageToken       string          `json:"next_page_token"`
	CurrentSize         int             `json:"current_size"`
	MeetRecordBasicInfo recordBasicInfo `json:"meet_record_basic_info"`
}

// maxFullRecordPages caps the number of pagination requests for the full
// recording snapshot. 2 pages × 100 per page = 200 records max.
const (
	maxFullRecordPages = 2
	fullRecordPageSize = 100
)

// enrichMeetingsWithRecords fetches recording basic info for every meeting in
// data[meetingInfoPath] and merges records_total_count + records into each
// meeting element (at the same level as meeting_id). It is best-effort: on any
// failure the original data is returned unchanged so meeting output is never
// blocked by record query errors.
//
// meetingInfoPath is the top-level JSON key whose value is the meeting list
// (e.g. "meeting_info_list" for list/list-ended; some APIs may use a
// different key, which is why it is passed by the caller).
//
// includeSubMeetingID controls whether sub_meeting_id is carried in the
// request payload. `meeting get` and `meeting list` should pass false (query
// by meeting_id only); `meeting list-ended` should pass true so recurring
// sub-meetings are matched precisely.
//
// Callers using --compact should pass recordEnrichmentFields as extra
// arguments to output.WithCompact so the injected fields survive trimming.
func enrichMeetingsWithRecords(ctx context.Context, tmeet *internal.Tmeet, data []byte, meetingInfoPath string, includeSubMeetingID bool) []byte {
	var meetingData map[string]interface{}
	if err := json.Unmarshal(data, &meetingData); err != nil {
		log.Warnf(ctx, "enrichMeetingsWithRecords: unmarshal meeting data failed, skip enrich: %v", err)
		return data
	}

	meetingInfoList, ok := meetingData[meetingInfoPath].([]interface{})
	if !ok || len(meetingInfoList) == 0 {
		log.Debugf(ctx, "enrichMeetingsWithRecords: %s missing or empty, skip enrich", meetingInfoPath)
		return data
	}

	// Collect meeting ID infos while keeping a reference to each meeting map
	// so we can merge results back without a second lookup pass.
	type meetingRef struct {
		meetingID    string
		subMeetingID string
		obj          map[string]interface{}
	}
	refs := make([]meetingRef, 0, len(meetingInfoList))
	idInfos := make([]meetingIDInfo, 0, len(meetingInfoList))
	for idx, item := range meetingInfoList {
		m, ok := item.(map[string]interface{})
		if !ok {
			log.Warnf(ctx, "enrichMeetingsWithRecords: %s[%d] is not an object, skip", meetingInfoPath, idx)
			continue
		}
		meetingID, _ := m["meeting_id"].(string)
		if meetingID == "" {
			log.Warnf(ctx, "enrichMeetingsWithRecords: %s[%d] missing meeting_id, skip", meetingInfoPath, idx)
			continue
		}
		subMeetingID, _ := m["sub_meeting_id"].(string)
		refs = append(refs, meetingRef{meetingID: meetingID, subMeetingID: subMeetingID, obj: m})
		info := meetingIDInfo{
			MeetingID: meetingID,
			StartTime: parseUnixSeconds(m["start_time"]),
			EndTime:   parseUnixSeconds(m["end_time"]),
		}
		if includeSubMeetingID {
			info.SubMeetingID = subMeetingID
		}
		idInfos = append(idInfos, info)
	}
	if len(idInfos) == 0 {
		log.Debugf(ctx, "enrichMeetingsWithRecords: no valid meeting id collected, skip record query")
		return data
	}

	recordInfos, err := fetchMeetRecordBasicInfos(ctx, tmeet, idInfos)
	if err != nil {
		log.Errorf(ctx, "enrichMeetingsWithRecords: fetch record basic infos failed: %v", err)
		return data
	}

	// Build lookup map keyed by "meeting_id|sub_meeting_id". When the request
	// did not carry sub_meeting_id we key solely on meeting_id, because the
	// API is expected to return one aggregated entry per meeting_id.
	lookup := make(map[string]recordBasicInfo, len(recordInfos))
	for _, ri := range recordInfos {
		if includeSubMeetingID {
			lookup[ri.MeetingID+"|"+ri.SubMeetingID] = ri
		} else {
			lookup[ri.MeetingID] = ri
		}
	}

	// Merge record info into each meeting object.
	var matched, unmatched int
	for _, ref := range refs {
		var key string
		if includeSubMeetingID {
			key = ref.meetingID + "|" + ref.subMeetingID
		} else {
			key = ref.meetingID
		}
		if ri, found := lookup[key]; found {
			ref.obj["records_total_count"] = ri.RecordsTotalCount
			ref.obj["records"] = normalizeRecords(ri.Records)
			matched++
			log.Debugf(ctx, "enrichMeetingsWithRecords: matched meeting_id=%s sub_meeting_id=%s records_total_count=%d",
				ref.meetingID, ref.subMeetingID, ri.RecordsTotalCount)
		} else {
			// Meeting not returned by the records API: treat as no recordings.
			ref.obj["records_total_count"] = 0
			ref.obj["records"] = []interface{}{}
			unmatched++
			log.Debugf(ctx, "enrichMeetingsWithRecords: no record for meeting_id=%s sub_meeting_id=%s",
				ref.meetingID, ref.subMeetingID)
		}
	}

	result, err := json.Marshal(meetingData)
	if err != nil {
		log.Warnf(ctx, "enrichMeetingsWithRecords: marshal enriched data failed, return original: %v", err)
		return data
	}
	return result
}

// fetchMeetRecordBasicInfos calls POST /v1/mcp/records/meet-basic-info to
// retrieve recording basic info for the given meetings.
func fetchMeetRecordBasicInfos(ctx context.Context, tmeet *internal.Tmeet, idInfos []meetingIDInfo) ([]recordBasicInfo, error) {
	body := map[string]interface{}{
		"operator_id":      tmeet.UserConfig.OpenId,
		"operator_id_type": "2", // OpenId
		"meeting_id_infos": idInfos,
	}
	req := &thttp.Request{
		ApiURI: "/v1/mcp/records/meet-basic-info",
		Body:   body,
	}

	rsp, err := restProxy.RequestProxy(ctx, http.MethodPost, tmeet, req)
	if err != nil {
		log.Errorf(ctx, "fetchMeetRecordBasicInfos: request failed: %v", err)
		return nil, err
	}

	var resp meetRecordBasicInfosRsp
	if err := json.Unmarshal([]byte(rsp.Data), &resp); err != nil {
		log.Errorf(ctx, "fetchMeetRecordBasicInfos: unmarshal response failed: %v", err)
		return nil, err
	}
	return resp.MeetRecordBasicInfos, nil
}

// enrichMeetingWithFullRecords is like enrichMeetingsWithRecords but uses the
// paginated API (/v1/mcp/records/meet-basic-info-list) to fetch the full
// recording snapshot (up to maxFullRecords) for each meeting, instead of the
// batch top-5 API. It is intended for `meeting get` where the user wants to
// see all recordings, not just a preview.
func enrichMeetingWithFullRecords(ctx context.Context, tmeet *internal.Tmeet, data []byte, meetingInfoPath string) []byte {
	var meetingData map[string]interface{}
	if err := json.Unmarshal(data, &meetingData); err != nil {
		log.Warnf(ctx, "enrichMeetingWithFullRecords: unmarshal failed, skip: %v", err)
		return data
	}

	meetingInfoList, ok := meetingData[meetingInfoPath].([]interface{})
	if !ok || len(meetingInfoList) == 0 {
		log.Debugf(ctx, "enrichMeetingWithFullRecords: %s missing or empty, skip", meetingInfoPath)
		return data
	}

	for idx, item := range meetingInfoList {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		meetingID, _ := m["meeting_id"].(string)
		if meetingID == "" {
			log.Warnf(ctx, "enrichMeetingWithFullRecords: %s[%d] missing meeting_id, skip", meetingInfoPath, idx)
			continue
		}

		ri, err := fetchFullRecordBasicInfo(ctx, tmeet, meetingID)
		if err != nil {
			log.Errorf(ctx, "enrichMeetingWithFullRecords: fetch failed for meeting_id=%s: %v", meetingID, err)
			m["records_total_count"] = 0
			m["records"] = []interface{}{}
			continue
		}

		m["records_total_count"] = ri.RecordsTotalCount
		m["records"] = normalizeRecords(ri.Records)
		log.Debugf(ctx, "enrichMeetingWithFullRecords: meeting_id=%s total_count=%d fetched=%d",
			meetingID, ri.RecordsTotalCount, len(ri.Records))
	}

	result, err := json.Marshal(meetingData)
	if err != nil {
		log.Warnf(ctx, "enrichMeetingWithFullRecords: marshal failed, return original: %v", err)
		return data
	}
	return result
}

// fetchFullRecordBasicInfo calls GET /v1/mcp/records/meet-basic-info-list
// repeatedly until has_more is false or maxFullRecordPages is reached.
// Returns the aggregated recordBasicInfo with RecordsTotalCount set to the
// API's total_count (which may exceed the actual records returned because
// some records are security-struck and filtered out by the backend).
func fetchFullRecordBasicInfo(ctx context.Context, tmeet *internal.Tmeet, meetingID string) (recordBasicInfo, error) {
	var result recordBasicInfo
	result.MeetingID = meetingID
	result.Records = []interface{}{}

	pageToken := ""
	for page := 1; page <= maxFullRecordPages; page++ {
		queryParams := thttp.QueryParams{}
		queryParams.Set("meeting_id", meetingID)
		queryParams.Set("operator_id", tmeet.UserConfig.OpenId)
		queryParams.Set("operator_id_type", "2") // OpenId
		queryParams.Set("page_size", strconv.Itoa(fullRecordPageSize))
		if pageToken != "" {
			queryParams.Set("page_token", pageToken)
		}

		req := &thttp.Request{
			ApiURI:      "/v1/mcp/records/meet-basic-info-list",
			QueryParams: queryParams,
		}

		rsp, err := restProxy.RequestProxy(ctx, http.MethodGet, tmeet, req)
		if err != nil {
			log.Errorf(ctx, "fetchFullRecordBasicInfo: page %d request failed: %v", page, err)
			return result, err
		}

		var resp meetRecordBasicInfoListRsp
		if err := json.Unmarshal([]byte(rsp.Data), &resp); err != nil {
			log.Errorf(ctx, "fetchFullRecordBasicInfo: page %d unmarshal failed: %v", page, err)
			return result, err
		}

		result.Records = append(result.Records, resp.MeetRecordBasicInfo.Records...)
		result.RecordsTotalCount = resp.TotalCount
		log.Debugf(ctx, "fetchFullRecordBasicInfo: page=%d current_size=%d has_more=%v accumulated=%d total_count=%d",
			page, resp.CurrentSize, resp.HasMore, len(result.Records), resp.TotalCount)

		if !resp.HasMore {
			break
		}
		pageToken = resp.NextPageToken
		if pageToken == "" {
			break
		}
	}

	return result, nil
}

// recordEnrichmentFields lists the top-level fields that
// enrichMeetingsWithRecords injects into each meeting object. Using bare names
// (not "records.subject" etc.) makes utils.KeepFields keep the whole records
// subtree, so subject / url / state / media_start_time etc. are all retained
// without a sub-field list.
var recordEnrichmentFields = []string{"records", "records_total_count"}

// compactFieldsWithRecords returns the remote compact whitelist from ctx with
// recordEnrichmentFields appended, so the client-side injected record subtree
// survives output.WithCompact trimming (the remote schema does not know about
// those fields).
//
// The result is always a freshly allocated slice: the slice returned by
// middleware.GetCompactFields is backed by ctx and shared across callers, so
// appending to it in place could corrupt other readers.
func compactFieldsWithRecords(ctx context.Context) []string {
	base := middleWare.GetCompactFields(ctx)
	merged := make([]string, 0, len(base)+len(recordEnrichmentFields))
	merged = append(merged, base...)
	merged = append(merged, recordEnrichmentFields...)
	return merged
}

// normalizeRecords ensures the records slice is never nil so every meeting
// always emits a records array in the output. It also renames the API's
// state_int / type_int integer fields to human-readable state / type string
// fields, dropping the _int fields afterward.
//
// This rename-and-delete transform must stay here rather than in the
// caller's output.WithConvert convertMap: utils.ConvertFields only replaces
// the value of an already-existing key, it cannot rename a key or delete a
// sibling key. Pure value-format conversions that don't need renaming
// (subject base64 decode, duration seconds→HH:MM:SS) are instead handled by
// the caller via utils.Base64DecodeConverter / utils.HHMMSSConverter.
func normalizeRecords(records []interface{}) []interface{} {
	if records == nil {
		return []interface{}{}
	}
	for _, r := range records {
		m, ok := r.(map[string]interface{})
		if !ok {
			continue
		}
		// The API returns state_int / type_int as integers; rename them to
		// state / type with human-readable string values, then drop the _int
		// fields so they don't leak into the output.
		if stateInt, ok := m["state_int"]; ok {
			if state, ok := toInt(stateInt); ok {
				m["state"] = enumerate.RecordStateName(state)
			}
			delete(m, "state_int")
		}
		if typeInt, ok := m["type_int"]; ok {
			if recordType, ok := toInt(typeInt); ok {
				m["type"] = enumerate.RecordTypeName(recordType)
			}
			delete(m, "type_int")
		}
	}
	return records
}

// toInt converts a JSON-decoded numeric value (float64 / json.Number / string)
// to int. Returns ok=false when the value is not numeric.
func toInt(v interface{}) (int, bool) {
	switch t := v.(type) {
	case float64:
		return int(t), true
	case json.Number:
		n, err := t.Int64()
		return int(n), err == nil
	case string:
		n, err := strconv.Atoi(t)
		return n, err == nil
	}
	return 0, false
}

// parseUnixSeconds converts a JSON-decoded timestamp value (float64/string/
// json.Number) into a *int64 representing unix seconds. Returns nil when the
// value is missing or cannot be parsed so the field is omitted via omitempty
// instead of being sent as a misleading zero.
func parseUnixSeconds(v interface{}) *int64 {
	switch t := v.(type) {
	case float64:
		n := int64(t)
		return &n
	case json.Number:
		if n, err := t.Int64(); err == nil {
			return &n
		}
	case string:
		if t == "" {
			return nil
		}
		if n, err := strconv.ParseInt(t, 10, 64); err == nil {
			return &n
		}
	}
	return nil
}
