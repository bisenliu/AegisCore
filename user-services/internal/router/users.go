package router

import (
	"github.com/aegiscore/user-services/internal/controller"
	"github.com/gin-gonic/gin"
)

func registerUserRoutes(group *gin.RouterGroup, userController *controller.UserController) {
	group.GET("", userController.List)
	group.POST("", userController.Create)
	group.GET("/:user_id", userController.GetByID)
}
