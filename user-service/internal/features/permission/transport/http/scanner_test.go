package permissionhttp

import (
	"context"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/stretchr/testify/require"

	permissionapplication "github.com/aegiscore/user-service/internal/features/permission/application"
	rolehttp "github.com/aegiscore/user-service/internal/features/role/transport/http"
	userhttp "github.com/aegiscore/user-service/internal/features/user/transport/http"
	"github.com/aegiscore/user-service/internal/shared/rbacbaseline"
)

func TestRouteCatalogScanner(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/livez", func(*gin.Context) {})
	engine.GET("/metrics", func(*gin.Context) {})
	engine.POST("/api/v1/auth/login", func(*gin.Context) {})
	engine.GET("/api/v1/users", func(*gin.Context) {})
	engine.POST("/api/v1/users", func(*gin.Context) {})

	scanner := NewRouteCatalogScanner(engine)
	routes, err := scanner.DiscoverRoutes(context.Background())
	require.NoError(t, err)
	require.Len(t, routes, 2)
}

func TestRouteCatalogScannerFiltersAuthorizableRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/livez", func(*gin.Context) {})
	engine.GET("/metrics", func(*gin.Context) {})
	engine.GET("/internal/metrics", func(*gin.Context) {})
	engine.OPTIONS("/api/v1/users", func(*gin.Context) {})
	engine.POST("/api/v1/auth/login", func(*gin.Context) {})
	engine.POST("/api/v1/auth/refresh", func(*gin.Context) {})
	engine.POST("/api/v1/auth/change-password", func(*gin.Context) {})
	engine.POST("/api/v1/auth/logout", func(*gin.Context) {})
	engine.POST("/api/v1/auth/logout-all", func(*gin.Context) {})
	engine.GET("/api/v1/roles", func(*gin.Context) {})
	engine.GET("/api/v1/permissions/route-diff", func(*gin.Context) {})

	scanner := NewRouteCatalogScanner(engine)
	routes, err := scanner.DiscoverRoutes(context.Background())
	require.NoError(t, err)
	require.Len(t, routes, 2)
	assertDiscoveredRoute(t, routes, http.MethodGet, "/api/v1/roles")
	assertDiscoveredRoute(t, routes, http.MethodGet, "/api/v1/permissions/route-diff")
}

func TestRouteCatalogScannerMatchesRBACBaseline(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	v1 := engine.Group("/api/v1")
	RegisterRoutes(v1.Group("/permissions"), &PermissionController{})
	rolehttp.RegisterRoleRoutes(v1.Group("/roles"), &rolehttp.RoleController{})
	rolehttp.RegisterUserRoleRoutes(v1.Group("/users"), &rolehttp.RoleController{})
	userhttp.RegisterRoutes(v1.Group("/users"), &userhttp.UserController{})

	scanner := NewRouteCatalogScanner(engine)
	routes, err := scanner.DiscoverRoutes(context.Background())
	require.NoError(t, err)
	discovered := discoveredRouteSet(routes)
	baseline := baselineRouteSet()
	for route := range baseline {
		require.Contains(t, discovered, route, "scanner missing baseline route")
	}
	for route := range discovered {
		require.Contains(t, baseline, route, "baseline missing scanned route")
	}
}

func assertDiscoveredRoute(t *testing.T, routes []permissionapplication.DiscoveredRoute, method string, path string) {
	t.Helper()
	require.Contains(t, discoveredRouteSet(routes), method+" "+path)
}

func discoveredRouteSet(routes []permissionapplication.DiscoveredRoute) map[string]struct{} {
	result := make(map[string]struct{}, len(routes))
	for _, route := range routes {
		result[route.Method+" "+route.Path] = struct{}{}
	}
	return result
}

func baselineRouteSet() map[string]struct{} {
	permissions := rbacbaseline.DefaultPermissions()
	result := make(map[string]struct{}, len(permissions))
	for _, permission := range permissions {
		result[permission.Method+" "+permission.PathTemplate] = struct{}{}
	}
	return result
}
