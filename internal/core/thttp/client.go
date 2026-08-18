package thttp

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"
	"tmeet/internal/core/serializable"
)

// Client wraps the standard REST client interface.
type Client interface {
	Get(ctx context.Context, req *Request, opts ...RequestOptionFunc) (resp *Response, err error)
	Post(ctx context.Context, req *Request, opts ...RequestOptionFunc) (resp *Response, err error)
	Put(ctx context.Context, req *Request, opts ...RequestOptionFunc) (resp *Response, err error)
	Delete(ctx context.Context, req *Request, opts ...RequestOptionFunc) (resp *Response, err error)
}

// Authentication provides authentication capability.
type Authentication interface {
	AuthHeader(httpReq *http.Request) error
}

type client struct {
	clt      *http.Client
	host     string
	protocol string

	serializer serializable.Serializable
}

type ClientOptionFunc func(c *client)

// DefaultHttpClient is the default HTTP client.
var DefaultHttpClient = &http.Client{
	Transport: &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          200,
		IdleConnTimeout:       50 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		MaxIdleConnsPerHost:   100,
		MaxConnsPerHost:       200,
		ExpectContinueTimeout: time.Second,
	},
	CheckRedirect: nil,
	Jar:           nil,
	Timeout:       3 * time.Second,
}

func WithSerializer(serializer serializable.Serializable) ClientOptionFunc {
	return func(c *client) {
		c.serializer = serializer
	}
}

func WithProtocol(protocol string) ClientOptionFunc {
	return func(c *client) {
		c.protocol = protocol
	}
}

func WithClient(httpClt *http.Client) ClientOptionFunc {
	return func(c *client) {
		c.clt = httpClt
	}
}

func NewClient(host string, opts ...ClientOptionFunc) (Client, error) {
	clt := &client{
		clt:      DefaultHttpClient,
		host:     host,
		protocol: DefaultHttpsProtocol,
	}

	for _, opt := range opts {
		opt(clt)
	}

	if clt.host == "" {
		return nil, errors.New("http client's host can't be empty")
	}

	if _, err := url.Parse(fmt.Sprintf("%s://%s", clt.protocol, clt.host)); err != nil {
		return nil, errors.New("http client's protocol or host is illegal")
	}

	return clt, nil
}

func (c *client) Get(ctx context.Context, req *Request,
	opts ...RequestOptionFunc) (*Response, error) {
	// Handle nil value.
	if req == nil {
		return nil, fmt.Errorf("http client do request error, 'req' is nil")
	}

	return c.doRequest(ctx, req, http.MethodGet, opts...)
}

func (c *client) Put(ctx context.Context, req *Request,
	opts ...RequestOptionFunc) (*Response, error) {
	// Handle nil value.
	if req == nil {
		return nil, fmt.Errorf("http client do request error, 'req' is nil")
	}

	return c.doRequest(ctx, req, http.MethodPut, opts...)
}

func (c *client) Delete(ctx context.Context, req *Request,
	opts ...RequestOptionFunc) (*Response, error) {
	// Handle nil value.
	if req == nil {
		return nil, fmt.Errorf("http client do request error, 'req' is nil")
	}

	return c.doRequest(ctx, req, http.MethodDelete, opts...)
}

func (c *client) Post(ctx context.Context, req *Request,
	opts ...RequestOptionFunc) (*Response, error) {
	// Handle nil value.
	if req == nil {
		return nil, fmt.Errorf("http client do request error, 'req' is nil")
	}

	return c.doRequest(ctx, req, http.MethodPost, opts...)
}

func (c *client) doRequest(ctx context.Context, req *Request, method string,
	opts ...RequestOptionFunc) (*Response, error) {

	for _, opt := range opts {
		opt(req)
	}

	// Get the serializer; the one configured in the current request takes precedence.
	serializer := c.serializer
	if req.serializer != nil {
		serializer = req.serializer
	}

	// Serialize the request body.
	var bodyReader io.Reader
	if req.Body != nil {
		if reader, ok := req.Body.(*bytes.Buffer); ok {
			bodyReader = reader
		} else if serializer != nil {
			data, err := serializer.Serialize(req.Body)
			if err != nil {
				return nil, err
			}
			bodyReader = bytes.NewBuffer(data)
		} else if reader, ok := req.Body.(io.Reader); ok {
			bodyReader = reader
		}
	}

	// Generate the URL.
	u, err := req.GenerateURL(fmt.Sprintf("%s://%s", c.protocol, c.host))
	if err != nil {
		return nil, err
	}
	// Build the native HTTP request.
	httpReq, err := http.NewRequestWithContext(ctx, method, u.String(), bodyReader)
	if err != nil {
		return nil, err
	}

	// Set request headers.
	for k, v := range req.header {
		httpReq.Header[k] = v
	}
	// Add authentication headers.
	if req.authenticators != nil {
		for _, authenticator := range req.authenticators {
			if err = authenticator.AuthHeader(httpReq); err != nil {
				return nil, err
			}
		}
	}

	// Get the native HTTP client.
	if c.clt == nil {
		c.clt = http.DefaultClient
	}

	// Send the request.
	httpRsp, err := c.clt.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer func() {
		if httpRsp.Body != nil {
			_ = httpRsp.Body.Close()
		}
	}()

	// Read the response.
	respBody, err := io.ReadAll(httpRsp.Body)
	if err != nil {
		return nil, err
	}

	// Wrap and return the response.
	return &Response{
		StatusCode: httpRsp.StatusCode,
		Header:     httpRsp.Header,
		RawBody:    respBody,
		serializer: serializer,
	}, nil
}
