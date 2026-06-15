package router

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRegisterHealthRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	registerHealthRoutes(engine, "aegiscore-user-services", HealthChecks{
		Readiness: []HealthChecker{
			staticHealthChecker{Name: "postgres.user_db", Status: HealthCheckStatusOK},
			staticHealthChecker{Name: "redis.cache_redis", Status: HealthCheckStatusOK},
		},
		Startup: []HealthChecker{
			staticHealthChecker{Name: "rbac.casbin_policy", Status: HealthCheckStatusOK},
		},
	})

	t.Run("livez returns liveness without dependency checks", func(t *testing.T) {
		recorder := executeHealthRequest(engine, "/livez")
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
		}
		health := decodeHealthResponse(t, recorder)
		if health.Status != HealthCheckStatusOK {
			t.Fatalf("status = %q, want %q", health.Status, HealthCheckStatusOK)
		}
		if health.Service != "aegiscore-user-services" {
			t.Fatalf("service = %q, want aegiscore-user-services", health.Service)
		}
		if len(health.Checks) != 0 {
			t.Fatalf("checks = %#v, want empty liveness checks", health.Checks)
		}
	})

	t.Run("readyz returns component checks", func(t *testing.T) {
		recorder := executeHealthRequest(engine, "/readyz")
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
		}
		health := decodeHealthResponse(t, recorder)
		if health.Status != HealthCheckStatusOK || len(health.Checks) != 2 {
			t.Fatalf("health = %#v, want ok with 2 checks", health)
		}
	})

	t.Run("startupz returns startup checks", func(t *testing.T) {
		recorder := executeHealthRequest(engine, "/startupz")
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
		}
		health := decodeHealthResponse(t, recorder)
		if health.Status != HealthCheckStatusOK || len(health.Checks) != 1 || health.Checks[0].Name != "rbac.casbin_policy" {
			t.Fatalf("health = %#v, want startup checks", health)
		}
	})
}

func TestProbezReturnsUnavailableWhenAnyCheckFails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	registerHealthRoutes(engine, "aegiscore-user-services", HealthChecks{
		Readiness: []HealthChecker{
			staticHealthChecker{Name: "postgres.user_db", Status: HealthCheckStatusOK},
			staticHealthChecker{Name: "redis.cache_redis", Status: HealthCheckStatusUnavailable, Message: "redis unavailable"},
		},
		Startup: []HealthChecker{
			staticHealthChecker{Name: "rbac.policy_watcher", Status: HealthCheckStatusUnavailable, Message: "rbac policy watcher stopped"},
		},
	})

	for _, path := range []string{"/readyz", "/startupz"} {
		t.Run(path, func(t *testing.T) {
			recorder := executeHealthRequest(engine, path)
			if recorder.Code != http.StatusServiceUnavailable {
				t.Fatalf("status = %d, want %d", recorder.Code, http.StatusServiceUnavailable)
			}
			health := decodeHealthResponse(t, recorder)
			if health.Status != HealthCheckStatusUnavailable {
				t.Fatalf("status = %q, want %q", health.Status, HealthCheckStatusUnavailable)
			}
			if len(health.Checks) == 0 {
				t.Fatal("checks empty, want failed component")
			}
		})
	}
}

type staticHealthChecker struct {
	Name    string
	Status  HealthCheckStatus
	Message string
}

func (c staticHealthChecker) Check(context.Context) HealthCheckResult {
	return HealthCheckResult{Name: c.Name, Status: c.Status, Message: c.Message}
}

func executeHealthRequest(engine *gin.Engine, path string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, path, nil)
	engine.ServeHTTP(recorder, request)
	return recorder
}

func decodeHealthResponse(t *testing.T, recorder *httptest.ResponseRecorder) HealthResponse {
	t.Helper()
	var health HealthResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &health); err != nil {
		t.Fatalf("unmarshal health response: %v", err)
	}
	return health
}
