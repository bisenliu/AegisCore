package router

import (
	"github.com/aegiscore/user-services/internal/user"
	"github.com/gin-gonic/gin"
)

func registerUserRoutes(group *gin.RouterGroup, userController *user.UserController) {
	group.GET("", userController.ListUsers)
	group.POST("", userController.CreateUser)
	group.GET("/:user_id", userController.GetByUserID)
}
