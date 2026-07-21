package permissionhttp

import "github.com/gin-gonic/gin"

// RegisterRoutes 将权限目录路由挂载到传入的 /permissions 分组下。
func RegisterRoutes(group *gin.RouterGroup, controller *PermissionController) {
	group.GET("", controller.ListPermissions)
	group.GET("/users/:user_id/effective", controller.ListUserEffectivePermissions)
}
