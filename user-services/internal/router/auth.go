package router

import (
	"github.com/aegiscore/user-services/internal/controller"
	"github.com/gin-gonic/gin"
)

func registerPublicAuthRoutes(group *gin.RouterGroup, authController *controller.AuthController) {
	group.POST("/login", authController.Login)
	group.POST("/refresh", authController.Refresh)
	group.POST("/change-password", authController.ChangePassword)
}

func registerProtectedAuthRoutes(group *gin.RouterGroup, authController *controller.AuthController) {
	group.POST("/logout", authController.Logout)
	group.POST("/logout-all", authController.LogoutAll)
}
