package authhttp

import "github.com/gin-gonic/gin"

// RegisterPublicRoutes 挂载不需要普通 access token 的认证路由。
func RegisterPublicRoutes(group *gin.RouterGroup, controller *AuthController) {
	group.POST("/login", controller.LoginUser)
	group.POST("/refresh", controller.RefreshToken)
	group.POST("/force-change-password", controller.ForceChangePassword)
}

// RegisterProtectedRoutes 挂载需要普通 access token 的认证路由。
func RegisterProtectedRoutes(group *gin.RouterGroup, controller *AuthController) {
	group.POST("/logout", controller.LogoutCurrentSession)
	group.POST("/logout-all", controller.LogoutAllSessions)
}
