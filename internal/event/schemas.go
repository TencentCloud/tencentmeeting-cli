// Package event — built-in EventKey definitions.
//
// The following keys are seeded, all sourced from the Tencent Meeting
// open-platform webhook contract:
//
//   - meeting.started — meeting started
//     (https://cloud.tencent.com/document/product/1095/51618)
//   - meeting.end     — meeting ended
//     (https://cloud.tencent.com/document/product/1095/51619)
//   - meeting.created — meeting created
//     (https://cloud.tencent.com/document/product/1095/51614)
//   - meeting.updated — meeting updated
//     (https://cloud.tencent.com/document/product/1095/51615)
//   - meeting.canceled — meeting canceled
//     (https://cloud.tencent.com/document/product/1095/51616)
//   - recording.completed — cloud recording completed
//     (https://cloud.tencent.com/document/product/1095/53230)
//   - recording.failed — cloud recording failed
//     (https://cloud.tencent.com/document/product/1095/53231)
//   - smart.transcripts — recording transcript generated
//     (https://cloud.tencent.com/document/product/1095/121282)
//   - smart.minutes — smart minutes generated
//
// The Tencent Meeting webhook envelope is:
//
//	{
//	  "event":    "<key>",
//	  "trace_id": "<id>",
//	  "payload":  [ { "operate_time": ..., "operator": {...}, "meeting_info": {...} } ]
//	}
//
// About the payload shape (confirmed with the server team):
//
//   - The wire payload is array<object>, but for meeting.started / meeting.end
//     the server contract guarantees the array length is always 1 — the array
//     shape is reserved for future batch pushes and is not enabled today.
//   - Therefore addressing payload[0] directly via PayloadPath (e.g.
//     "0.meeting_info.meeting_id") is safe: there is no risk of false
//     negatives from missing non-first elements.
//   - extractScalarString already supports non-negative integer index
//     segments (see the stepInto notes in params.go), so this PayloadPath
//     drives L2 filtering as-is.
//
// Note on the event name: per doc 51619 the actual key is "meeting.end"
// (not "meeting.ended"), as shown explicitly in the official example.
//
// Code organization:
//   - init() only dispatches to the three register<Domain>Domain() helpers;
//   - each domain helper invokes the register<Key>() functions of that
//     domain in order;
//   - each register<Key>() helper performs exactly one RegisterKey call.
//     To add a new event: write a register<Key>() helper, then append it to
//     the matching domain helper — nothing else needs to change.
package event

import "encoding/json"

func init() {
	registerMeetingDomain()
	registerRecordingDomain()
	registerSmartDomain()
}

// ---------------------------------------------------------------------------
// meeting domain: meeting lifecycle events
// ---------------------------------------------------------------------------

// registerMeetingDomain registers all events under the meeting domain in order.
// The registration order follows the meeting lifecycle:
// created → updated → canceled → started → end.
func registerMeetingDomain() {
	registerMeetingCreated()
	registerMeetingUpdated()
	registerMeetingCanceled()
	registerMeetingStarted()
	registerMeetingEnd()
}

