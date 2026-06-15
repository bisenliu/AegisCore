package command

import permissionapplication "github.com/aegiscore/user-service/internal/features/permission/application"

type permissionCommandService struct {
	store permissionapplication.PermissionStore
}

// NewPermissionCommandService 根据权限仓储依赖构造权限写侧服务。
func NewPermissionCommandService(store permissionapplication.PermissionStore) PermissionCommandService {
	return &permissionCommandService{store: store}
}
