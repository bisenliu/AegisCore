package auth

import (
	"github.com/gin-gonic/gin"

	authhttp "github.com/aegiscore/user-service/internal/features/auth/transport/http"
)

type authRouteRegistrar struct {
	controller *authhttp.AuthController
}

func newAuthRouteRegistrar(controller *authhttp.AuthController) *authRouteRegistrar {
	return &authRouteRegistrar{controller: controller}
}

func newPublicAuthRouteRegistrar(controller *authhttp.AuthController) *authRouteRegistrar {
	return newAuthRouteRegistrar(controller)
}

func newAuthenticatedAuthRouteRegistrar(controller *authhttp.AuthController) *authRouteRegistrar {
	return newAuthRouteRegistrar(controller)
}

func (r *authRouteRegistrar) RouteKey() string {
	return "auth"
}

func (r *authRouteRegistrar) RegisterPublicRoutes(group *gin.RouterGroup) {
	authhttp.RegisterPublicRoutes(group.Group("/auth"), r.controller)
}

func (r *authRouteRegistrar) RegisterAuthenticatedRoutes(group *gin.RouterGroup) {
	authhttp.RegisterProtectedRoutes(group.Group("/auth"), r.controller)
}
