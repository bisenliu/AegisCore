package domain

import (
	"fmt"
	"strings"
)

var allowedHTTPMethods = map[string]struct{}{
	"GET":    {},
	"POST":   {},
	"PUT":    {},
	"PATCH":  {},
	"DELETE": {},
}

// NormalizeHTTPMethod 规范化并校验权限 HTTP 方法。
func NormalizeHTTPMethod(method string) (string, error) {
	normalized := strings.ToUpper(strings.TrimSpace(method))
	if _, ok := allowedHTTPMethods[normalized]; !ok {
		return "", fmt.Errorf("%w: unsupported http method", ErrPermissionInvalid)
	}
	return normalized, nil
}
