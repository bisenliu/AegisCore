package router

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRegisterHealthRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	registerHealthRoutes(engine, "aegiscore-user-services")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	engine.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}

	var health HealthResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &health); err != nil {
		t.Fatalf("unmarshal health response: %v", err)
	}
	if health.Status != healthStatusOK {
		t.Fatalf("status = %q, want %q", health.Status, healthStatusOK)
	}
	if health.Service != "aegiscore-user-services" {
		t.Fatalf("service = %q, want aegiscore-user-services", health.Service)
	}
}
