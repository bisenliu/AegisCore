package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type corsResult struct {
	status  int
	headers http.Header
	body    string
	handled bool
}

func TestCORSUsesDefaultOptions(t *testing.T) {
	tests := []struct {
		name        string
		method      string
		wantStatus  int
		wantBody    string
		wantHandled bool
	}{
		{
			name:        "simple request",
			method:      http.MethodGet,
			wantStatus:  http.StatusAccepted,
			wantBody:    "handled",
			wantHandled: true,
		},
		{
			name:        "preflight request",
			method:      http.MethodOptions,
			wantStatus:  http.StatusNoContent,
			wantBody:    "",
			wantHandled: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := exerciseCORSRequest(t, CORS(), tt.method)
			want := exerciseCORSRequest(t, CORSWithOptions(defaultCORSOptions), tt.method)

			requireEquivalentCORSResult(t, want, got)
			require.Equal(t, tt.wantStatus, got.status)
			require.Equal(t, tt.wantBody, got.body)
			require.Equal(t, tt.wantHandled, got.handled)
			requireDefaultCORSHeaders(t, got.headers)
		})
	}
}

func TestCORSWithOptionsAppliesConfiguredHeaders(t *testing.T) {
	got := exerciseCORSRequest(t, CORSWithOptions(CORSOptions{
		AllowedMethods:   []string{http.MethodGet},
		AllowedHeaders:   []string{"Content-Type"},
		ExposedHeaders:   []string{"X-Trace-ID"},
		AllowCredentials: true,
		MaxAgeSeconds:    600,
		ReflectOrigin:    true,
	}), http.MethodGet)

	require.Equal(t, http.StatusAccepted, got.status)
	require.True(t, got.handled)
	require.Equal(t, "handled", got.body)
	require.Equal(t, "https://client.test", got.headers.Get(HeaderAccessControlAllowOrigin))
	require.Equal(t, HeaderOrigin, got.headers.Get(HeaderVary))
	require.Equal(t, http.MethodGet, got.headers.Get(HeaderAccessControlAllowMethods))
	require.Equal(t, "Content-Type", got.headers.Get(HeaderAccessControlAllowHeaders))
	require.Equal(t, "X-Trace-ID", got.headers.Get(HeaderAccessControlExposeHeaders))
	require.Equal(t, "true", got.headers.Get(HeaderAccessControlAllowCredentials))
	require.Equal(t, "600", got.headers.Get(HeaderAccessControlMaxAge))
}

func exerciseCORSRequest(t *testing.T, middleware gin.HandlerFunc, method string) corsResult {
	t.Helper()

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(middleware)
	handled := false
	engine.Handle(method, "/cors", func(c *gin.Context) {
		handled = true
		c.String(http.StatusAccepted, "handled")
	})

	req := httptest.NewRequest(method, "/cors", nil)
	req.Header.Set(HeaderOrigin, "https://client.test")
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)

	return corsResult{
		status:  recorder.Code,
		headers: recorder.Header().Clone(),
		body:    recorder.Body.String(),
		handled: handled,
	}
}

func requireDefaultCORSHeaders(t *testing.T, headers http.Header) {
	t.Helper()

	require.Equal(t, "*", headers.Get(HeaderAccessControlAllowOrigin))
	require.Equal(t, "GET,POST,PUT,PATCH,DELETE,OPTIONS", headers.Get(HeaderAccessControlAllowMethods))
	require.Equal(t, "Authorization,Content-Type", headers.Get(HeaderAccessControlAllowHeaders))
	require.Empty(t, headers.Get(HeaderAccessControlAllowCredentials))
	require.Empty(t, headers.Get(HeaderAccessControlMaxAge))
	require.Empty(t, headers.Get(HeaderAccessControlExposeHeaders))
	require.Empty(t, headers.Get(HeaderVary))
}

func requireEquivalentCORSResult(t *testing.T, want corsResult, got corsResult) {
	t.Helper()

	require.Equal(t, want.status, got.status)
	require.Equal(t, want.body, got.body)
	require.Equal(t, want.handled, got.handled)
	for _, header := range []string{
		HeaderAccessControlAllowOrigin,
		HeaderAccessControlAllowMethods,
		HeaderAccessControlAllowHeaders,
		HeaderAccessControlExposeHeaders,
		HeaderAccessControlAllowCredentials,
		HeaderAccessControlMaxAge,
		HeaderVary,
	} {
		require.Equal(t, want.headers.Get(header), got.headers.Get(header), header)
	}
}
