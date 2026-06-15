package domain

import (
	"fmt"
	"strings"
)

// RouteIdentity 是权限目录和路由发现比较时使用的稳定身份。
type RouteIdentity struct {
	Method       string
	PathTemplate string
}

// NewRouteIdentity 创建规范化后的路由身份。
func NewRouteIdentity(method string, pathTemplate string) (RouteIdentity, error) {
	normalizedMethod, err := NormalizeHTTPMethod(method)
	if err != nil {
		return RouteIdentity{}, err
	}
	normalizedPath, err := NormalizePathTemplate(pathTemplate)
	if err != nil {
		return RouteIdentity{}, err
	}
	return RouteIdentity{Method: normalizedMethod, PathTemplate: normalizedPath}, nil
}

// Key 返回可用于集合比较的路由身份键。
func (r RouteIdentity) Key() string {
	return r.Method + " " + r.PathTemplate
}

// NormalizePathTemplate 规范化并校验权限路径模板。
func NormalizePathTemplate(pathTemplate string) (string, error) {
	normalized := strings.TrimSpace(pathTemplate)
	if normalized == "" {
		return "", fmt.Errorf("%w: path template is required", ErrPermissionInvalid)
	}
	if !strings.HasPrefix(normalized, "/") {
		return "", fmt.Errorf("%w: path template must be absolute", ErrPermissionInvalid)
	}
	if !strings.HasPrefix(normalized, "/api/v1/") && normalized != "/api/v1" {
		return "", fmt.Errorf("%w: path template must be under /api/v1", ErrPermissionInvalid)
	}
	return normalized, nil
}