// registerMeetingStarted — meeting started event.
// Doc: https://cloud.tencent.com/document/product/1095/51618
//
// Field semantics (excerpted from the official example):
//
//	payload[].operate_time       event operation timestamp (ms)
//	payload[].operator.userid    operator id (userid for same-tenant users, openId for OAuth users, roomsId for rooms)
//	payload[].operator.open_id   open platform OpenID
//	payload[].operator.uuid      user identity id
//	payload[].operator.user_name operator display name
//	payload[].operator.ms_open_id
//	payload[].operator.instance_id user terminal device type
//	payload[].meeting_info.meeting_id           meeting ID
//	payload[].meeting_info.meeting_code         meeting code
//	payload[].meeting_info.subject              meeting subject
//	payload[].meeting_info.creator.{userid,open_id,uuid,user_name,ms_open_id,instance_id}
//	payload[].meeting_info.meeting_type         0 one-time / 1 recurring / 2 WeChat-exclusive / 4 rooms projection / 5 personal meeting ID
//	payload[].meeting_info.start_time           start timestamp (seconds)
//	payload[].meeting_info.end_time             end timestamp (seconds)
//	payload[].meeting_info.meeting_create_mode  0 regular / 1 quick
//	payload[].meeting_info.meeting_create_from  0 empty / 1 client / 2 web / 3 WeCom / 4 WeChat / 5 outlook / 6 restapi / 7 Tencent Docs / 8 Rooms smart recording
//
// PayloadPath addresses payload[0] directly: the server contract guarantees
// this array has length 1 (reserved for future batching, not enabled today);
// extractScalarString already supports array-index segments (see the
// stepInto notes in params.go).
func registerMeetingStarted() {
	RegisterKey(KeyDef{
		Key:         "meeting.started",
		Domain:      "meeting",
		Description: "会议开始事件",
		JQRootPath:  ".payload",
		ParamsSchema: map[string]ParamDef{
			"meeting_id": {
				Type:        "string",
				Required:    false,
				Description: "仅推送该 meeting_id 的事件；不传则接收本账号名下所有会议",
				PayloadPath: "0.meeting_info.meeting_id",
			},
		},
		ResolvedOutputSchema: json.RawMessage(`{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "required": ["event", "trace_id", "payload"],
  "properties": {
    "event":    { "type": "string", "const": "meeting.started" },
    "trace_id": { "type": "string" },
    "payload": {
      "type": "array",
      "description": "事件内容",
      "items": {
        "type": "object",
        "required": ["operate_time", "operator", "meeting_info"],
        "properties": {
          "operate_time": { "type": "integer", "description": "毫秒级别事件操作时间戳" },
          "operator": {
            "type": "object",
            "description": "事件操作者信息",
            "properties": {
              "userid":      { "type": "string", "description": "用户 id" },
              "open_id":     { "type": "string", "description": "会议开放平台用户标识" },
              "uuid":        { "type": "string", "description": "用户身份 ID" },
              "user_name":   { "type": "string", "description": "用户昵称" },
              "ms_open_id":  { "type": "string", "description": "会议开放平台会中用户标识" },
              "instance_id": { "type": "string", "description": "用户的终端设备类型" }
            }
          },
          "meeting_info": {
            "type": "object",
            "description": "会议信息",
            "required": ["meeting_id", "meeting_code", "subject"],
            "properties": {
              "meeting_id":   { "type": "string", "description": "会议唯一标识" },
              "meeting_code": { "type": "string", "description": "会议号" },
              "subject":      { "type": "string", "description": "会议主题" },
              "creator": {
                "type": "object",
                "description": "会议创建者信息",
                "properties": {
                  "userid":      { "type": "string", "description": "用户 id" },
                  "open_id":     { "type": "string", "description": "会议开放平台用户标识" },
                  "uuid":        { "type": "string", "description": "用户身份 ID" },
                  "user_name":   { "type": "string", "description": "用户昵称" },
                  "ms_open_id":  { "type": "string", "description": "会议开放平台会中用户标识" },
                  "instance_id": { "type": "string", "description": "用户的终端设备类型" }
                }
              },
              "meeting_type":        { "type": "integer", "description": "会议类型 0:一次性 1:周期性 2:微信专属 4:rooms投屏 5:个人会议号" },
              "start_time":          { "type": "integer", "description": "秒级别会议开始时间戳" },
              "end_time":            { "type": "integer", "description": "秒级别会议结束时间戳" },
              "meeting_create_mode": { "type": "integer", "description": "会议创建类型 0:普通 1:快速" },
              "meeting_create_from": { "type": "integer", "description": "会议创建来源 0:空 1:客户端 2:web 3:企微 4:微信 5:outlook 6:restapi 7:腾讯文档 8:Rooms智能录制" }
            }
          }
        }
      }
    }
  }
}`),
	})
}

