package client

import (
	"time"

	"github.com/go-resty/resty/v2"
)

// SendRequest 封装一次出站 HTTP 请求的参数。
//
// RestyClient、JSONData 中的引用值和 io.Reader 由调用方拥有。发送过程不会修改 client-level 配置，
// 也不会 deep-copy 或重放 body。同一个 SendRequest 只能顺序修改和发送，不能并发使用。
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

// requestSnapshot 是单次发送拥有的浅层配置快照。
type requestSnapshot struct {
	url         string
	method      string
	queryParams map[string]string
	jsonData    any
	formData    map[string]string
	headers     map[string]string
	proxyURL    string
	timeout     time.Duration
	restyClient *resty.Client
}
