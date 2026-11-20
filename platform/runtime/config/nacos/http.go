package nacos

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// newRequest 构造 Nacos API 请求并统一设置 User-Agent 和可选 bearer token。
func (l *v3Loader) newRequest(
	ctx context.Context,
	method string,
	endpoint *url.URL,
	body io.Reader,
	token string,
) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, endpoint.String(), body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", clientUserAgent)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return req, nil
}

// doJSON 执行 HTTP 请求并将成功响应解码到 target。
// 响应体大小受 maxResponseBytes 约束，避免异常 Nacos 响应导致无界内存读取。
func (l *v3Loader) doJSON(req *http.Request, target any) error {
	resp, err := l.client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	payload, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	if len(payload) > maxResponseBytes {
		return fmt.Errorf("response exceeds %d bytes", maxResponseBytes)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("unexpected HTTP status %d", resp.StatusCode)
	}
	if err := json.Unmarshal(payload, target); err != nil {
		return fmt.Errorf("decode JSON response: %w", err)
	}
	return nil
}

// safeMessage 清洗远端错误消息，确保错误文本非空且长度有界。
func safeMessage(message string) string {
	message = strings.TrimSpace(message)
	if message == "" {
		return "unspecified error"
	}
	const maxMessageBytes = 512
	if len(message) > maxMessageBytes {
		return message[:maxMessageBytes] + "..."
	}
	return message
}
