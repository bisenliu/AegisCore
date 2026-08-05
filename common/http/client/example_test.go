package client_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/go-resty/resty/v2"

	"github.com/aegiscore/common/http/client"
)

// ExampleSendRequest 展示 query、header 和 JSON body 的基本请求。
func ExampleSendRequest() {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(writer, `{"method":%q,"page":%q,"request_id":%q}`,
			request.Method,
			request.URL.Query().Get("page"),
			request.Header.Get("X-Request-ID"),
		)
	}))
	defer server.Close()

	request := client.NewRequest(server.URL+"/resources", http.MethodPost)
	request.QueryParams["page"] = "2"
	request.Headers["X-Request-ID"] = "request-123"
	request.JSONData = map[string]string{"name": "aegis"}

	success, body, err := request.SendContext(context.Background())
	fmt.Println(success, err == nil)
	fmt.Println(string(body))

	// Output:
	// true true
	// {"method":"POST","page":"2","request_id":"request-123"}
}

// ExampleSendRequest_SendContext 展示调用方取消会在网络请求前终止发送。
func ExampleSendRequest_SendContext() {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	success, body, err := client.NewRequest("https://example.invalid", http.MethodGet).SendContext(ctx)
	fmt.Println(success)
	fmt.Println(body == nil)
	fmt.Println(errors.Is(err, context.Canceled))

	// Output:
	// false
	// true
	// true
}

// ExampleStatusError 展示如何同时检查非 2xx 状态并解析上游 body。
func ExampleStatusError() {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusConflict)
		_, _ = writer.Write([]byte(`{"code":"already_exists"}`))
	}))
	defer server.Close()

	success, body, err := client.NewRequest(server.URL, http.MethodPost).Send()
	var statusErr *client.StatusError
	fmt.Println(success)
	fmt.Println(errors.As(err, &statusErr), statusErr.StatusCode)
	fmt.Println(string(body))

	// Output:
	// false
	// true 409
	// {"code":"already_exists"}
}

// ExampleSendRequest_injectedClient 展示调用方注入并拥有预配置的 Resty client。
func ExampleSendRequest_injectedClient() {
	httpClient := &http.Client{Transport: exampleRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(request.Header.Get("Authorization"))),
			Request:    request,
		}, nil
	})}
	restyClient := resty.NewWithClient(httpClient).SetHeader("Authorization", "Bearer configured-by-integration")

	request := client.NewRequest("https://upstream.example/resource", http.MethodGet)
	request.RestyClient = restyClient
	success, body, err := request.Send()
	fmt.Println(success, err == nil)
	fmt.Println(string(body))

	// Output:
	// true true
	// Bearer configured-by-integration
}

type exampleRoundTripFunc func(*http.Request) (*http.Response, error)

// RoundTrip 将示例请求转发给固定 transport 函数。
func (fn exampleRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}