// registerMeetingEnd — meeting ended event.
// Doc: https://cloud.tencent.com/document/product/1095/51619
//
// Note: the official Tencent Meeting event name is "meeting.end" (not
// "meeting.ended").
//
// Compared to meeting.started, each payload[] element carries one extra
// top-level field:
//
//	payload[].meeting_end_type
//	  0: proactively ended by the host
//	  1: last participant left and scheduled end time exceeded
//	  2: no participants in the meeting and scheduled end time exceeded
//	  3: no participants and scheduled end time not yet reached
func registerMeetingEnd() {
	RegisterKey(KeyDef{
		Key:         "meeting.end",
		Domain:      "meeting",
		Description: "会议结束事件",
		JQRootPath:  ".payload",
		ParamsSchema: map[string]ParamDef{
			"meeting_id": {
				Type:        "string",
				Required:    false,
				Description: "仅推送该 meeting_id 的事件；不传则接收本账号名下所有会议",
				PayloadPath: "0.meeting_info.meeting_id",
			},
		},
		ResolvedOutputSchema: json.RawMessage(`{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "required": ["event", "trace_id", "payload"],
  "properties": {
    "event":    { "type": "string", "const": "meeting.end" },
    "trace_id": { "type": "string" },
    "payload": {
      "type": "array",
      "description": "事件内容",
      "items": {
        "type": "object",
        "required": ["operate_time", "operator", "meeting_info"],
        "properties": {
          "operate_time": { "type": "integer", "description": "毫秒级别事件操作时间戳" },
          "operator": {
            "type": "object",
            "description": "事件操作者信息",
            "properties": {
              "userid":      { "type": "string", "description": "用户 id" },
              "open_id":     { "type": "string", "description": "会议开放平台用户标识" },
              "uuid":        { "type": "string", "description": "用户身份 ID" },
              "user_name":   { "type": "string", "description": "用户昵称" },
              "ms_open_id":  { "type": "string", "description": "会议开放平台会中用户标识" },
              "instance_id": { "type": "string", "description": "用户的终端设备类型" }
            }
          },
          "meeting_end_type": {
            "type": "integer",
            "description": "0:主动结束 1:最后一人离开且超时 2:无人且超时 3:无人且未到结束时间"
          },
          "meeting_info": {
            "type": "object",
            "description": "会议信息",
            "required": ["meeting_id", "meeting_code", "subject"],
            "properties": {
              "meeting_id":   { "type": "string", "description": "会议唯一标识" },
              "meeting_code": { "type": "string", "description": "会议号" },
              "subject":      { "type": "string", "description": "会议主题" },
              "creator": {
                "type": "object",
                "description": "会议创建者信息",
                "properties": {
                  "userid":      { "type": "string", "description": "用户 id" },
                  "open_id":     { "type": "string", "description": "会议开放平台用户标识" },
                  "uuid":        { "type": "string", "description": "用户身份 ID" },
                  "user_name":   { "type": "string", "description": "用户昵称" },
                  "ms_open_id":  { "type": "string", "description": "会议开放平台会中用户标识" },
                  "instance_id": { "type": "string", "description": "用户的终端设备类型" }
                }
              },
              "meeting_type":        { "type": "integer", "description": "会议类型 0:一次性 1:周期性 2:微信专属 4:rooms投屏 5:个人会议号" },
              "start_time":          { "type": "integer", "description": "秒级别会议开始时间戳" },
              "end_time":            { "type": "integer", "description": "秒级别会议结束时间戳" },
              "meeting_create_mode": { "type": "integer", "description": "会议创建类型 0:普通 1:快速" },
              "meeting_create_from": { "type": "integer", "description": "会议创建来源 0:空 1:客户端 2:web 3:企微 4:微信 5:outlook 6:restapi 7:腾讯文档 8:Rooms智能录制" }
            }
          }
        }
      }
    }
  }
}`),
	})
}

