package client

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/stretchr/testify/require"
)

func TestNewRequestDefaults(t *testing.T) {
	request := NewRequest("https://example.com", http.MethodGet)

	require.Equal(t, "https://example.com", request.URL)
	require.Equal(t, http.MethodGet, request.Method)
	require.Equal(t, DefaultTimeout, request.Timeout)
	require.NotNil(t, request.QueryParams)
	require.NotNil(t, request.FormData)
	require.NotNil(t, request.Headers)
	require.Nil(t, request.JSONData)
}

func TestSnapshotCopiesAndNormalizesRequestConfiguration(t *testing.T) {
	request := &SendRequest{
		URL:         "  https://example.com/resources  ",
		Method:      "\tPATCH\n",
		QueryParams: map[string]string{"page": "1"},
		FormData:    map[string]string{"name": "aegis"},
		Headers:     map[string]string{"X-Request-ID": "request-123"},
		ProxyURL:    "  http://proxy.example.com  ",
	}

	snapshot, err := request.snapshot(context.Background())
	require.NoError(t, err)
	require.Equal(t, "https://example.com/resources", snapshot.url)
	require.Equal(t, http.MethodPatch, snapshot.method)
	require.Equal(t, "http://proxy.example.com", snapshot.proxyURL)
	require.Equal(t, DefaultTimeout, snapshot.timeout)

	request.QueryParams["page"] = "2"
	request.FormData["name"] = "changed"
	request.Headers["X-Request-ID"] = "changed"
	require.Equal(t, "1", snapshot.queryParams["page"])
	require.Equal(t, "aegis", snapshot.formData["name"])
	require.Equal(t, "request-123", snapshot.headers["X-Request-ID"])

	require.Equal(t, "  https://example.com/resources  ", request.URL)
	require.Equal(t, "\tPATCH\n", request.Method)
	require.Equal(t, "  http://proxy.example.com  ", request.ProxyURL)
}

func TestSendContextUsesTrimmedURLAndMethod(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		require.Equal(t, http.MethodGet, request.Method)
		_, err := writer.Write([]byte("ok"))
		require.NoError(t, err)
	}))
	defer server.Close()

	request := NewRequest("  "+server.URL+"  ", "\tGET\n")
	success, body, err := request.Send()
	require.NoError(t, err)
	require.True(t, success)
	require.Equal(t, "ok", string(body))
}

func TestSendContextReusesRequestSequentiallyWithoutRestyRequestState(t *testing.T) {
	received := make(chan string, 2)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		received <- request.URL.Query().Get("page") + ":" + request.Header.Get("X-Request-ID")
		writer.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	request := NewRequest(server.URL, http.MethodGet)
	request.QueryParams["page"] = "1"
	request.Headers["X-Request-ID"] = "first"
	firstSuccess, _, firstErr := request.Send()

	request.QueryParams["page"] = "2"
	request.Headers["X-Request-ID"] = "second"
	secondSuccess, _, secondErr := request.Send()

	require.NoError(t, firstErr)
	require.NoError(t, secondErr)
	require.True(t, firstSuccess)
	require.True(t, secondSuccess)
	require.Equal(t, "1:first", <-received)
	require.Equal(t, "2:second", <-received)
}

func TestSendContextUsesDefaultTimeoutForZeroValue(t *testing.T) {
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		deadline, ok := request.Context().Deadline()
		require.True(t, ok)
		remaining := time.Until(deadline)
		require.Greater(t, remaining, DefaultTimeout-time.Second)
		require.LessOrEqual(t, remaining, DefaultTimeout)
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader("ok")),
			Request:    request,
		}, nil
	})
	request := &SendRequest{
		URL:         "https://example.com",
		Method:      http.MethodGet,
		RestyClient: resty.NewWithClient(&http.Client{Transport: transport}),
	}

	success, body, err := request.SendContext(context.Background())
	require.NoError(t, err)
	require.True(t, success)
	require.Equal(t, "ok", string(body))
}

