package meeting

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
	"tmeet/internal"
	"tmeet/internal/core/thttp"
	"tmeet/internal/log"
	restProxy "tmeet/internal/proxy/rest-proxy"
)

// minuteQuery is one element of the queries array in the request payload for
// POST /v1/mcp/asr/get-meeting-minutes-summary-mcp.
type minuteQuery struct {
	MeetingID    string `json:"meeting_id"`
	SubMeetingID string `json:"sub_meeting_id,omitempty"`
}

// minuteInfo is one element of the response array containing minute summary
// for a single meeting.
type minuteInfo struct {
	MeetingID        string        `json:"meeting_id"`
	SubMeetingID     string        `json:"sub_meeting_id,omitempty"`
	MinuteTotalCount int           `json:"minute_total_count"`
	Minutes          []interface{} `json:"minutes"`
}

// meetMinutesSummaryRsp is the response body of
// POST /v1/mcp/asr/get-meeting-minutes-summary-mcp.
type meetMinutesSummaryRsp struct {
	MeetMinuteInfos []minuteInfo `json:"summaries"`
}

// enrichMeetingsWithMinutes fetches Yuanbao minute summary for every meeting
// in data[meetingInfoPath] and merges minute_total_count + minutes into each
// meeting element (at the same level as meeting_id). It is best-effort: on any
// failure the original data is returned unchanged so meeting output is never
// blocked by minute query errors.
//
// meetingInfoPath is the top-level JSON key whose value is the meeting list
// (e.g. "meeting_info_list" for get/list; some APIs may use a different key,
// which is why it is passed by the caller).
//
// includeSubMeetingID controls whether sub_meeting_id is carried in the
// request payload. `meeting get` and `meeting list` should pass false (query
// by meeting_id only); `meeting list-ended` should pass true so recurring
// sub-meetings are matched precisely.
func enrichMeetingsWithMinutes(ctx context.Context, tmeet *internal.Tmeet, data []byte, meetingInfoPath string, includeSubMeetingID bool) []byte {
	var meetingData map[string]interface{}
	if err := json.Unmarshal(data, &meetingData); err != nil {
		log.Warnf(ctx, "enrichMeetingsWithMinutes: unmarshal meeting data failed, skip enrich: %v", err)
		return data
	}

	meetingInfoList, ok := meetingData[meetingInfoPath].([]interface{})
	if !ok || len(meetingInfoList) == 0 {
		log.Debugf(ctx, "enrichMeetingsWithMinutes: %s missing or empty, skip enrich", meetingInfoPath)
		return data
	}

	// Collect meeting queries while keeping a reference to each meeting map
	// so we can merge results back without a second lookup pass.
	type meetingRef struct {
		meetingID    string
		subMeetingID string
		obj          map[string]interface{}
	}
	refs := make([]meetingRef, 0, len(meetingInfoList))
	queries := make([]minuteQuery, 0, len(meetingInfoList))
	for idx, item := range meetingInfoList {
		m, ok := item.(map[string]interface{})
		if !ok {
			log.Warnf(ctx, "enrichMeetingsWithMinutes: %s[%d] is not an object, skip", meetingInfoPath, idx)
			continue
		}
		meetingID, _ := m["meeting_id"].(string)
		if meetingID == "" {
			log.Warnf(ctx, "enrichMeetingsWithMinutes: %s[%d] missing meeting_id, skip", meetingInfoPath, idx)
			continue
		}
		subMeetingID, _ := m["sub_meeting_id"].(string)
		refs = append(refs, meetingRef{meetingID: meetingID, subMeetingID: subMeetingID, obj: m})
		q := minuteQuery{
			MeetingID: meetingID,
		}
		if includeSubMeetingID {
			q.SubMeetingID = subMeetingID
		}
		queries = append(queries, q)
	}
	if len(queries) == 0 {
		log.Debugf(ctx, "enrichMeetingsWithMinutes: no valid meeting id collected, skip minute query")
		return data
	}

	minuteInfos, err := fetchMeetMinutesSummary(ctx, tmeet, queries)
	if err != nil {
		log.Errorf(ctx, "enrichMeetingsWithMinutes: fetch minute summary failed: %v", err)
		return data
	}

	// Build lookup map keyed by "meeting_id|sub_meeting_id". When the request
	// did not carry sub_meeting_id we key solely on meeting_id, because the
	// API is expected to return one aggregated entry per meeting_id.
	lookup := make(map[string]minuteInfo, len(minuteInfos))
	for _, mi := range minuteInfos {
		if includeSubMeetingID {
			lookup[mi.MeetingID+"|"+mi.SubMeetingID] = mi
		} else {
			lookup[mi.MeetingID] = mi
		}
	}

	// Merge minute info into each meeting object.
	var matched, unmatched int
	for _, ref := range refs {
		var key string
		if includeSubMeetingID {
			key = ref.meetingID + "|" + ref.subMeetingID
		} else {
			key = ref.meetingID
		}
		if mi, found := lookup[key]; found {
			ref.obj["minute_total_count"] = mi.MinuteTotalCount
			ref.obj["minutes"] = normalizeMinutes(mi.Minutes)
			matched++
			log.Debugf(ctx, "enrichMeetingsWithMinutes: matched meeting_id=%s sub_meeting_id=%s minute_total_count=%d",
				ref.meetingID, ref.subMeetingID, mi.MinuteTotalCount)
		} else {
			// Meeting not returned by the minutes API: treat as no minutes.
			ref.obj["minute_total_count"] = 0
			ref.obj["minutes"] = []interface{}{}
			unmatched++
			log.Debugf(ctx, "enrichMeetingsWithMinutes: no minute for meeting_id=%s sub_meeting_id=%s",
				ref.meetingID, ref.subMeetingID)
		}
	}

	result, err := json.Marshal(meetingData)
	if err != nil {
		log.Warnf(ctx, "enrichMeetingsWithMinutes: marshal enriched data failed, return original: %v", err)
		return data
	}
	return result
}

