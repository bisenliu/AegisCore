package client

import "context"

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
	snapshot, err := r.snapshot(ctx)
	if err != nil {
		return false, nil, err
	}
	return snapshot.send(ctx)
}
