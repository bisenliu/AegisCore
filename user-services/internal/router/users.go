package router

import (
	"github.com/aegiscore/user-services/internal/controller"
	"github.com/gin-gonic/gin"
)

func registerUserRoutes(group *gin.RouterGroup, userController *controller.UserController) {
	group.GET("", userController.ListUsers)
	group.POST("", userController.CreateUser)
	group.GET("/:user_id", userController.GetByUserID)
}
