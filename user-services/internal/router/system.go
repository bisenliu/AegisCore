package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// HealthResponse 是健康检查端点返回的最小存活响应体。
type HealthResponse struct {
	Status  string `json:"status" example:"ok"`
	Service string `json:"service" example:"aegiscore-user-services"`
}

func registerSystemRoutes(engine *gin.Engine) {
	engine.GET("/healthz", healthz)
}

// healthz 返回服务的最小存活响应。
// @Summary 服务健康检查
// @Description 返回用户服务最小健康状态。
// @Tags 系统
// @Produce json
// @Success 200 {object} HealthResponse "服务健康"
// @Router /healthz [get]
func healthz(c *gin.Context) {
	c.JSON(http.StatusOK, HealthResponse{Status: "ok", Service: "aegiscore-user-services"})
}
