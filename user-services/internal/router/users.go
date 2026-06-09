package router

import (
	userapp "github.com/aegiscore/user-services/internal/features/user/app"
	"github.com/gin-gonic/gin"
)

func registerUserRoutes(group *gin.RouterGroup, userController *userapp.UserController) {
	group.GET("", userController.ListUsers)
	group.POST("", userController.CreateUser)
	group.GET("/:user_id", userController.GetByUserID)
}
