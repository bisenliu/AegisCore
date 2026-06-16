package permissionhttp

import (
	"context"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"

	permissionapplication "github.com/aegiscore/user-service/internal/features/permission/application"
)

type RouteCatalogScanner struct {
	engine *gin.Engine
}

var _ permissionapplication.RouteCatalogScanner = (*RouteCatalogScanner)(nil)

// RouteCatalogScannerParams 包含 Gin route scanner 所需的 Fx 输入。
type RouteCatalogScannerParams struct {
	fx.In

	Engine *gin.Engine
}

// NewRouteCatalogScanner 构造基于 Gin engine 的只读路由扫描器。
func NewRouteCatalogScanner(params RouteCatalogScannerParams) *RouteCatalogScanner {
	return &RouteCatalogScanner{engine: params.Engine}
}

// DiscoverRoutes 返回当前 Gin engine 上可授权的 HTTP 路由。
func (s *RouteCatalogScanner) DiscoverRoutes(_ context.Context) ([]permissionapplication.DiscoveredRoute, error) {
	routes := s.engine.Routes()
	result := make([]permissionapplication.DiscoveredRoute, 0, len(routes))
	for _, route := range routes {
		if !isAuthorizableRoute(route.Method, route.Path) {
			continue
		}
		result = append(result, permissionapplication.DiscoveredRoute{Method: route.Method, Path: route.Path})
	}
	return result, nil
}

func isAuthorizableRoute(method string, path string) bool {
	if method == "OPTIONS" {
		return false
	}
	if !strings.HasPrefix(path, "/api/v1/") {
		return false
	}
	switch path {
	case "/api/v1/auth/login",
		"/api/v1/auth/refresh",
		"/api/v1/auth/change-password",
		"/api/v1/auth/logout",
		"/api/v1/auth/logout-all":
		return false
	}
	return true
}
