package command

import (
	"context"
	"errors"

	"github.com/google/uuid"

	permissionapplication "github.com/aegiscore/user-service/internal/features/permission/application"
	roleapplication "github.com/aegiscore/user-service/internal/features/role/application"
)

type roleCommandService struct {
	policyChanges   permissionapplication.PolicyChangeNotifier
	permissions     roleapplication.PermissionLookup
	rolePermissions roleapplication.RolePermissionStore
	roles           roleapplication.RoleStore
	userRoles       roleapplication.UserRoleStore
}

// NewRoleCommandService 根据角色相关端口构造角色写侧服务。
func NewRoleCommandService(
	roles roleapplication.RoleStore,
	userRoles roleapplication.UserRoleStore,
	rolePermissions roleapplication.RolePermissionStore,
	permissions roleapplication.PermissionLookup,
	policyChanges permissionapplication.PolicyChangeNotifier,
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

func (s *roleCommandService) notifyPolicyChanged(ctx context.Context, reason string) error {
	return s.policyChanges.NotifyPolicyChanged(ctx, permissionapplication.NewPolicyReloadChange(reason))
}

func (s *roleCommandService) notifyUserRoleChanged(ctx context.Context, reason string, userID uuid.UUID, roleID uuid.UUID) error {
	return s.policyChanges.NotifyPolicyChanged(ctx, permissionapplication.NewUserRoleChange(reason, userID, roleID))
}
