package userhttp

import "github.com/gin-gonic/gin"

// RegisterRoutes mounts user profile routes under the provided /users group.
func RegisterRoutes(group *gin.RouterGroup, controller *UserController) {
	group.GET("", controller.ListUsers)
	group.POST("", controller.CreateUser)
	group.GET("/:user_id", controller.GetByUserID)
}
