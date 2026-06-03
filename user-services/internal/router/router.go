package router

import (
	"net/http"

	"github.com/aegiscore/common/config"
	"github.com/aegiscore/common/credentials"
	commonmw "github.com/aegiscore/common/middleware"
	"github.com/aegiscore/user-services/internal/controller"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type RouteParams struct {
	Environment           string
	Log                   *zap.Logger
	JWT                   *credentials.JWTService
	AuthConfig            config.AuthConfig
	TokenVersionValidator commonmw.TokenVersionValidator
	AuthController        *controller.AuthController
	UserController        *controller.UserController
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
		publicAuth := v1.Group("/auth")
		publicAuth.POST("/login", params.AuthController.Login)
		publicAuth.POST("/refresh", params.AuthController.Refresh)
		publicAuth.POST("/change-password", params.AuthController.ChangePassword)

		authenticated := v1.Group("")
		authenticated.Use(commonmw.AuthWithTokenVersionValidator(params.Log, params.JWT, params.AuthConfig, params.TokenVersionValidator))

		protectedAuth := authenticated.Group("/auth")
		protectedAuth.POST("/logout", params.AuthController.Logout)
		protectedAuth.POST("/logout-all", params.AuthController.LogoutAll)

		// Mount future Casbin authorization middleware on this group after authentication.
		authorized := authenticated.Group("")
		users := authorized.Group("/users")
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
