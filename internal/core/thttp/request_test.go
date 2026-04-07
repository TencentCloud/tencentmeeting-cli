package thttp

import (
	"fmt"
	"net/url"
	"reflect"
	"testing"
)

func TestApiRequest_GenerateURL(t *testing.T) {
	req := Request{
		ApiURI:      "/v1/meetings/{meeting_id}",
		QueryParams: QueryParams{},
		PathParams:  PathParams{},
	}

	// path 参数
	req.PathParams.Set("meeting_id", fmt.Sprint(12233344434))
	// query 参数
	req.QueryParams.Set("instanceid", fmt.Sprint(1))
	req.QueryParams.Set("operator_id", "userId")
	req.QueryParams.Set("operator_id_type", fmt.Sprint(1))

	expected, _ := url.Parse("https://meeting.tencent.com/v1/meetings/12233344434?instanceid=1&operator_id=user_id&operator_id_type=1")
	if u, err := req.GenerateURL("https://meeting.tencent.com"); err != nil {
		t.Errorf("request generate URL error, err: %v\n", err)
	} else if reflect.DeepEqual(u, expected) {
		t.Errorf("request generate URL doesn't right, \nactual url: %s\nexpected url: %s\n", u.String(), expected.String())
	} else {
		t.Logf("request generate URL success, \nactual url: %s\n", u.String())
	}
}
