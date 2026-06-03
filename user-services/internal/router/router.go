package router

import (
	"net/http"

	"github.com/aegiscore/user-services/internal/controller"
	"github.com/gin-gonic/gin"
)

type RouteParams struct {
	Environment    string
	AuthController *controller.AuthController
	UserController *controller.UserController
}

type HealthResponse struct {
	Status  string `json:"status" example:"ok"`
	Service string `json:"service" example:"aegiscore-user-services"`
}

func RegisterRoutes(engine *gin.Engine, params RouteParams) {
	engine.GET("/healthz", healthz)
	RegisterSwagger(engine, params.Environment)

	v1 := engine.Group("/api/v1")
	{
		auth := v1.Group("/auth")
		auth.POST("/login", params.AuthController.Login)
		auth.POST("/refresh", params.AuthController.Refresh)
		auth.POST("/change-password", params.AuthController.ChangePassword)
		auth.POST("/logout", params.AuthController.Logout)
		auth.POST("/logout-all", params.AuthController.LogoutAll)

		users := v1.Group("/users")
		users.GET("", params.UserController.List)
		users.POST("", params.UserController.Create)
		users.GET("/:user_id", params.UserController.GetByID)
	}
}

// healthz godoc
// @Summary 服务健康检查
// @Description 返回用户服务最小健康状态。
// @Tags 系统
// @Produce json
// @Success 200 {object} HealthResponse "服务健康"
// @Router /healthz [get]
func healthz(c *gin.Context) {
	c.JSON(http.StatusOK, HealthResponse{Status: "ok", Service: "aegiscore-user-services"})
}
