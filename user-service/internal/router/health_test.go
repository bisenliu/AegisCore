package router

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRegisterHealthRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	registerHealthRoutes(engine, "aegiscore-user-service", HealthChecks{
		Readiness: []HealthChecker{
			staticHealthChecker{name: "postgres.primary_db", status: HealthCheckStatusOK},
			staticHealthChecker{name: "redis.cache_redis", status: HealthCheckStatusOK},
		},
		Startup: []HealthChecker{
			staticHealthChecker{name: "rbac.casbin_policy", status: HealthCheckStatusOK},
		},
	})

	t.Run("livez returns liveness without dependency checks", func(t *testing.T) {
		recorder := executeHealthRequest(engine, "/livez")
		require.Equal(t, http.StatusOK, recorder.Code)
		health := decodeHealthResponse(t, recorder)
		require.Equal(t, HealthCheckStatusOK, health.Status)
		require.Equal(t, "aegiscore-user-service", health.Service)
		require.Empty(t, health.Checks)
	})

	t.Run("readyz returns component checks", func(t *testing.T) {
		recorder := executeHealthRequest(engine, "/readyz")
		require.Equal(t, http.StatusOK, recorder.Code)
		health := decodeHealthResponse(t, recorder)
		require.Equal(t, HealthCheckStatusOK, health.Status)
		require.Len(t, health.Checks, 2)
	})

	t.Run("startupz returns startup checks", func(t *testing.T) {
		recorder := executeHealthRequest(engine, "/startupz")
		require.Equal(t, http.StatusOK, recorder.Code)
		health := decodeHealthResponse(t, recorder)
		require.Equal(t, HealthCheckStatusOK, health.Status)
		require.Len(t, health.Checks, 1)
		require.Equal(t, "rbac.casbin_policy", health.Checks[0].Name)
	})
}

func TestProbezReturnsUnavailableWhenAnyCheckFails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	registerHealthRoutes(engine, "aegiscore-user-service", HealthChecks{
		Readiness: []HealthChecker{
			staticHealthChecker{name: "postgres.primary_db", status: HealthCheckStatusOK},
			staticHealthChecker{name: "redis.cache_redis", status: HealthCheckStatusUnavailable, message: "redis unavailable"},
		},
		Startup: []HealthChecker{
			staticHealthChecker{name: "rbac.policy_watcher", status: HealthCheckStatusUnavailable, message: "rbac policy watcher stopped"},
		},
	})

	for _, path := range []string{"/readyz", "/startupz"} {
		t.Run(path, func(t *testing.T) {
			recorder := executeHealthRequest(engine, path)
			require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
			health := decodeHealthResponse(t, recorder)
			require.Equal(t, HealthCheckStatusUnavailable, health.Status)
			require.NotEmpty(t, health.Checks)
		})
	}
}

func TestProbezRunsChecksConcurrently(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	registerHealthRoutes(engine, "aegiscore-user-service", HealthChecks{
		Readiness: []HealthChecker{
			delayedHealthChecker{name: "postgres.primary_db", delay: 300 * time.Millisecond, status: HealthCheckStatusOK},
			delayedHealthChecker{name: "redis.cache_redis", delay: 300 * time.Millisecond, status: HealthCheckStatusOK},
		},
	})

	startedAt := time.Now()
	recorder := executeHealthRequest(engine, "/readyz")
	elapsed := time.Since(startedAt)
	require.Equal(t, http.StatusOK, recorder.Code, "body=%s", recorder.Body.String())
	require.Less(t, elapsed, healthProbeTimeout)

	health := decodeHealthResponse(t, recorder)
	require.Len(t, health.Checks, 2)
	require.Equal(t, "postgres.primary_db", health.Checks[0].Name)
	require.Equal(t, "redis.cache_redis", health.Checks[1].Name)
}

func TestProbezReturnsUnavailableForTimedOutCheck(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	release := make(chan struct{})
	defer close(release)
	registerHealthRoutes(engine, "aegiscore-user-service", HealthChecks{
		Readiness: []HealthChecker{
			staticHealthChecker{name: "postgres.primary_db", status: HealthCheckStatusOK},
			blockingHealthChecker{name: "redis.cache_redis", release: release},
		},
	})

	recorder := executeHealthRequest(engine, "/readyz")
	require.Equal(t, http.StatusServiceUnavailable, recorder.Code, "body=%s", recorder.Body.String())

	health := decodeHealthResponse(t, recorder)
	require.Equal(t, HealthCheckStatusUnavailable, health.Status)
	require.Len(t, health.Checks, 2)
	require.Equal(t, HealthCheckResult{
		Name:    "redis.cache_redis",
		Status:  HealthCheckStatusUnavailable,
		Message: "health check timeout",
	}, health.Checks[1])
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
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &health))
	return health
}
