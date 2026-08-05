package client

import (
	"errors"
	"fmt"
)

var (
	ErrNilRequest      = errors.New("request must not be nil")
	ErrNilContext      = errors.New("request context must not be nil")
	ErrEmptyURL        = errors.New("request URL must not be empty")
	ErrEmptyMethod     = errors.New("request method must not be empty")
	ErrInvalidTimeout  = errors.New("request timeout must not be negative")
	ErrInvalidProxyURL = errors.New("proxy URL must be an absolute HTTP(S) URL")
	ErrProxyWithClient = errors.New("proxy URL cannot be combined with an injected Resty client")
)

// StatusError 表示上游返回了非 2xx HTTP 状态。
// Body 不进入错误文本，调用方可使用 Send 返回的 body 按具体协议解析。
type StatusError struct {
	StatusCode int
}

// Error 返回不包含上游响应 body 的稳定状态错误文本。
func (e *StatusError) Error() string {
	return fmt.Sprintf("unexpected HTTP status %d", e.StatusCode)
}
