package thttp

import (
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"tmeet/internal/core/serializable"
)

type Response struct {
	StatusCode int
	Header     http.Header
	RawBody    []byte

	serializer serializable.Serializable
}

func (resp Response) Write(writer http.ResponseWriter) error {
	writer.WriteHeader(resp.StatusCode)
	for k, vs := range resp.Header {
		for _, v := range vs {
			writer.Header().Add(k, v)
		}
	}
	if _, err := writer.Write(resp.RawBody); err != nil {
		return err
	}
	return nil
}

func (resp Response) Translate(dst interface{}, opts ...ResponseOptionFunc) error {

	copyResp := &Response{
		RawBody:    make([]byte, len(resp.RawBody)),
		serializer: resp.serializer, // The serializer passed in opts takes precedence and will override this.
	}
	copy(copyResp.RawBody, resp.RawBody)

	for _, opt := range opts {
		opt(copyResp)
	}

	v := reflect.ValueOf(dst)
	if v.Kind() != reflect.Pointer || v.IsNil() {
		return fmt.Errorf("response body translate error, dst is nil or dst's type is not the pointer")
	}
	if copyResp.serializer == nil {
		return fmt.Errorf("response body translate error, serializer is nil")
	}
	if err := copyResp.serializer.Deserialize(copyResp.RawBody, dst); err != nil {
		return errors.Join(
			fmt.Errorf("response body translate error, "+
				"body can't be translated by '%s' serializer", copyResp.serializer.Name()),
			err)
	}
	return nil
}

type ResponseOptionFunc func(resp *Response)

func WithResponseSerializer(serializer serializable.Serializable) ResponseOptionFunc {
	return func(response *Response) {
		response.serializer = serializer
	}
}