// fetchMeetMinutesSummary calls POST /v1/mcp/asr/get-meeting-minutes-summary-mcp
// to retrieve Yuanbao minute summary for the given meetings.
func fetchMeetMinutesSummary(ctx context.Context, tmeet *internal.Tmeet, queries []minuteQuery) ([]minuteInfo, error) {
	body := map[string]interface{}{
		"operator_id":      tmeet.UserConfig.OpenId,
		"operator_id_type": 2, // OpenId
		"queries":          queries,
	}
	req := &thttp.Request{
		ApiURI: "/v1/mcp/asr/get-meeting-minutes-summary-mcp",
		Body:   body,
	}

	rsp, err := restProxy.RequestProxy(ctx, http.MethodPost, tmeet, req)
	if err != nil {
		log.Errorf(ctx, "fetchMeetMinutesSummary: request failed: %v", err)
		return nil, err
	}

	var resp meetMinutesSummaryRsp
	if err := json.Unmarshal([]byte(rsp.Data), &resp); err != nil {
		log.Errorf(ctx, "fetchMeetMinutesSummary: unmarshal response failed: %v", err)
		return nil, err
	}
	return resp.MeetMinuteInfos, nil
}

// minuteEnrichmentFields lists the top-level fields that
// enrichMeetingsWithMinutes injects into each meeting object. Callers pass
// this as extra arguments to output.WithCompact so the injected subtree
// survives compact trimming.
var minuteEnrichmentFields = []string{"minutes", "minute_total_count"}

// normalizeMinutes ensures the minutes slice is never nil so every meeting
// always emits a minutes array in the output. It also converts the
// minute_start_time field from a unix timestamp to a human-readable string
// (YYYY-MM-DD HH:MM format) in-place. Conversion is best-effort: if the
// value is missing or fails to parse, the original value is kept.
func normalizeMinutes(minutes []interface{}) []interface{} {
	if minutes == nil {
		return []interface{}{}
	}
	for _, item := range minutes {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		// Convert minute_start_time from timestamp to readable format.
		if raw, ok := m["minute_start_time"]; ok {
			if formatted := formatMinuteTime(raw); formatted != "" {
				m["minute_start_time"] = formatted
			}
		}
	}
	return minutes
}

// formatMinuteTime converts a timestamp value (float64/string/json.Number)
// into a "YYYY-MM-DD HH:MM" formatted string. Returns empty string when the
// value cannot be parsed, so the caller can keep the original value.
func formatMinuteTime(v interface{}) string {
	p := parseUnixSeconds(v)
	if p == nil {
		return ""
	}
	ts := *p
	const millisecondThreshold = int64(1e11)
	if ts > millisecondThreshold {
		// Millisecond-level timestamp, convert to second-level.
		ts = ts / 1000
	}
	if ts == 0 {
		return ""
	}
	return time.Unix(ts, 0).Format("2006-01-02 15:04")
}
