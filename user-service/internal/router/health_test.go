package router

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestRegisterHealthRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	registerHealthRoutes(engine, "aegiscore-user-services", HealthChecks{
		Readiness: []HealthChecker{
			staticHealthChecker{name: "postgres.user_db", status: HealthCheckStatusOK},
			staticHealthChecker{name: "redis.cache_redis", status: HealthCheckStatusOK},
		},
		Startup: []HealthChecker{
			staticHealthChecker{name: "rbac.casbin_policy", status: HealthCheckStatusOK},
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
			staticHealthChecker{name: "postgres.user_db", status: HealthCheckStatusOK},
			staticHealthChecker{name: "redis.cache_redis", status: HealthCheckStatusUnavailable, message: "redis unavailable"},
		},
		Startup: []HealthChecker{
			staticHealthChecker{name: "rbac.policy_watcher", status: HealthCheckStatusUnavailable, message: "rbac policy watcher stopped"},
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

func TestProbezRunsChecksConcurrently(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	registerHealthRoutes(engine, "aegiscore-user-services", HealthChecks{
		Readiness: []HealthChecker{
			delayedHealthChecker{name: "postgres.user_db", delay: 300 * time.Millisecond, status: HealthCheckStatusOK},
			delayedHealthChecker{name: "redis.cache_redis", delay: 300 * time.Millisecond, status: HealthCheckStatusOK},
		},
	})

	startedAt := time.Now()
	recorder := executeHealthRequest(engine, "/readyz")
	elapsed := time.Since(startedAt)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if elapsed >= healthProbeTimeout {
		t.Fatalf("elapsed = %s, want less than %s to prove concurrent checks", elapsed, healthProbeTimeout)
	}

	health := decodeHealthResponse(t, recorder)
	if len(health.Checks) != 2 {
		t.Fatalf("checks = %#v, want 2 checks", health.Checks)
	}
	if health.Checks[0].Name != "postgres.user_db" || health.Checks[1].Name != "redis.cache_redis" {
		t.Fatalf("checks = %#v, want configured order", health.Checks)
	}
}

func TestProbezReturnsUnavailableForTimedOutCheck(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	release := make(chan struct{})
	defer close(release)
	registerHealthRoutes(engine, "aegiscore-user-services", HealthChecks{
		Readiness: []HealthChecker{
			staticHealthChecker{name: "postgres.user_db", status: HealthCheckStatusOK},
			blockingHealthChecker{name: "redis.cache_redis", release: release},
		},
	})

	recorder := executeHealthRequest(engine, "/readyz")
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusServiceUnavailable, recorder.Body.String())
	}

	health := decodeHealthResponse(t, recorder)
	if health.Status != HealthCheckStatusUnavailable {
		t.Fatalf("status = %q, want %q", health.Status, HealthCheckStatusUnavailable)
	}
	if len(health.Checks) != 2 {
		t.Fatalf("checks = %#v, want 2 checks", health.Checks)
	}
	if health.Checks[1].Name != "redis.cache_redis" || health.Checks[1].Status != HealthCheckStatusUnavailable || health.Checks[1].Message != "health check timeout" {
		t.Fatalf("timed out check = %#v, want redis timeout", health.Checks[1])
	}
}

type staticHealthChecker struct {
	name    string
	status  HealthCheckStatus
	message string
}

func (c staticHealthChecker) Name() string {
	return c.name
}

func (c staticHealthChecker) Check(context.Context) HealthCheckResult {
	return HealthCheckResult{Name: c.name, Status: c.status, Message: c.message}
}

type delayedHealthChecker struct {
	name   string
	delay  time.Duration
	status HealthCheckStatus
}

func (c delayedHealthChecker) Name() string {
	return c.name
}

func (c delayedHealthChecker) Check(ctx context.Context) HealthCheckResult {
	select {
	case <-time.After(c.delay):
		return HealthCheckResult{Name: c.name, Status: c.status}
	case <-ctx.Done():
		return HealthCheckResult{Name: c.name, Status: HealthCheckStatusUnavailable, Message: "dependency unavailable"}
	}
}

type blockingHealthChecker struct {
	name    string
	release <-chan struct{}
}

func (c blockingHealthChecker) Name() string {
	return c.name
}

func (c blockingHealthChecker) Check(context.Context) HealthCheckResult {
	<-c.release
	return HealthCheckResult{Name: c.name, Status: HealthCheckStatusOK}
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