// registerMeetingCreated — meeting created event.
// Doc: https://cloud.tencent.com/document/product/1095/51614
//
// The payload[] shape is essentially identical to meeting.started:
//
//	operate_time / operator / meeting_info
//
// where meeting_info covers the full meeting metadata (meeting_id,
// meeting_code, subject, creator, meeting_type, start_time, end_time,
// meeting_create_mode, meeting_create_from). The only semantic differences
// are the trigger timing:
//
//	meeting.created  the meeting has been created (scheduled or quick meeting persisted)
//	meeting.started  the meeting has actually started
//
// As with meeting.started / meeting.end, the server contract guarantees the
// payload array length is 1 (batching not enabled), so PayloadPath
// addressing payload[0] directly is safe.
func registerMeetingCreated() {
	RegisterKey(KeyDef{
		Key:         "meeting.created",
		Domain:      "meeting",
		Description: "会议创建事件",
		JQRootPath:  ".payload",
		ParamsSchema: map[string]ParamDef{
			"meeting_id": {
				Type:        "string",
				Required:    false,
				Description: "仅推送该 meeting_id 的事件；不传则接收本账号名下所有会议",
				PayloadPath: "0.meeting_info.meeting_id",
			},
		},
		ResolvedOutputSchema: json.RawMessage(`{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "required": ["event", "trace_id", "payload"],
  "properties": {
    "event":    { "type": "string", "const": "meeting.created" },
    "trace_id": { "type": "string" },
    "payload": {
      "type": "array",
      "description": "事件内容",
      "items": {
        "type": "object",
        "required": ["operate_time", "operator", "meeting_info"],
        "properties": {
          "operate_time": { "type": "integer", "description": "毫秒级别事件操作时间戳" },
          "operator": {
            "type": "object",
            "description": "事件操作者信息",
            "properties": {
              "userid":      { "type": "string", "description": "用户 id" },
              "open_id":     { "type": "string", "description": "会议开放平台用户标识" },
              "uuid":        { "type": "string", "description": "用户身份 ID" },
              "user_name":   { "type": "string", "description": "用户昵称" },
              "ms_open_id":  { "type": "string", "description": "会议开放平台会中用户标识" },
              "instance_id": { "type": "string", "description": "用户的终端设备类型" }
            }
          },
          "meeting_info": {
            "type": "object",
            "description": "会议信息",
            "required": ["meeting_id", "meeting_code", "subject"],
            "properties": {
              "meeting_id":   { "type": "string", "description": "会议唯一标识" },
              "meeting_code": { "type": "string", "description": "会议号" },
              "subject":      { "type": "string", "description": "会议主题" },
              "creator": {
                "type": "object",
                "description": "会议创建者信息",
                "properties": {
                  "userid":      { "type": "string", "description": "用户 id" },
                  "open_id":     { "type": "string", "description": "会议开放平台用户标识" },
                  "uuid":        { "type": "string", "description": "用户身份 ID" },
                  "user_name":   { "type": "string", "description": "用户昵称" },
                  "ms_open_id":  { "type": "string", "description": "会议开放平台会中用户标识" },
                  "instance_id": { "type": "string", "description": "用户的终端设备类型" }
                }
              },
              "meeting_type":        { "type": "integer", "description": "会议类型 0:一次性 1:周期性 2:微信专属 4:rooms投屏 5:个人会议号" },
              "start_time":          { "type": "integer", "description": "秒级别会议开始时间戳" },
              "end_time":            { "type": "integer", "description": "秒级别会议结束时间戳" },
              "meeting_create_mode": { "type": "integer", "description": "会议创建类型 0:普通 1:快速" },
              "meeting_create_from": { "type": "integer", "description": "会议创建来源 0:空 1:客户端 2:web 3:企微 4:微信 5:outlook 6:restapi 7:腾讯文档 8:Rooms智能录制" }
            }
          }
        }
      }
    }
  }
}`),
	})
}

