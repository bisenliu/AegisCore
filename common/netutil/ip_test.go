package netutil

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestGetClientIP(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		headers    map[string]string
		remoteAddr string
		want       string
	}{
		{
			name: "forwarded for first IP",
			headers: map[string]string{
				XForwardedFor: "203.0.113.10, 10.0.0.1",
			},
			remoteAddr: "198.51.100.10:12345",
			want:       "203.0.113.10",
		},
		{
			name: "forwarded for skips blank candidates",
			headers: map[string]string{
				XForwardedFor: "  , 203.0.113.11 ",
			},
			remoteAddr: "198.51.100.10:12345",
			want:       "203.0.113.11",
		},
		{
			name: "real IP fallback",
			headers: map[string]string{
				XForwardedFor: " ",
				XRealIP:       " 203.0.113.12 ",
				XClientIP:     "203.0.113.13",
			},
			remoteAddr: "198.51.100.10:12345",
			want:       "203.0.113.12",
		},
		{
			name: "client IP fallback",
			headers: map[string]string{
				XRealIP:   " ",
				XClientIP: " 203.0.113.13 ",
			},
			remoteAddr: "198.51.100.10:12345",
			want:       "203.0.113.13",
		},
		{
			name:       "gin client IP fallback",
			remoteAddr: "198.51.100.20:12345",
			want:       "198.51.100.20",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newTestContext(tt.remoteAddr, tt.headers)

			if got := GetClientIP(c); got != tt.want {
				t.Fatalf("GetClientIP() = %q, want %q", got, tt.want)
			}
		})
	}
}

func newTestContext(remoteAddr string, headers map[string]string) *gin.Context {
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = remoteAddr
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	c.Request = req

	return c
}