func TestSendContextEncodesQueryHeadersAndJSON(t *testing.T) {
	type requestBody struct {
		Name string `json:"name"`
	}

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		require.Equal(t, http.MethodPatch, request.Method)
		require.Equal(t, "/resources", request.URL.Path)
		require.Equal(t, "existing", request.URL.Query().Get("source"))
		require.Equal(t, "2", request.URL.Query().Get("page"))
		require.Equal(t, "request-123", request.Header.Get("X-Request-ID"))
		require.Equal(t, "application/json", request.Header.Get("Content-Type"))

		var body requestBody
		require.NoError(t, json.NewDecoder(request.Body).Decode(&body))
		require.Equal(t, requestBody{Name: "aegis"}, body)
		writer.WriteHeader(http.StatusCreated)
		_, err := writer.Write([]byte(`{"id":"resource-1"}`))
		require.NoError(t, err)
	}))
	defer server.Close()

	request := NewRequest(server.URL+"/resources?source=existing", http.MethodPatch)
	request.QueryParams["page"] = "2"
	request.Headers["X-Request-ID"] = "request-123"
	request.JSONData = requestBody{Name: "aegis"}

	success, body, err := request.SendContext(context.Background())
	require.NoError(t, err)
	require.True(t, success)
	require.JSONEq(t, `{"id":"resource-1"}`, string(body))
}

func TestSendContextUsesFormBeforeJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		require.Equal(t, "application/x-www-form-urlencoded", request.Header.Get("Content-Type"))
		payload, err := io.ReadAll(request.Body)
		require.NoError(t, err)
		form, err := url.ParseQuery(string(payload))
		require.NoError(t, err)
		require.Equal(t, "aegis", form.Get("name"))
		require.Empty(t, form.Get("ignored"))
		writer.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	request := NewRequest(server.URL, http.MethodPost)
	request.JSONData = map[string]any{"ignored": true}
	request.FormData["name"] = "aegis"

	success, body, err := request.Send()
	require.NoError(t, err)
	require.True(t, success)
	require.Empty(t, body)
}

func TestSendContextReturnsStatusErrorAndResponseBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusUnprocessableEntity)
		_, err := writer.Write([]byte(`{"code":"invalid_input"}`))
		require.NoError(t, err)
	}))
	defer server.Close()

	success, body, err := NewRequest(server.URL, http.MethodGet).Send()
	require.False(t, success)
	require.JSONEq(t, `{"code":"invalid_input"}`, string(body))
	var statusErr *StatusError
	require.ErrorAs(t, err, &statusErr)
	require.Equal(t, http.StatusUnprocessableEntity, statusErr.StatusCode)
	require.NotContains(t, err.Error(), "invalid_input")
}

func TestSendContextValidatesInputBeforeSending(t *testing.T) {
	tests := []struct {
		name    string
		request *SendRequest
		wantErr error
	}{
		{name: "nil request", request: nil, wantErr: ErrNilRequest},
		{name: "empty URL", request: NewRequest("", http.MethodGet), wantErr: ErrEmptyURL},
		{name: "empty method", request: NewRequest("https://example.com", ""), wantErr: ErrEmptyMethod},
		{name: "negative timeout", request: &SendRequest{URL: "https://example.com", Method: http.MethodGet, Timeout: -time.Second}, wantErr: ErrInvalidTimeout},
		{name: "invalid proxy scheme", request: &SendRequest{URL: "https://example.com", Method: http.MethodGet, ProxyURL: "socks5://proxy.example.com"}, wantErr: ErrInvalidProxyURL},
		{name: "proxy without host", request: &SendRequest{URL: "https://example.com", Method: http.MethodGet, ProxyURL: "http:///proxy"}, wantErr: ErrInvalidProxyURL},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			success, body, err := test.request.SendContext(context.Background())
			require.False(t, success)
			require.Nil(t, body)
			require.ErrorIs(t, err, test.wantErr)
		})
	}
}

func TestSendContextRejectsNilContext(t *testing.T) {
	//nolint:staticcheck // 此处显式验证公开 API 对 nil context 的防御。
	success, body, err := NewRequest("https://example.com", http.MethodGet).SendContext(nil)
	require.False(t, success)
	require.Nil(t, body)
	require.ErrorIs(t, err, ErrNilContext)
}

func TestSendContextReportsJSONEncodingError(t *testing.T) {
	request := NewRequest("https://example.com", http.MethodPost)
	request.JSONData = make(chan int)

	success, body, err := request.Send()
	require.False(t, success)
	require.Nil(t, body)
	require.ErrorContains(t, err, "unsupported 'Body' type/value")
}

func TestSendContextHonorsTimeout(t *testing.T) {
	requestStarted := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
		close(requestStarted)
		<-request.Context().Done()
	}))
	defer server.Close()

	request := NewRequest(server.URL, http.MethodGet)
	request.Timeout = 20 * time.Millisecond

	success, body, err := request.Send()
	require.False(t, success)
	require.Nil(t, body)
	require.Error(t, err)
	select {
	case <-requestStarted:
	default:
		t.Fatal("request did not reach test server")
	}
}

func TestSendContextHonorsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	success, body, err := NewRequest("https://example.com", http.MethodGet).SendContext(ctx)
	require.False(t, success)
	require.Nil(t, body)
	require.ErrorIs(t, err, context.Canceled)
}

