package authhttp

import "github.com/gin-gonic/gin"

// RegisterPublicRoutes mounts authentication routes that do not require a normal access token.
func RegisterPublicRoutes(group *gin.RouterGroup, controller *AuthController) {
	group.POST("/login", controller.LoginUser)
	group.POST("/refresh", controller.RefreshToken)
	group.POST("/change-password", controller.ChangePassword)
}

// RegisterProtectedRoutes mounts authentication routes that require a normal access token.
func RegisterProtectedRoutes(group *gin.RouterGroup, controller *AuthController) {
	group.POST("/logout", controller.LogoutCurrentSession)
	group.POST("/logout-all", controller.LogoutAllSessions)
}