// registerMeetingUpdated — meeting updated event.
// Doc: https://cloud.tencent.com/document/product/1095/51615
//
// Trigger: pushed after the meeting information (subject, start/end time,
// creation source, etc.) is modified.
// The payload[] element shape is identical to meeting.created:
// operate_time / operator / meeting_info; only the semantics ("updated" vs
// "created") and the event key value differ.
func registerMeetingUpdated() {
	RegisterKey(KeyDef{
		Key:         "meeting.updated",
		Domain:      "meeting",
		Description: "会议更新事件",
		JQRootPath:  ".payload",
		ParamsSchema: map[string]ParamDef{
			"meeting_id": {
				Type:        "string",
				Required:    false,
				Description: "仅推送该 meeting_id 的事件；不传则接收本账号名下所有会议",
				PayloadPath: "0.meeting_info.meeting_id",
			},
		},
		ResolvedOutputSchema: json.RawMessage(`{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "required": ["event", "trace_id", "payload"],
  "properties": {
    "event":    { "type": "string", "const": "meeting.updated" },
    "trace_id": { "type": "string" },
    "payload": {
      "type": "array",
      "description": "事件内容",
      "items": {
        "type": "object",
        "required": ["operate_time", "operator", "meeting_info"],
        "properties": {
          "operate_time": { "type": "integer", "description": "毫秒级别事件操作时间戳" },
          "operator": {
            "type": "object",
            "description": "事件操作者信息",
            "properties": {
              "userid":      { "type": "string", "description": "用户 id" },
              "open_id":     { "type": "string", "description": "会议开放平台用户标识" },
              "uuid":        { "type": "string", "description": "用户身份 ID" },
              "user_name":   { "type": "string", "description": "用户昵称" },
              "ms_open_id":  { "type": "string", "description": "会议开放平台会中用户标识" },
              "instance_id": { "type": "string", "description": "用户的终端设备类型" }
            }
          },
          "meeting_info": {
            "type": "object",
            "description": "会议信息",
            "required": ["meeting_id", "meeting_code", "subject"],
            "properties": {
              "meeting_id":   { "type": "string", "description": "会议唯一标识" },
              "meeting_code": { "type": "string", "description": "会议号" },
              "subject":      { "type": "string", "description": "会议主题" },
              "creator": {
                "type": "object",
                "description": "会议创建者信息",
                "properties": {
                  "userid":      { "type": "string", "description": "用户 id" },
                  "open_id":     { "type": "string", "description": "会议开放平台用户标识" },
                  "uuid":        { "type": "string", "description": "用户身份 ID" },
                  "user_name":   { "type": "string", "description": "用户昵称" },
                  "ms_open_id":  { "type": "string", "description": "会议开放平台会中用户标识" },
                  "instance_id": { "type": "string", "description": "用户的终端设备类型" }
                }
              },
              "meeting_type":        { "type": "integer", "description": "会议类型 0:一次性 1:周期性 2:微信专属 4:rooms投屏 5:个人会议号" },
              "start_time":          { "type": "integer", "description": "秒级别会议开始时间戳" },
              "end_time":            { "type": "integer", "description": "秒级别会议结束时间戳" },
              "meeting_create_mode": { "type": "integer", "description": "会议创建类型 0:普通 1:快速" },
              "meeting_create_from": { "type": "integer", "description": "会议创建来源 0:空 1:客户端 2:web 3:企微 4:微信 5:outlook 6:restapi 7:腾讯文档 8:Rooms智能录制" }
            }
          }
        }
      }
    }
  }
}`),
	})
}

// registerMeetingCanceled — meeting canceled event.
// Doc: https://cloud.tencent.com/document/product/1095/51616
//
// Trigger: a scheduled meeting is canceled. The official Tencent Meeting key
// is "meeting.canceled" (American spelling, single "l"), NOT
// "meeting.cancelled"; do not change it.
// The payload[] element shape is identical to meeting.created /
// meeting.updated.
func registerMeetingCanceled() {
	RegisterKey(KeyDef{
		Key:         "meeting.canceled",
		Domain:      "meeting",
		Description: "会议取消事件",
		JQRootPath:  ".payload",
		ParamsSchema: map[string]ParamDef{
			"meeting_id": {
				Type:        "string",
				Required:    false,
				Description: "仅推送该 meeting_id 的事件；不传则接收本账号名下所有会议",
				PayloadPath: "0.meeting_info.meeting_id",
			},
		},
		ResolvedOutputSchema: json.RawMessage(`{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "required": ["event", "trace_id", "payload"],
  "properties": {
    "event":    { "type": "string", "const": "meeting.canceled" },
    "trace_id": { "type": "string" },
    "payload": {
      "type": "array",
      "description": "事件内容",
      "items": {
        "type": "object",
        "required": ["operate_time", "operator", "meeting_info"],
        "properties": {
          "operate_time": { "type": "integer", "description": "毫秒级别事件操作时间戳" },
          "operator": {
            "type": "object",
            "description": "事件操作者信息",
            "properties": {
              "userid":      { "type": "string", "description": "用户 id" },
              "open_id":     { "type": "string", "description": "会议开放平台用户标识" },
              "uuid":        { "type": "string", "description": "用户身份 ID" },
              "user_name":   { "type": "string", "description": "用户昵称" },
              "ms_open_id":  { "type": "string", "description": "会议开放平台会中用户标识" },
              "instance_id": { "type": "string", "description": "用户的终端设备类型" }
            }
          },
          "meeting_info": {
            "type": "object",
            "description": "会议信息",
            "required": ["meeting_id", "meeting_code", "subject"],
            "properties": {
              "meeting_id":   { "type": "string", "description": "会议唯一标识" },
              "meeting_code": { "type": "string", "description": "会议号" },
              "subject":      { "type": "string", "description": "会议主题" },
              "creator": {
                "type": "object",
                "description": "会议创建者信息",
                "properties": {
                  "userid":      { "type": "string", "description": "用户 id" },
                  "open_id":     { "type": "string", "description": "会议开放平台用户标识" },
                  "uuid":        { "type": "string", "description": "用户身份 ID" },
                  "user_name":   { "type": "string", "description": "用户昵称" },
                  "ms_open_id":  { "type": "string", "description": "会议开放平台会中用户标识" },
                  "instance_id": { "type": "string", "description": "用户的终端设备类型" }
                }
              },
              "meeting_type":        { "type": "integer", "description": "会议类型 0:一次性 1:周期性 2:微信专属 4:rooms投屏 5:个人会议号" },
              "start_time":          { "type": "integer", "description": "秒级别会议开始时间戳" },
              "end_time":            { "type": "integer", "description": "秒级别会议结束时间戳" },
              "meeting_create_mode": { "type": "integer", "description": "会议创建类型 0:普通 1:快速" },
              "meeting_create_from": { "type": "integer", "description": "会议创建来源 0:空 1:客户端 2:web 3:企微 4:微信 5:outlook 6:restapi 7:腾讯文档 8:Rooms智能录制" }
            }
          }
        }
      }
    }
  }
}`),
	})
}

