package thttp

import (
	"context"
	"testing"
)

func TestNewClient(t *testing.T) {
	var clt Client
	clt, err := NewClient("www.baidu.com", WithProtocol("https"))
	if err != nil {
		t.Errorf("new client error, \nerr: %v\n", err)
	}

	resp, err := clt.Get(context.Background(), &Request{
		ApiURI: "/",
		Body:   nil,
	})
	if err != nil {
		t.Errorf("new client error, \nerr: %v\n", err)
	}
	var data = resp.RawBody
	t.Logf("new client success, \nresp body: %s\n", data)
}
