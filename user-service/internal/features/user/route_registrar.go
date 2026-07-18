package user

import (
	"github.com/gin-gonic/gin"

	userhttp "github.com/aegiscore/user-service/internal/features/user/transport/http"
)

type userRouteRegistrar struct {
	controller *userhttp.UserController
}

func newUserRouteRegistrar(controller *userhttp.UserController) *userRouteRegistrar {
	return &userRouteRegistrar{controller: controller}
}

func (r *userRouteRegistrar) RouteKey() string {
	return "user"
}

func (r *userRouteRegistrar) RegisterAuthorizedRoutes(group *gin.RouterGroup) {
	userhttp.RegisterRoutes(group.Group("/users"), r.controller)
}
