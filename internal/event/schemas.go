// Package event — built-in EventKey definitions.
//
// Keys seeded from the Tencent Meeting open-platform contract:
//
//   - meeting.started  — 会议开始 (master)
//     (https://cloud.tencent.com/document/product/1095/51618)
//   - meeting.end      — 会议结束 (master)
//     (https://cloud.tencent.com/document/product/1095/51619)
//   - meeting.asr-push — 实时转写推送 (agent，仅子账号可订阅)
//   - smart.minutes    — 智能纪要生成 (master)
//
// SubscribeRole 决定该事件可被哪类账号订阅（master 主账号 / agent 子账号 /
// none 不限），由 `event consume` 在订阅前校验、并把 agent_open_id 带到上游。
//
// The Tencent Meeting webhook envelope is:
//
//	{
//	  "event":    "<key>",
//	  "trace_id": "<id>",
//	  "payload":  [ { "operate_time": ..., "operator": {...}, "meeting_info": {...} } ]
//	}
//
// 关于 payload 的形态（已与服务端确认）：
//
//   - 协议层 payload 是 array<object>，但对 meeting.started / meeting.end
//     这两个事件，服务端契约保证数组长度恒为 1 —— 数组形态仅是为未来批量
//     推送预留，当前未启用。
//   - 因此 PayloadPath 直接寻址 payload[0]（写法 "0.meeting_info.meeting_id"）
//     是安全的：不会出现"漏过非首元素"的 false negative。
//   - extractScalarString 已支持非负整数下标段（详见 params.go 的 stepInto），
//     这套 PayloadPath 即可直接驱动 L2 过滤。
//
// 事件名注意 51619 的真实键是 "meeting.end"（不是 "meeting.ended"），文档
// 示例中明确给出。
package event

import "encoding/json"

