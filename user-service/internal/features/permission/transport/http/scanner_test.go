package permissionhttp

import (
	"context"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"

	permissionapplication "github.com/aegiscore/user-service/internal/features/permission/application"
	"github.com/aegiscore/user-service/internal/features/permission/application/rbacbaseline"
	rolehttp "github.com/aegiscore/user-service/internal/features/role/transport/http"
	userhttp "github.com/aegiscore/user-service/internal/features/user/transport/http"
)

func TestRouteCatalogScanner(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/healthz", func(*gin.Context) {})
	engine.POST("/api/v1/auth/login", func(*gin.Context) {})
	engine.GET("/api/v1/users", func(*gin.Context) {})
	engine.POST("/api/v1/users", func(*gin.Context) {})

	scanner := NewRouteCatalogScanner(RouteCatalogScannerParams{Engine: engine})
	routes, err := scanner.DiscoverRoutes(context.Background())
	if err != nil {
		t.Fatalf("DiscoverRoutes: %v", err)
	}
	if len(routes) != 2 {
		t.Fatalf("routes = %#v, want 2 authorizable routes", routes)
	}
}

func TestRouteCatalogScannerFiltersAuthorizableRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/healthz", func(*gin.Context) {})
	engine.OPTIONS("/api/v1/users", func(*gin.Context) {})
	engine.POST("/api/v1/auth/login", func(*gin.Context) {})
	engine.POST("/api/v1/auth/refresh", func(*gin.Context) {})
	engine.POST("/api/v1/auth/change-password", func(*gin.Context) {})
	engine.POST("/api/v1/auth/logout", func(*gin.Context) {})
	engine.POST("/api/v1/auth/logout-all", func(*gin.Context) {})
	engine.GET("/api/v1/roles", func(*gin.Context) {})
	engine.GET("/api/v1/permissions/route-diff", func(*gin.Context) {})

	scanner := NewRouteCatalogScanner(RouteCatalogScannerParams{Engine: engine})
	routes, err := scanner.DiscoverRoutes(context.Background())
	if err != nil {
		t.Fatalf("DiscoverRoutes: %v", err)
	}
	if len(routes) != 2 {
		t.Fatalf("routes = %#v, want 2 authorizable routes", routes)
	}
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

	scanner := NewRouteCatalogScanner(RouteCatalogScannerParams{Engine: engine})
	routes, err := scanner.DiscoverRoutes(context.Background())
	if err != nil {
		t.Fatalf("DiscoverRoutes: %v", err)
	}
	discovered := discoveredRouteSet(routes)
	baseline := baselineRouteSet()
	for route := range baseline {
		if _, ok := discovered[route]; !ok {
			t.Fatalf("scanner missing baseline route %s", route)
		}
	}
	for route := range discovered {
		if _, ok := baseline[route]; !ok {
			t.Fatalf("baseline missing scanned route %s", route)
		}
	}
}

func assertDiscoveredRoute(t *testing.T, routes []permissionapplication.DiscoveredRoute, method string, path string) {
	t.Helper()
	for _, route := range routes {
		if route.Method == method && route.Path == path {
			return
		}
	}
	t.Fatalf("missing route %s %s in %#v", method, path, routes)
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
