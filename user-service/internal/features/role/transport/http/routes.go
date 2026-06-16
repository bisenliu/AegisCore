package rolehttp

import "github.com/gin-gonic/gin"

// RegisterRoleRoutes 挂载角色生命周期和角色权限绑定路由。
func RegisterRoleRoutes(group *gin.RouterGroup, controller *RoleController) {
	group.GET("", controller.ListRoles)
	group.POST("", controller.CreateRole)
	group.GET("/:role_id", controller.GetRole)
	group.PATCH("/:role_id", controller.UpdateRole)
	group.PATCH("/:role_id/status", controller.SetRoleStatus)
	group.GET("/:role_id/permissions", controller.ListRolePermissions)
	group.PUT("/:role_id/permissions", controller.ReplaceRolePermissions)
	group.POST("/:role_id/permissions", controller.AddRolePermission)
	group.DELETE("/:role_id/permissions/:permission_id", controller.RemoveRolePermission)
}

// RegisterUserRoleRoutes 挂载用户角色绑定路由。
func RegisterUserRoleRoutes(group *gin.RouterGroup, controller *RoleController) {
	group.GET("/:user_id/roles", controller.ListUserRoles)
	group.PUT("/:user_id/roles", controller.ReplaceUserRoles)
	group.POST("/:user_id/roles", controller.AddUserRole)
	group.DELETE("/:user_id/roles/:role_id", controller.RemoveUserRole)
}