// ---------------------------------------------------------------------------
// recording domain: cloud recording lifecycle events
// ---------------------------------------------------------------------------

// registerRecordingDomain registers all events under the recording domain in order.
func registerRecordingDomain() {
	registerRecordingCompleted()
}

// registerRecordingCompleted — cloud recording completed event.
// Doc: https://cloud.tencent.com/document/product/1095/53230
//
// Trigger: pushed after the cloud recording is finished and the recording
// files are transcoded and persisted.
// Compared to the meeting.* events, each payload[] element adds a
// recording_files[] array next to meeting_info:
//
//	payload[].operate_time                       event operation timestamp (ms)
//	payload[].operator{...}                      operator who triggered recording
//	payload[].meeting_info{...}                  meeting metadata (same as meeting.*)
//	payload[].recording_files[].record_file_id   recording file ID
//	payload[].recording_files[].lang             file language
//	payload[].recording_files[].start_time       recording start timestamp (seconds)
//	payload[].recording_files[].end_time         recording end timestamp (seconds)
//	payload[].recording_files[].total_size       file size in bytes
func registerRecordingCompleted() {
	RegisterKey(KeyDef{
		Key:         "recording.completed",
		Domain:      "recording",
		Description: "云录制已完成事件",
		JQRootPath:  ".payload",
		ParamsSchema: map[string]ParamDef{
			"meeting_id": {
				Type:        "string",
				Required:    false,
				Description: "仅推送该 meeting_id 的事件；不传则接收本账号名下所有会议",
				PayloadPath: "0.meeting_info.meeting_id",
			},
		},
		ResolvedOutputSchema: json.RawMessage(`{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "required": ["event", "trace_id", "payload"],
  "properties": {
    "event":    { "type": "string", "const": "recording.completed" },
    "trace_id": { "type": "string" },
    "payload": {
      "type": "array",
      "description": "事件内容",
      "items": {
        "type": "object",
        "required": ["operate_time", "meeting_info"],
        "properties": {
          "operate_time": { "type": "integer", "description": "毫秒级别事件操作时间戳" },
          "operator": {
            "type": "object",
            "description": "事件操作者信息",
            "properties": {
              "userid":      { "type": "string", "description": "用户 id" },
              "open_id":     { "type": "string", "description": "会议开放平台用户标识" },
              "uuid":        { "type": "string", "description": "用户身份 ID" },
              "user_name":   { "type": "string", "description": "用户昵称" },
              "ms_open_id":  { "type": "string", "description": "会议开放平台会中用户标识" },
              "instance_id": { "type": "string", "description": "用户的终端设备类型" }
            }
          },
          "recording_files": {
            "type": "array",
            "description": "录制文件列表",
            "items": {
              "type": "object",
              "properties": {
                "record_file_id": { "type": "string", "description": "录制文件 ID" },
                "lang":           { "type": "string", "description": "录制文件语言" },
                "start_time":     { "type": "integer", "description": "秒级录制开始时间戳" },
                "end_time":       { "type": "integer", "description": "秒级录制结束时间戳" },
                "total_size":     { "type": "integer", "description": "录制文件大小（字节）" }
              }
            }
          },
          "meeting_info": {
            "type": "object",
            "description": "会议信息",
            "required": ["meeting_id", "meeting_code", "subject"],
            "properties": {
              "meeting_id":   { "type": "string", "description": "会议唯一标识" },
              "meeting_code": { "type": "string", "description": "会议号" },
              "subject":      { "type": "string", "description": "会议主题" },
              "creator": {
                "type": "object",
                "description": "会议创建者信息",
                "properties": {
                  "userid":      { "type": "string", "description": "用户 id" },
                  "open_id":     { "type": "string", "description": "会议开放平台用户标识" },
                  "uuid":        { "type": "string", "description": "用户身份 ID" },
                  "user_name":   { "type": "string", "description": "用户昵称" },
                  "ms_open_id":  { "type": "string", "description": "会议开放平台会中用户标识" },
                  "instance_id": { "type": "string", "description": "用户的终端设备类型" }
                }
              },
              "meeting_type":        { "type": "integer", "description": "会议类型 0:一次性 1:周期性 2:微信专属 4:rooms投屏 5:个人会议号" },
              "start_time":          { "type": "integer", "description": "秒级别会议开始时间戳" },
              "end_time":            { "type": "integer", "description": "秒级别会议结束时间戳" },
              "meeting_create_mode": { "type": "integer", "description": "会议创建类型 0:普通 1:快速" },
              "meeting_create_from": { "type": "integer", "description": "会议创建来源 0:空 1:客户端 2:web 3:企微 4:微信 5:outlook 6:restapi 7:腾讯文档 8:Rooms智能录制" }
            }
          }
        }
      }
    }
  }
}`),
	})
}

