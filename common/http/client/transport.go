package client

import (
	"net/http"

	"github.com/go-resty/resty/v2"
)

// defaultRestyClient 只提供无状态默认请求能力，不保存跨请求 cookie，也不配置隐式重试。
var defaultRestyClient = resty.NewWithClient(&http.Client{})

// selectClient 选择调用方 client、package 默认 client 或单次代理 client，并返回对应清理函数。
func (r requestSnapshot) selectClient() (*resty.Client, func()) {
	if r.restyClient != nil {
		return r.restyClient, func() {}
	}
	if r.proxyURL == "" {
		return defaultRestyClient, func() {}
	}

	client := resty.New()
	client.SetProxy(r.proxyURL)
	return client, client.GetClient().CloseIdleConnections
}
