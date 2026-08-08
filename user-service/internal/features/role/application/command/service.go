package command

import (
	"context"
	"errors"

	"github.com/google/uuid"

	permissionapplication "github.com/aegiscore/user-service/internal/features/permission/application"
	roleapplication "github.com/aegiscore/user-service/internal/features/role/application"
)

type roleCommandService struct {
	policyChanges   PolicyChangeNotifier
	permissions     roleapplication.PermissionLookup
	rolePermissions roleapplication.RolePermissionStore
	roles           roleapplication.RoleStore
	userRoles       roleapplication.UserRoleStore
}

// PolicyChangeNotifier 消费数据库已提交 revision 执行即时 policy 同步。
type PolicyChangeNotifier interface {
	NotifyPolicyChanged(ctx context.Context, revision int64, change permissionapplication.PolicyChange) error
	NotifyUserRoleChanged(ctx context.Context, revision int64, change permissionapplication.PolicyChange) error
}

// NewRoleCommandService 根据角色相关端口构造角色写侧服务。
func NewRoleCommandService(
	roles roleapplication.RoleStore,
	userRoles roleapplication.UserRoleStore,
	rolePermissions roleapplication.RolePermissionStore,
	permissions roleapplication.PermissionLookup,
	policyChanges PolicyChangeNotifier,
) (RoleCommandService, error) {
	if policyChanges == nil {
		return nil, errors.New("role policy change notifier is required")
	}
	return &roleCommandService{
		roles:           roles,
		userRoles:       userRoles,
		rolePermissions: rolePermissions,
		permissions:     permissions,
		policyChanges:   policyChanges,
	}, nil
}

func (s *roleCommandService) notifyPolicyChanged(ctx context.Context, revision int64, reason string) error {
	return s.policyChanges.NotifyPolicyChanged(ctx, revision, permissionapplication.NewPolicyReloadChange(reason))
}

func (s *roleCommandService) notifyUserRoleChanged(ctx context.Context, revision int64, reason string, userID uuid.UUID, roleID uuid.UUID) error {
	return s.policyChanges.NotifyUserRoleChanged(ctx, revision, permissionapplication.NewUserRoleChange(reason, userID, roleID))
}