// ---------------------------------------------------------------------------
// smart domain: AI / smart capability events
// ---------------------------------------------------------------------------

// registerSmartDomain registers all events under the smart domain in order.
func registerSmartDomain() {
	registerSmartTranscripts()
	registerSmartMinutes()
}

// registerSmartTranscripts — recording transcript generated event.
// Doc: https://cloud.tencent.com/document/product/1095/121282
//
// Trigger: pushed after the speech transcription (transcript) for a
// recording file has been generated.
// The official key is "smart.transcripts" (it belongs to the AI/smart
// capability domain alongside smart.minutes), NOT under the recording.*
// namespace — do not confuse them.
// Compared to recording.completed, each payload[] element does not carry
// operator, and recording_files[] only exposes record_file_id / lang (the
// transcript file itself has no size/duration semantics; query the recording
// file detail API separately if needed).
func registerSmartTranscripts() {
	RegisterKey(KeyDef{
		Key:         "smart.transcripts",
		Domain:      "smart",
		Description: "录制转写生成事件",
		JQRootPath:  ".payload",
		ParamsSchema: map[string]ParamDef{
			"meeting_id": {
				Type:        "string",
				Required:    false,
				Description: "仅推送该 meeting_id 的事件；不传则接收本账号名下所有会议",
				PayloadPath: "0.meeting_info.meeting_id",
			},
		},
		ResolvedOutputSchema: json.RawMessage(`{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "required": ["event", "trace_id", "payload"],
  "properties": {
    "event":    { "type": "string", "const": "smart.transcripts" },
    "trace_id": { "type": "string" },
    "payload": {
      "type": "array",
      "description": "事件内容",
      "items": {
        "type": "object",
        "required": ["operate_time", "meeting_info"],
        "properties": {
          "operate_time": { "type": "integer", "description": "毫秒级别事件操作时间戳" },
          "recording_files": {
            "type": "array",
            "description": "转写文件列表",
            "items": {
              "type": "object",
              "properties": {
                "record_file_id": { "type": "string", "description": "录制文件 ID" },
                "lang":           { "type": "string", "description": "转写文件语言" }
              }
            }
          },
          "meeting_info": {
            "type": "object",
            "description": "会议信息",
            "required": ["meeting_id", "meeting_code", "subject"],
            "properties": {
              "meeting_id":   { "type": "string", "description": "会议唯一标识" },
              "meeting_code": { "type": "string", "description": "会议号" },
              "subject":      { "type": "string", "description": "会议主题" },
              "creator": {
                "type": "object",
                "description": "会议创建者信息",
                "properties": {
                  "userid":      { "type": "string", "description": "用户 id" },
                  "open_id":     { "type": "string", "description": "会议开放平台用户标识" },
                  "uuid":        { "type": "string", "description": "用户身份 ID" },
                  "user_name":   { "type": "string", "description": "用户昵称" },
                  "ms_open_id":  { "type": "string", "description": "会议开放平台会中用户标识" },
                  "instance_id": { "type": "string", "description": "用户的终端设备类型" }
                }
              },
              "meeting_type":        { "type": "integer", "description": "会议类型 0:一次性 1:周期性 2:微信专属 4:rooms投屏 5:个人会议号" },
              "start_time":          { "type": "integer", "description": "秒级别会议开始时间戳" },
              "end_time":            { "type": "integer", "description": "秒级别会议结束时间戳" },
              "meeting_create_mode": { "type": "integer", "description": "会议创建类型 0:普通 1:快速" },
              "meeting_create_from": { "type": "integer", "description": "会议创建来源 0:空 1:客户端 2:web 3:企微 4:微信 5:outlook 6:restapi 7:腾讯文档 8:Rooms智能录制" }
            }
          }
        }
      }
    }
  }
}`),
	})
}

