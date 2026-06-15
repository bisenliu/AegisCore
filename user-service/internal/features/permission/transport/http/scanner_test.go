package permissionhttp

import (
	"context"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"

	permissionapplication "github.com/aegiscore/user-service/internal/features/permission/application"
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

func assertDiscoveredRoute(t *testing.T, routes []permissionapplication.DiscoveredRoute, method string, path string) {
	t.Helper()
	for _, route := range routes {
		if route.Method == method && route.Path == path {
			return
		}
	}
	t.Fatalf("missing route %s %s in %#v", method, path, routes)
}
