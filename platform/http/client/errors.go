package client

import (
	"errors"
	"fmt"
)

var (
	// ErrNilRequest 请求内容为空，无法对外发起网络请求。
	ErrNilRequest = errors.New("request must not be nil")
	// ErrNilContext 缺少超时或取消控制等生命周期信息，无法安全发送网络请求。
	ErrNilContext = errors.New("request context must not be nil")
	// ErrEmptyURL 目标访问地址为空，无法确定请求目标。
	ErrEmptyURL = errors.New("request URL must not be empty")
	// ErrEmptyMethod 请求操作方式（如 GET、POST）未指定，无法发起请求。
	ErrEmptyMethod = errors.New("request method must not be empty")
	// ErrInvalidTimeout 网络等待超时时间不能设置为负数。
	ErrInvalidTimeout = errors.New("request timeout must not be negative")
	// ErrInvalidProxyURL 网络代理地址不合法，必须是以 http:// 或 https:// 开头的完整访问网址。
	ErrInvalidProxyURL = errors.New("proxy URL must be an absolute HTTP(S) URL")
	// ErrProxyWithClient 已经直接传入了自定义网络客户端，不能再重复指定网络代理地址，避免配置冲突。
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
