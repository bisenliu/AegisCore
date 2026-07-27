// Package client 提供基于 Resty 的业务中立出站 HTTP 请求能力。
package client

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
)

const DefaultTimeout = 60 * time.Second

var (
	ErrNilRequest      = errors.New("request must not be nil")
	ErrNilContext      = errors.New("request context must not be nil")
	ErrEmptyURL        = errors.New("request URL must not be empty")
	ErrEmptyMethod     = errors.New("request method must not be empty")
	ErrInvalidTimeout  = errors.New("request timeout must not be negative")
	ErrInvalidProxyURL = errors.New("proxy URL must be an absolute HTTP(S) URL")
	ErrProxyWithClient = errors.New("proxy URL cannot be combined with an injected Resty client")
)

// defaultRestyClient 只提供无状态默认请求能力，不保存跨请求 cookie，也不配置隐式重试。
var defaultRestyClient = resty.NewWithClient(&http.Client{})

// StatusError 表示上游返回了非 2xx HTTP 状态。
// Body 不进入错误文本，调用方可使用 Send 返回的 body 按具体协议解析。
type StatusError struct {
	StatusCode int
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("unexpected HTTP status %d", e.StatusCode)
}

// SendRequest 封装一次出站 HTTP 请求的参数。
// RestyClient 由调用方拥有，发送过程不会修改其 client-level 配置。
type SendRequest struct {
	URL         string
	Method      string
	QueryParams map[string]string
	JSONData    any
	FormData    map[string]string
	Headers     map[string]string
	ProxyURL    string
	Timeout     time.Duration
	RestyClient *resty.Client
}

// NewRequest 创建带默认 timeout 和空参数集合的请求。
func NewRequest(rawURL string, method string) *SendRequest {
	return &SendRequest{
		URL:         rawURL,
		Method:      method,
		QueryParams: make(map[string]string),
		FormData:    make(map[string]string),
		Headers:     make(map[string]string),
		Timeout:     DefaultTimeout,
	}
}

// Send 使用 background context 发送请求。
func (r *SendRequest) Send() (bool, []byte, error) {
	return r.SendContext(context.Background())
}

// SendContext 使用调用方 context 发送请求。
// 2xx 响应返回 success=true；其他状态返回 body 和可检查的 *StatusError。
func (r *SendRequest) SendContext(ctx context.Context) (bool, []byte, error) {
	if err := r.validate(ctx); err != nil {
		return false, nil, err
	}

	client, closeIdleConnections := r.client()
	defer closeIdleConnections()

	timeout := r.Timeout
	if timeout == 0 {
		timeout = DefaultTimeout
	}
	requestCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	request := client.R().
		SetContext(requestCtx).
		SetQueryParams(r.QueryParams).
		SetHeaders(r.Headers)
	if len(r.FormData) > 0 {
		request.SetFormData(r.FormData)
	} else if r.JSONData != nil {
		request.SetBody(r.JSONData)
	}

	response, err := request.Execute(r.Method, r.URL)
	if err != nil {
		return false, nil, fmt.Errorf("send HTTP request: %w", err)
	}
	body := response.Body()
	if !response.IsSuccess() {
		return false, body, &StatusError{StatusCode: response.StatusCode()}
	}
	return true, body, nil
}

func (r *SendRequest) validate(ctx context.Context) error {
	if r == nil {
		return ErrNilRequest
	}
	if ctx == nil {
		return ErrNilContext
	}
	if strings.TrimSpace(r.URL) == "" {
		return ErrEmptyURL
	}
	if strings.TrimSpace(r.Method) == "" {
		return ErrEmptyMethod
	}
	if r.Timeout < 0 {
		return ErrInvalidTimeout
	}
	if strings.TrimSpace(r.ProxyURL) != "" {
		if _, err := parseProxyURL(r.ProxyURL); err != nil {
			return err
		}
		if r.RestyClient != nil {
			return ErrProxyWithClient
		}
	}
	return nil
}

func (r *SendRequest) client() (*resty.Client, func()) {
	if r.RestyClient != nil {
		return r.RestyClient, func() {}
	}
	if strings.TrimSpace(r.ProxyURL) == "" {
		return defaultRestyClient, func() {}
	}

	client := resty.New()
	client.SetProxy(r.ProxyURL)
	return client, client.GetClient().CloseIdleConnections
}

func parseProxyURL(rawURL string) (*url.URL, error) {
	proxyURL, err := url.Parse(rawURL)
	if err != nil || proxyURL.Host == "" || (proxyURL.Scheme != "http" && proxyURL.Scheme != "https") {
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidProxyURL, err)
		}
		return nil, ErrInvalidProxyURL
	}
	return proxyURL, nil
}
