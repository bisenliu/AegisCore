package command

import (
	"context"

	permissionapplication "github.com/aegiscore/user-service/internal/features/permission/application"
)

type permissionCommandService struct {
	policyChanges permissionapplication.PolicyChangeNotifier
	store         permissionapplication.PermissionStore
}

// NewPermissionCommandService 根据权限仓储依赖构造权限写侧服务。
func NewPermissionCommandService(store permissionapplication.PermissionStore, policyChanges permissionapplication.PolicyChangeNotifier) PermissionCommandService {
	if policyChanges == nil {
		panic("permission policy change notifier is required")
	}
	return &permissionCommandService{store: store, policyChanges: policyChanges}
}

func (s *permissionCommandService) notifyPolicyChanged(ctx context.Context, reason string) error {
	return s.policyChanges.NotifyPolicyChanged(ctx, permissionapplication.NewPolicyReloadChange(reason))
}