func TestSendContextKeepsDefaultTLSVerification(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, err := writer.Write([]byte("trusted"))
		require.NoError(t, err)
	}))
	defer server.Close()

	request := NewRequest(server.URL, http.MethodGet)
	success, body, err := request.Send()
	require.False(t, success)
	require.Nil(t, body)
	require.Error(t, err)

	request.RestyClient = resty.NewWithClient(server.Client())
	success, body, err = request.Send()
	require.NoError(t, err)
	require.True(t, success)
	require.Equal(t, "trusted", string(body))
}

func TestSendContextUsesProxyURL(t *testing.T) {
	proxy := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		require.Equal(t, "upstream.invalid", request.URL.Host)
		require.Equal(t, "/through-proxy", request.URL.Path)
		_, err := writer.Write([]byte("proxied"))
		require.NoError(t, err)
	}))
	defer proxy.Close()

	request := NewRequest("http://upstream.invalid/through-proxy", http.MethodGet)
	request.ProxyURL = "  " + proxy.URL + "\t"

	success, body, err := request.Send()
	require.NoError(t, err)
	require.True(t, success)
	require.Equal(t, "proxied", string(body))
}

func TestSendContextDoesNotModifyInjectedClient(t *testing.T) {
	const originalTimeout = 5 * time.Second
	transport := &roundTripStub{roundTrip: func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader("ok")),
			Request:    request,
		}, nil
	}}
	httpClient := &http.Client{Transport: transport, Timeout: originalTimeout}
	request := NewRequest("https://example.com", http.MethodGet)
	request.RestyClient = resty.NewWithClient(httpClient)
	request.Timeout = time.Second

	success, body, err := request.Send()
	require.NoError(t, err)
	require.True(t, success)
	require.Equal(t, "ok", string(body))
	require.Equal(t, originalTimeout, httpClient.Timeout)
	require.Same(t, transport, httpClient.Transport)
}

func TestSendContextRejectsProxyWithInjectedRestyClient(t *testing.T) {
	request := NewRequest("https://example.com", http.MethodGet)
	request.RestyClient = resty.NewWithClient(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("must not be called")
	})})
	request.ProxyURL = "http://proxy.example.com"

	success, body, err := request.Send()
	require.False(t, success)
	require.Nil(t, body)
	require.ErrorIs(t, err, ErrProxyWithClient)
}

func TestSendContextPreservesInjectedRestyRetryPolicy(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		if attempts.Add(1) == 1 {
			writer.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, err := writer.Write([]byte("recovered"))
		require.NoError(t, err)
	}))
	defer server.Close()

	restyClient := resty.New().
		SetRetryCount(1).
		SetRetryWaitTime(time.Millisecond).
		AddRetryCondition(func(response *resty.Response, err error) bool {
			return err == nil && response.StatusCode() == http.StatusServiceUnavailable
		})
	request := NewRequest(server.URL, http.MethodGet)
	request.RestyClient = restyClient

	success, body, err := request.Send()
	require.NoError(t, err)
	require.True(t, success)
	require.Equal(t, "recovered", string(body))
	require.Equal(t, int32(2), attempts.Load())
}

func TestDefaultRestyClientDoesNotPersistCookiesOrRetry(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if attempts.Add(1) == 1 {
			http.SetCookie(writer, &http.Cookie{Name: "session", Value: "secret"})
			writer.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, err := request.Cookie("session")
		require.ErrorIs(t, err, http.ErrNoCookie)
		_, err = writer.Write([]byte("ok"))
		require.NoError(t, err)
	}))
	defer server.Close()

	success, _, err := NewRequest(server.URL, http.MethodGet).Send()
	require.False(t, success)
	var statusErr *StatusError
	require.ErrorAs(t, err, &statusErr)
	require.Equal(t, http.StatusServiceUnavailable, statusErr.StatusCode)
	require.Equal(t, int32(1), attempts.Load())

	success, body, err := NewRequest(server.URL, http.MethodGet).Send()
	require.NoError(t, err)
	require.True(t, success)
	require.Equal(t, "ok", string(body))
	require.Equal(t, int32(2), attempts.Load())
}

type roundTripFunc func(*http.Request) (*http.Response, error)

// RoundTrip 将请求转发给测试函数。
func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

type roundTripStub struct {
	roundTrip func(*http.Request) (*http.Response, error)
}

// RoundTrip 将请求转发给可检查身份的测试 stub。
func (stub *roundTripStub) RoundTrip(request *http.Request) (*http.Response, error) {
	return stub.roundTrip(request)
}
