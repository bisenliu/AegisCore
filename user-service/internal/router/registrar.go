package router

import "github.com/gin-gonic/gin"

// PublicRouteRegistrar 注册不需要普通 access token 的 /api/v1 路由。
type PublicRouteRegistrar interface {
	RouteKey() string
	RegisterPublicRoutes(*gin.RouterGroup)
}

// AuthenticatedRouteRegistrar 注册需要认证但不需要 RBAC 授权的 /api/v1 路由。
type AuthenticatedRouteRegistrar interface {
	RouteKey() string
	RegisterAuthenticatedRoutes(*gin.RouterGroup)
}

// AuthorizedRouteRegistrar 注册需要认证和 RBAC 授权的 /api/v1 业务路由。
type AuthorizedRouteRegistrar interface {
	RouteKey() string
	RegisterAuthorizedRoutes(*gin.RouterGroup)
}
