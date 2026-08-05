package client

import (
	"context"
	"fmt"
	"maps"
	"net/url"
	"strings"
)

// snapshot 复制并归一化单次发送使用的 request-level 配置。
func (r *SendRequest) snapshot(ctx context.Context) (requestSnapshot, error) {
	if r == nil {
		return requestSnapshot{}, ErrNilRequest
	}
	if ctx == nil {
		return requestSnapshot{}, ErrNilContext
	}

	snapshot := requestSnapshot{
		url:         strings.TrimSpace(r.URL),
		method:      strings.TrimSpace(r.Method),
		queryParams: maps.Clone(r.QueryParams),
		jsonData:    r.JSONData,
		formData:    maps.Clone(r.FormData),
		headers:     maps.Clone(r.Headers),
		proxyURL:    strings.TrimSpace(r.ProxyURL),
		timeout:     r.Timeout,
		restyClient: r.RestyClient,
	}
	if snapshot.timeout == 0 {
		snapshot.timeout = DefaultTimeout
	}
	if err := snapshot.validate(); err != nil {
		return requestSnapshot{}, err
	}
	return snapshot, nil
}

// validate 校验归一化后的请求快照，不执行网络操作。
func (r requestSnapshot) validate() error {
	if r.url == "" {
		return ErrEmptyURL
	}
	if r.method == "" {
		return ErrEmptyMethod
	}
	if r.timeout < 0 {
		return ErrInvalidTimeout
	}
	if r.proxyURL == "" {
		return nil
	}
	if err := validateProxyURL(r.proxyURL); err != nil {
		return err
	}
	if r.restyClient != nil {
		return ErrProxyWithClient
	}
	return nil
}

// validateProxyURL 校验代理地址是带 host 的绝对 HTTP(S) URL。
func validateProxyURL(rawURL string) error {
	proxyURL, err := url.Parse(rawURL)
	if err != nil || proxyURL.Host == "" || (proxyURL.Scheme != "http" && proxyURL.Scheme != "https") {
		if err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidProxyURL, err)
		}
		return ErrInvalidProxyURL
	}
	return nil
}
