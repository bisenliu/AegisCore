package main

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRunHealthcheck(t *testing.T) {
	for _, tc := range []struct {
		name      string
		status    int
		body      string
		wantError string
	}{
		{name: "ready", status: http.StatusOK, body: `{"status":"ok"}`},
		{name: "non ready", status: http.StatusOK, body: `{"status":"unavailable"}`, wantError: `healthcheck status is "unavailable"`},
		{name: "non 2xx", status: http.StatusServiceUnavailable, body: `{"status":"unavailable"}`, wantError: "HTTP 503"},
		{name: "invalid response", status: http.StatusOK, body: `{`, wantError: "decode healthcheck response"},
		{name: "missing status", status: http.StatusOK, body: `{}`, wantError: "missing status"},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			t.Cleanup(server.Close)

			err := runHealthcheck(context.Background(), healthcheckOptions{url: server.URL, timeout: time.Second})

			if tc.wantError == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tc.wantError)
		})
	}
}

func TestRunHealthcheckConnectionFailure(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := listener.Addr().String()
	require.NoError(t, listener.Close())

	err = runHealthcheck(context.Background(), healthcheckOptions{url: "http://" + addr + "/readyz", timeout: time.Second})

	require.ErrorContains(t, err, "healthcheck request failed")
}

func TestRunHealthcheckTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	t.Cleanup(server.Close)

	err := runHealthcheck(context.Background(), healthcheckOptions{url: server.URL, timeout: 10 * time.Millisecond})

	require.ErrorContains(t, err, "healthcheck request failed")
}

func TestRunHealthcheckValidatesParameters(t *testing.T) {
	for _, tc := range []struct {
		name      string
		opts      healthcheckOptions
		wantError string
	}{
		{name: "missing url", opts: healthcheckOptions{timeout: time.Second}, wantError: "url is required"},
		{name: "bad url", opts: healthcheckOptions{url: "://bad", timeout: time.Second}, wantError: "parse healthcheck url"},
		{name: "bad scheme", opts: healthcheckOptions{url: "ftp://127.0.0.1/readyz", timeout: time.Second}, wantError: "scheme must be http or https"},
		{name: "missing host", opts: healthcheckOptions{url: "http:///readyz", timeout: time.Second}, wantError: "host is required"},
		{name: "bad timeout", opts: healthcheckOptions{url: defaultHealthcheckURL}, wantError: "timeout must be positive"},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := runHealthcheck(context.Background(), tc.opts)

			require.ErrorContains(t, err, tc.wantError)
		})
	}
}
