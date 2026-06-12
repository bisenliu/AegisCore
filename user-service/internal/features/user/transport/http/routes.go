package userhttp

import "github.com/gin-gonic/gin"

// RegisterRoutes 将用户资料路由挂载到传入的 /users 分组下。
func RegisterRoutes(group *gin.RouterGroup, controller *UserController) {
	group.GET("", controller.ListUsers)
	group.POST("", controller.CreateUser)
	group.GET("/:user_id", controller.GetByUserID)
}
