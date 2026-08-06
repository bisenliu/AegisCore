package router

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	// HealthCheckStatusOK 表示检查项当前可用。
	HealthCheckStatusOK HealthCheckStatus = "ok"
	// HealthCheckStatusUnavailable 表示检查项当前不可用或未能在探针预算内完成。
	HealthCheckStatusUnavailable HealthCheckStatus = "unavailable"
	healthProbeTimeout                             = 500 * time.Millisecond
	healthPathLivez                                = "/livez"
	healthPathReadyz                               = "/readyz"
	healthPathStartupz                             = "/startupz"
)

// HealthCheckStatus 表示单个健康检查项的状态。
type HealthCheckStatus string

// HealthChecker 定义健康检查端点消费的最小依赖检查能力。
type HealthChecker interface {
	Name() string
	Check(ctx context.Context) HealthCheckResult
}

// HealthChecks 包含用户服务启动和流量接入探针使用的检查项。
type HealthChecks struct {
	Readiness []HealthChecker
	Startup   []HealthChecker
}

// HealthCheckResult 表示单个健康检查项的输出。
type HealthCheckResult struct {
	Name    string            `json:"name" example:"postgres.primary_db"`
	Status  HealthCheckStatus `json:"status" example:"ok"`
	Message string            `json:"message,omitempty" example:"dependency unavailable"`
}

// HealthResponse 是健康检查端点返回的响应体。
type HealthResponse struct {
	Status  HealthCheckStatus   `json:"status" example:"ok"`
	Service string              `json:"service" example:"aegiscore-user-service"`
	Checks  []HealthCheckResult `json:"checks,omitempty"`
}

type healthCheckOutcome struct {
	index  int
	result HealthCheckResult
}

func registerHealthRoutes(engine *gin.Engine, serviceName string, checks HealthChecks) {
	engine.GET(healthPathLivez, livez(serviceName))
	engine.GET(healthPathReadyz, readyz(serviceName, checks.Readiness))
	engine.GET(healthPathStartupz, startupz(serviceName, checks.Startup))
}

// IsHealthProbePath 判断 path 是否为用户服务健康探针路径。
func IsHealthProbePath(path string) bool {
	switch path {
	case healthPathLivez, healthPathReadyz, healthPathStartupz:
		return true
	default:
		return false
	}
}

// livez 返回服务的最小存活响应。
// @Summary 服务存活检查
// @Description 返回用户服务最小存活状态，不检查 PostgreSQL、Redis 或 RBAC policy。
// @Tags 系统
// @Produce json
// @Success 200 {object} HealthResponse "服务存活"
// @Router /livez [get]
func livez(serviceName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, HealthResponse{Status: HealthCheckStatusOK, Service: serviceName})
	}
}

// readyz 返回服务是否可以接入业务流量。
// @Summary 服务就绪检查
// @Description 检查 PostgreSQL、Redis、Casbin policy 和 RBAC policy watcher 是否就绪。
// @Tags 系统
// @Produce json
// @Success 200 {object} HealthResponse "服务就绪"
// @Failure 503 {object} HealthResponse "服务未就绪"
// @Router /readyz [get]
func readyz(serviceName string, checks []HealthChecker) gin.HandlerFunc {
	return probez(serviceName, checks)
}

// startupz 返回服务启动关键依赖是否已完成初始化。
// @Summary 服务启动检查
// @Description 检查用户服务启动所需关键依赖是否已完成初始化。
// @Tags 系统
// @Produce json
// @Success 200 {object} HealthResponse "服务启动完成"
// @Failure 503 {object} HealthResponse "服务启动未完成"
// @Router /startupz [get]
func startupz(serviceName string, checks []HealthChecker) gin.HandlerFunc {
	return probez(serviceName, checks)
}

func probez(serviceName string, checks []HealthChecker) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), healthProbeTimeout)
		defer cancel()

		results := runHealthChecks(ctx, checks)
		status := HealthCheckStatusOK
		for _, result := range results {
			if result.Status != HealthCheckStatusOK {
				status = HealthCheckStatusUnavailable
			}
		}

		httpStatus := http.StatusOK
		if status != HealthCheckStatusOK {
			httpStatus = http.StatusServiceUnavailable
		}
		c.JSON(httpStatus, HealthResponse{Status: status, Service: serviceName, Checks: results})
	}
}

func runHealthChecks(ctx context.Context, checks []HealthChecker) []HealthCheckResult {
	// 检查项并发执行并按原 index 回填；nil checker 会在压缩结果时移除，超时会把未完成项标记为 unavailable。
	results := make([]HealthCheckResult, len(checks))
	pending := make(map[int]HealthChecker, len(checks))
	resultCh := make(chan healthCheckOutcome, len(checks))

	for index, checker := range checks {
		if checker == nil {
			continue
		}
		pending[index] = checker
		// 依赖 Go 1.22+ range 变量语义；如迁移到旧版本，需要显式复制 index 和 checker。
		go func() {
			resultCh <- healthCheckOutcome{index: index, result: normalizeHealthCheckResult(checker, checker.Check(ctx))}
		}()
	}

	for len(pending) > 0 {
		select {
		case outcome := <-resultCh:
			results[outcome.index] = outcome.result
			delete(pending, outcome.index)
		case <-ctx.Done():
			for index, checker := range pending {
				results[index] = HealthCheckResult{
					Name:    checker.Name(),
					Status:  HealthCheckStatusUnavailable,
					Message: "health check timeout",
				}
			}
			return compactHealthCheckResults(results)
		}
	}

	return compactHealthCheckResults(results)
}

func normalizeHealthCheckResult(checker HealthChecker, result HealthCheckResult) HealthCheckResult {
	if result.Name == "" {
		result.Name = checker.Name()
	}
	if result.Status == "" {
		result.Status = HealthCheckStatusUnavailable
	}
	return result
}

func compactHealthCheckResults(results []HealthCheckResult) []HealthCheckResult {
	compacted := make([]HealthCheckResult, 0, len(results))
	for _, result := range results {
		if result.Name == "" && result.Status == "" && result.Message == "" {
			continue
		}
		compacted = append(compacted, result)
	}
	return compacted
}
