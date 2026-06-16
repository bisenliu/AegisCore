package router

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	HealthCheckStatusOK          HealthCheckStatus = "ok"
	HealthCheckStatusUnavailable HealthCheckStatus = "unavailable"
	healthProbeTimeout                             = 500 * time.Millisecond
)

// HealthCheckStatus 表示单个健康检查项的状态。
type HealthCheckStatus string

// HealthChecker 定义健康检查端点消费的最小依赖检查能力。
type HealthChecker interface {
	Check(ctx context.Context) HealthCheckResult
}

// HealthChecks 包含用户服务启动和流量接入探针使用的检查项。
type HealthChecks struct {
	Readiness []HealthChecker
	Startup   []HealthChecker
}

// HealthCheckResult 表示单个健康检查项的输出。
type HealthCheckResult struct {
	Name    string            `json:"name" example:"postgres.user_db"`
	Status  HealthCheckStatus `json:"status" example:"ok"`
	Message string            `json:"message,omitempty" example:"dependency unavailable"`
}

// HealthResponse 是健康检查端点返回的响应体。
type HealthResponse struct {
	Status  HealthCheckStatus   `json:"status" example:"ok"`
	Service string              `json:"service" example:"aegiscore-user-services"`
	Checks  []HealthCheckResult `json:"checks,omitempty"`
}

func registerHealthRoutes(engine *gin.Engine, serviceName string, checks HealthChecks) {
	engine.GET("/livez", livez(serviceName))
	engine.GET("/readyz", readyz(serviceName, checks.Readiness))
	engine.GET("/startupz", startupz(serviceName, checks.Startup))
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

		status := HealthCheckStatusOK
		results := make([]HealthCheckResult, 0, len(checks))
		for _, checker := range checks {
			if checker == nil {
				continue
			}
			result := checker.Check(ctx)
			if result.Status == "" {
				result.Status = HealthCheckStatusUnavailable
			}
			if result.Status != HealthCheckStatusOK {
				status = HealthCheckStatusUnavailable
			}
			results = append(results, result)
		}

		httpStatus := http.StatusOK
		if status != HealthCheckStatusOK {
			httpStatus = http.StatusServiceUnavailable
		}
		c.JSON(httpStatus, HealthResponse{Status: status, Service: serviceName, Checks: results})
	}
}