// registerSmartMinutes — smart minutes generated event.
//
// Primary-account event: only the primary account can subscribe.
//
// Payload shape (still array<object>, current array length is always 1).
// job_id and result describe the overall outcome of this minutes job and
// are placed inside the payload[] element so that they are preserved when
// routed through the RawEvent pipeline (which only carries
// event/trace_id/payload):
//
//	payload[].job_id                             minutes job ID
//	payload[].result                             1:success 2:failure
//	payload[].operate_time                       event operation timestamp (ms)
//	payload[].recording_files[].record_file_id   recording file ID
//	payload[].recording_files[].lang             recording file language
//	payload[].meeting_info.{meeting_id,meeting_code,media_set_type,subject,
//	                        creator{userid,uuid,user_name},meeting_type,
//	                        start_time,end_time}
func registerSmartMinutes() {
	RegisterKey(KeyDef{
		Key:         "smart.minutes",
		Domain:      "smart",
		Description: "智能纪要生成事件",
		JQRootPath:  ".payload",
		ParamsSchema: map[string]ParamDef{
			"meeting_id": {
				Type:        "string",
				Required:    false,
				Description: "仅推送该 meeting_id 的纪要；不传则接收本账号名下所有会议",
				PayloadPath: "0.meeting_info.meeting_id",
			},
		},
		ResolvedOutputSchema: json.RawMessage(`{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "required": ["event", "trace_id", "payload"],
  "properties": {
    "event":    { "type": "string", "const": "smart.minutes" },
    "trace_id": { "type": "string" },
    "payload": {
      "type": "array",
      "items": {
        "type": "object",
        "required": ["job_id", "result", "meeting_info"],
        "properties": {
          "job_id": { "type": "string", "description": "纪要任务 ID" },
          "result": { "type": "integer", "description": "1:成功 2:失败" },
          "operate_time": { "type": "integer", "description": "毫秒级别事件操作时间戳" },
          "recording_files": {
            "type": "array",
            "description": "录制文件列表",
            "items": {
              "type": "object",
              "properties": {
                "record_file_id": { "type": "string", "description": "录制文件 ID" },
                "lang":           { "type": "string", "description": "录制文件语言" }
              }
            }
          },
          "meeting_info": {
            "type": "object",
            "required": ["meeting_id", "meeting_code", "subject"],
            "properties": {
              "meeting_id":     { "type": "string" },
              "meeting_code":   { "type": "string" },
              "media_set_type": { "type": "integer", "description": "媒体集合类型" },
              "subject":        { "type": "string" },
              "creator": {
                "type": "object",
                "properties": {
                  "userid":    { "type": "string" },
                  "uuid":      { "type": "string" },
                  "user_name": { "type": "string" }
                }
              },
              "meeting_type": { "type": "integer" },
              "start_time":   { "type": "integer", "description": "秒级别会议开始时间戳" },
              "end_time":     { "type": "integer", "description": "秒级别会议结束时间戳" }
            }
          }
        }
      }
    }
  }
}`),
	})
}
