package command

import (
	"context"

	"go.uber.org/fx"

	permissionapplication "github.com/aegiscore/user-service/internal/features/permission/application"
)

type permissionCommandService struct {
	policyChanges permissionapplication.PolicyChangeNotifier
	store         permissionapplication.PermissionStore
}

// PermissionCommandParams 包含权限写侧服务依赖。
type PermissionCommandParams struct {
	fx.In

	Store         permissionapplication.PermissionStore
	PolicyChanges permissionapplication.PolicyChangeNotifier
}

// NewPermissionCommandService 根据权限仓储依赖构造权限写侧服务。
func NewPermissionCommandService(params PermissionCommandParams) PermissionCommandService {
	return &permissionCommandService{store: params.Store, policyChanges: params.PolicyChanges}
}

func (s *permissionCommandService) notifyPolicyChanged(ctx context.Context, reason string) {
	if s.policyChanges == nil {
		return
	}
	s.policyChanges.NotifyPolicyChanged(ctx, reason)
}
