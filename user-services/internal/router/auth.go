package router

import (
	"github.com/aegiscore/user-services/internal/auth"
	"github.com/gin-gonic/gin"
)

func registerPublicAuthRoutes(group *gin.RouterGroup, authController *auth.AuthController) {
	group.POST("/login", authController.LoginUser)
	group.POST("/refresh", authController.RefreshToken)
	group.POST("/change-password", authController.ChangePassword)
}

func registerProtectedAuthRoutes(group *gin.RouterGroup, authController *auth.AuthController) {
	group.POST("/logout", authController.LogoutCurrentSession)
	group.POST("/logout-all", authController.LogoutAllSessions)
}
