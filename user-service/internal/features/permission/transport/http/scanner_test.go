package permissionhttp

import (
	"context"
	"testing"

	"github.com/gin-gonic/gin"
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
