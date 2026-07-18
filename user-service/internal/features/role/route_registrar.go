package role

import (
	"github.com/gin-gonic/gin"

	rolehttp "github.com/aegiscore/user-service/internal/features/role/transport/http"
)

type roleRouteRegistrar struct {
	controller *rolehttp.RoleController
}

func newRoleRouteRegistrar(controller *rolehttp.RoleController) *roleRouteRegistrar {
	return &roleRouteRegistrar{controller: controller}
}

func (r *roleRouteRegistrar) RouteKey() string {
	return "role"
}

func (r *roleRouteRegistrar) RegisterAuthorizedRoutes(group *gin.RouterGroup) {
	rolehttp.RegisterRoleRoutes(group.Group("/roles"), r.controller)
	rolehttp.RegisterUserRoleRoutes(group.Group("/users"), r.controller)
}
