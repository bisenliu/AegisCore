package validators

import (
	"fmt"
	"strings"

	roledomain "github.com/aegiscore/user-service/internal/features/role/domain"
)

// NormalizeRoleFields 清洗并校验角色输入的通用字段。
func NormalizeRoleFields(name string, description string) (string, string, error) {
	normalizedName := strings.TrimSpace(name)
	if normalizedName == "" {
		return "", "", fmt.Errorf("%w: name is required", roledomain.ErrRoleInvalid)
	}
	return normalizedName, strings.TrimSpace(description), nil
}
