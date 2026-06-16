package providers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestSkipSuccessfulHealthProbeLog(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/livez", func(c *gin.Context) {
		c.Status(http.StatusOK)
		if !skipSuccessfulHealthProbeLog(c) {
			t.Fatal("skipSuccessfulHealthProbeLog = false, want true")
		}
	})
	engine.GET("/readyz", func(c *gin.Context) {
		c.Status(http.StatusServiceUnavailable)
		if skipSuccessfulHealthProbeLog(c) {
			t.Fatal("skipSuccessfulHealthProbeLog = true, want false")
		}
	})
	engine.GET("/api/v1/users", func(c *gin.Context) {
		c.Status(http.StatusOK)
		if skipSuccessfulHealthProbeLog(c) {
			t.Fatal("skipSuccessfulHealthProbeLog = true, want false")
		}
	})

	for _, path := range []string{"/livez", "/readyz", "/api/v1/users"} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, path, nil)
		engine.ServeHTTP(recorder, request)
	}
}
