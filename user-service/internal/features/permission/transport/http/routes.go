package permissionhttp

import "github.com/gin-gonic/gin"

// RegisterRoutes 将权限目录路由挂载到传入的 /permissions 分组下。
func RegisterRoutes(group *gin.RouterGroup, controller *PermissionController) {
	group.GET("", controller.ListPermissions)
	group.POST("", controller.CreatePermission)
	group.GET("/route-diff", controller.GetRouteDiff)
	group.GET("/users/:user_id/effective", controller.ListUserEffectivePermissions)
	group.GET("/:permission_id", controller.GetPermission)
	group.PUT("/:permission_id", controller.UpdatePermission)
	group.POST("/:permission_id/enable", controller.EnablePermission)
	group.POST("/:permission_id/disable", controller.DisablePermission)
}
