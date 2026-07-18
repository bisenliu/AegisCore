package permission

import (
	"github.com/gin-gonic/gin"

	permissionhttp "github.com/aegiscore/user-service/internal/features/permission/transport/http"
)

type permissionRouteRegistrar struct {
	controller *permissionhttp.PermissionController
}

func newPermissionRouteRegistrar(controller *permissionhttp.PermissionController) *permissionRouteRegistrar {
	return &permissionRouteRegistrar{controller: controller}
}

func (r *permissionRouteRegistrar) RouteKey() string {
	return "permission"
}

func (r *permissionRouteRegistrar) RegisterAuthorizedRoutes(group *gin.RouterGroup) {
	permissionhttp.RegisterRoutes(group.Group("/permissions"), r.controller)
}