func init() {
	// meeting.started — 会议开始事件
	// 文档: https://cloud.tencent.com/document/product/1095/51618
	//
	// 字段含义（节选自官方示例）：
	//   payload[].operate_time       毫秒级事件操作时间戳
	//   payload[].operator.userid    事件操作者 id（同企业用户为 userid，OAuth 用户为 openId，rooms 为 roomsId）
	//   payload[].operator.open_id   开放平台 OpenID
	//   payload[].operator.uuid      用户身份 ID
	//   payload[].operator.user_name 事件操作者名称
	//   payload[].operator.ms_open_id
	//   payload[].operator.instance_id 用户终端设备类型
	//   payload[].meeting_info.meeting_id           会议 ID
	//   payload[].meeting_info.meeting_code         会议 code
	//   payload[].meeting_info.subject              会议主题
	//   payload[].meeting_info.creator.{userid,open_id,uuid,user_name,ms_open_id,instance_id}
	//   payload[].meeting_info.meeting_type         0 一次性 / 1 周期性 / 2 微信专属 / 4 rooms 投屏 / 5 个人会议号
	//   payload[].meeting_info.start_time           秒级开始时间戳
	//   payload[].meeting_info.end_time             秒级结束时间戳
	//   payload[].meeting_info.meeting_create_mode  0 普通 / 1 快速
	//   payload[].meeting_info.meeting_create_from  0 空 / 1 客户端 / 2 web / 3 企微 / 4 微信 / 5 outlook / 6 restapi / 7 腾讯文档 / 8 Rooms 智能录制
	//
	// PayloadPath 直接寻址 payload[0]：服务端契约保证该数组长度恒为 1
	// （扩展预留，当前未启用），extractScalarString 已支持数组下标段
	// （详见 params.go 的 stepInto 注释）。
	RegisterKey(KeyDef{
		Key:         "meeting.started",
		Domain:      "meeting",
		Description: "会议开始事件",
		JQRootPath:  ".payload",
		// 主账号事件：仅主账号可订阅。
		SubscribeRole: SubscribeRoleMaster,
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
      "items": {
        "type": "object",
        "required": ["operate_time", "operator", "meeting_info"],
        "properties": {
          "operate_time": { "type": "integer", "description": "毫秒级别事件操作时间戳" },
          "operator": {
            "type": "object",
            "properties": {
              "userid":      { "type": "string", "description": "事件操作者 id" },
              "open_id":     { "type": "string" },
              "uuid":        { "type": "string", "description": "用户身份 ID" },
              "user_name":   { "type": "string" },
              "ms_open_id":  { "type": "string" },
              "instance_id": { "type": "string", "description": "用户的终端设备类型" }
            }
          },
          "meeting_info": {
            "type": "object",
            "required": ["meeting_id", "meeting_code", "subject"],
            "properties": {
              "meeting_id":   { "type": "string" },
              "meeting_code": { "type": "string" },
              "subject":      { "type": "string" },
              "creator": {
                "type": "object",
                "properties": {
                  "userid":      { "type": "string" },
                  "open_id":     { "type": "string" },
                  "uuid":        { "type": "string" },
                  "user_name":   { "type": "string" },
                  "ms_open_id":  { "type": "string" },
                  "instance_id": { "type": "string" }
                }
              },
              "meeting_type":        { "type": "integer", "description": "0:一次性 1:周期性 2:微信专属 4:rooms投屏 5:个人会议号" },
              "start_time":          { "type": "integer", "description": "秒级别会议开始时间戳" },
              "end_time":            { "type": "integer", "description": "秒级别会议结束时间戳" },
              "meeting_create_mode": { "type": "integer", "description": "0:普通 1:快速" },
              "meeting_create_from": { "type": "integer", "description": "0:空 1:客户端 2:web 3:企微 4:微信 5:outlook 6:restapi 7:腾讯文档 8:Rooms智能录制" }
            }
          }
        }
      }
    }
  }
}`),
	})

	// meeting.end — 会议结束事件
	// 文档: https://cloud.tencent.com/document/product/1095/51619
	//
	// 注意：腾讯会议官方文档中事件名为 "meeting.end"（不是 "meeting.ended"）。
	//
	// 相对 meeting.started，payload[] 元素多一个顶层字段：
	//   payload[].meeting_end_type
	//     0: 主动结束会议
	//     1: 最后一个参会用户离开且超过预定结束时间
	//     2: 会议中无人且超过预定结束时间
	//     3: 会议中无人且未到预定结束时间
	RegisterKey(KeyDef{
		Key:         "meeting.end",
		Domain:      "meeting",
		Description: "会议结束事件",
		JQRootPath:  ".payload",
		// 主账号事件：仅主账号可订阅。
		SubscribeRole: SubscribeRoleMaster,
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
      "items": {
        "type": "object",
        "required": ["operate_time", "operator", "meeting_info"],
        "properties": {
          "operate_time": { "type": "integer", "description": "毫秒级别事件操作时间戳" },
          "operator": {
            "type": "object",
            "properties": {
              "userid":      { "type": "string" },
              "open_id":     { "type": "string" },
              "uuid":        { "type": "string" },
              "user_name":   { "type": "string" },
              "ms_open_id":  { "type": "string" },
              "instance_id": { "type": "string" }
            }
          },
          "meeting_end_type": {
            "type": "integer",
            "description": "0:主动结束 1:最后一人离开且超时 2:无人且超时 3:无人且未到结束时间"
          },
          "meeting_info": {
            "type": "object",
            "required": ["meeting_id", "meeting_code", "subject"],
            "properties": {
              "meeting_id":   { "type": "string" },
              "meeting_code": { "type": "string" },
              "subject":      { "type": "string" },
              "creator": {
                "type": "object",
                "properties": {
                  "userid":      { "type": "string" },
                  "open_id":     { "type": "string" },
                  "uuid":        { "type": "string" },
                  "user_name":   { "type": "string" },
                  "ms_open_id":  { "type": "string" },
                  "instance_id": { "type": "string" }
                }
              },
              "meeting_type":        { "type": "integer" },
              "start_time":          { "type": "integer" },
              "end_time":            { "type": "integer" },
              "meeting_create_mode": { "type": "integer" },
              "meeting_create_from": { "type": "integer" }
            }
          }
        }
      }
    }
  }
}`),
	})

	// meeting.asr-push — 实时转写推送事件
	//
	// 子账号（agent）事件：仅配置了 agent 的会话可订阅。`event consume`
	// 在 fork bus 之前校验 agent 是否存在，并把 agent_open_id 一路带到上游
	// SubscribeReq（见 cmd/event/consume.go 与 wsspb.EncodeSubscribeReq 的 TODO）。
	//
	// payload 形态（已与服务端约定，沿用 meeting.* 的 array<object> 约定，
	// 当前数组长度恒为 1）：
	//   payload[].asr_content[]            实时转写分句数组
	//   payload[].asr_content[].speaker.{userid,open_id,ms_open_id,nickname}
	//   payload[].asr_content[].speech_time 该分句的语音时间（毫秒）
	//   payload[].asr_content[].content.text       转写文本
	//   payload[].asr_content[].content.language   文本语言
	//   payload[].asr_content[].content.translate[] 翻译结果数组
	//   payload[].sid                      转写会话 ID
	//   payload[].meeting_info.{meeting_id,meeting_code,subject,...}
	RegisterKey(KeyDef{
		Key:           "meeting.asr-push",
		Domain:        "meeting",
		Description:   "实时转写推送事件",
		JQRootPath:    ".payload",
		SubscribeRole: SubscribeRoleAgent,
		ParamsSchema: map[string]ParamDef{
			"meeting_id": {
				Type:        "string",
				Required:    false,
				Description: "仅推送该 meeting_id 的转写；不传则接收本账号名下所有会议",
				PayloadPath: "0.meeting_info.meeting_id",
			},
		},
		ResolvedOutputSchema: json.RawMessage(`{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "required": ["event", "trace_id", "payload"],
  "properties": {
    "event":    { "type": "string", "const": "meeting.asr-push" },
    "trace_id": { "type": "string" },
    "payload": {
      "type": "array",
      "items": {
        "type": "object",
        "required": ["asr_content", "meeting_info"],
        "properties": {
          "sid": { "type": "string", "description": "转写会话 ID" },
          "asr_content": {
            "type": "array",
            "description": "实时转写分句数组",
            "items": {
              "type": "object",
              "properties": {
                "speaker": {
                  "type": "object",
                  "properties": {
                    "userid":     { "type": "string" },
                    "open_id":    { "type": "string" },
                    "ms_open_id": { "type": "string" },
                    "nickname":   { "type": "string" }
                  }
                },
                "speech_time": { "type": "integer", "description": "该分句语音时间（毫秒）" },
                "content": {
                  "type": "object",
                  "properties": {
                    "text":     { "type": "string", "description": "转写文本" },
                    "language": { "type": "string", "description": "文本语言" },
                    "translate": {
                      "type": "array",
                      "description": "翻译结果数组",
                      "items": { "type": "string" }
                    }
                  }
                }
              }
            }
          },
          "meeting_info": {
            "type": "object",
            "required": ["meeting_id", "meeting_code", "subject"],
            "properties": {
              "meeting_id":   { "type": "string" },
              "meeting_code": { "type": "string" },
              "subject":      { "type": "string" },
              "creator": {
                "type": "object",
                "properties": {
                  "userid":      { "type": "string" },
                  "open_id":     { "type": "string" },
                  "uuid":        { "type": "string" },
                  "user_name":   { "type": "string" },
                  "ms_open_id":  { "type": "string" },
                  "instance_id": { "type": "string" }
                }
              },
              "meeting_type": { "type": "integer" },
              "start_time":   { "type": "integer" },
              "end_time":     { "type": "integer" }
            }
          }
        }
      }
    }
  }
}`),
	})

	// smart.minutes — 智能纪要生成事件
	//
	// 主账号事件：仅主账号可订阅。
	//
	// payload 形态（沿用 array<object>，当前数组长度恒为 1）。job_id 与
	// result 描述本次纪要任务的整体结果，置于 payload[] 元素内，确保经
	// RawEvent（仅承载 event/trace_id/payload）管道下发时不丢字段：
	//   payload[].job_id                   纪要任务 ID
	//   payload[].result                   1:成功 2:失败
	//   payload[].operate_time             毫秒级事件操作时间戳
	//   payload[].recording_files[].record_file_id 录制文件 ID
	//   payload[].recording_files[].lang   录制文件语言
	//   payload[].meeting_info.{meeting_id,meeting_code,media_set_type,subject,
	//                           creator{userid,uuid,user_name},meeting_type,
	//                           start_time,end_time}
	RegisterKey(KeyDef{
		Key:           "smart.minutes",
		Domain:        "smart",
		Description:   "智能纪要生成事件",
		JQRootPath:    ".payload",
		SubscribeRole: SubscribeRoleMaster,
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
