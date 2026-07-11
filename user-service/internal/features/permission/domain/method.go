package domain

import (
	"fmt"
	"strings"
)

// NormalizeHTTPMethod 规范化并校验权限 HTTP 方法。
func NormalizeHTTPMethod(method string) (string, error) {
	normalized := strings.ToUpper(strings.TrimSpace(method))
	switch normalized {
	case "GET", "POST", "PUT", "PATCH", "DELETE":
		return normalized, nil
	default:
		return "", fmt.Errorf("%w: unsupported http method", ErrPermissionInvalid)
	}
}
