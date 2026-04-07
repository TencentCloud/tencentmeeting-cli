package thttp

import (
	"encoding/json"
	"testing"
)

// jsonSerializer JSON serializer for test
type jsonSerializer struct{}

func (j jsonSerializer) Name() string {
	return "jsonSerializer"
}

func (j jsonSerializer) ContentType() string {
	return "application/json"
}

func (j jsonSerializer) Serialize(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

func (j jsonSerializer) Deserialize(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}

func TestApiResponse_Translate(t *testing.T) {
	resp := Response{
		RawBody: []byte("{\"invitees\": [{\"operator_id_type\": 1,\"operator_id\": \"21\",\"nick_name\": \"史磊\",\"userid\": \"71\"},{\"operator_id\": \"76\",\"userid\": \"13\",\"nick_name\": \"薛秀兰\",\"operator_id_type\": 1},{\"userid\": \"15\",\"operator_id_type\": 1,\"operator_id\": \"38\",\"nick_name\": \"沈杰\"},{\"operator_id_type\": 1,\"userid\": \"32\",\"operator_id\": \"71\",\"nick_name\": \"卢杰\"},{\"operator_id\": \"69\",\"operator_id_type\": 1,\"userid\": \"99\",\"nick_name\": \"魏洋\"}],\"has_remaining\": false,\"next_pos\": 66}"),
	}

	dst := new(struct {
		// 是否还存在受邀成员需要继续查询。
		HasRemaining *bool `json:"has_remaining,omitempty"`
		// 会议邀请的参会者
		Invitees []struct {
			// 用户昵称
			NickName *string `json:"nick_name,omitempty"`
			// 用户的唯一 ID（企业内部请使用企业唯一用户标识；OAuth2.0 鉴权用户请使用 openId）
			Userid *string `json:"userid,omitempty"`
		} `json:"invitees,omitempty"`
		// 当has_remaining为true时，下次查询时输入参数pos的值
		NextPos *int32 `json:"next_pos,omitempty"`
	})

	if err := resp.Translate(dst, WithResponseSerializer(jsonSerializer{})); err != nil {
		t.Errorf("response translate error, \nerr: %s\n", err)
	}
	data, _ := json.Marshal(dst)
	t.Logf("response translate success, \ndst: %s\n", data)
}
