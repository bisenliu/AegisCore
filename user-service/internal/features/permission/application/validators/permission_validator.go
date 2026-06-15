package validators

import (
	"fmt"
	"strings"

	permissiondomain "github.com/aegiscore/user-service/internal/features/permission/domain"
)

// NormalizePermissionFields 清洗并校验权限输入的通用字段。
func NormalizePermissionFields(name string, description string, module string, method string, pathTemplate string) (string, string, string, permissiondomain.RouteIdentity, error) {
	normalizedName := strings.TrimSpace(name)
	if normalizedName == "" {
		return "", "", "", permissiondomain.RouteIdentity{}, fmt.Errorf("%w: name is required", permissiondomain.ErrPermissionInvalid)
	}
	normalizedModule := strings.TrimSpace(module)
	if normalizedModule == "" {
		return "", "", "", permissiondomain.RouteIdentity{}, fmt.Errorf("%w: module is required", permissiondomain.ErrPermissionInvalid)
	}
	identity, err := permissiondomain.NewRouteIdentity(method, pathTemplate)
	if err != nil {
		return "", "", "", permissiondomain.RouteIdentity{}, err
	}
	return normalizedName, strings.TrimSpace(description), normalizedModule, identity, nil
}

// NormalizeOptionalHTTPMethod 清洗可选 HTTP 方法过滤条件。
func NormalizeOptionalHTTPMethod(method string) (string, error) {
	if strings.TrimSpace(method) == "" {
		return "", nil
	}
	return permissiondomain.NormalizeHTTPMethod(method)
}

// NormalizeOptionalModule 清洗可选模块过滤条件。
func NormalizeOptionalModule(module string) string {
	return strings.TrimSpace(module)
}
