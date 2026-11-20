package client

import (
	"context"
	"fmt"
)

// send 使用快照构造并执行单次 Resty 请求，再归一化响应结果。
func (r requestSnapshot) send(ctx context.Context) (bool, []byte, error) {
	client, closeIdleConnections := r.selectClient()
	defer closeIdleConnections()

	requestCtx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	request := client.R().
		SetContext(requestCtx).
		SetQueryParams(r.queryParams).
		SetHeaders(r.headers)
	if len(r.formData) > 0 {
		request.SetFormData(r.formData)
	} else if r.jsonData != nil {
		request.SetBody(r.jsonData)
	}

	response, err := request.Execute(r.method, r.url)
	if err != nil {
		return false, nil, fmt.Errorf("send HTTP request: %w", err)
	}
	body := response.Body()
	if !response.IsSuccess() {
		return false, body, &StatusError{StatusCode: response.StatusCode()}
	}
	return true, body, nil
}
