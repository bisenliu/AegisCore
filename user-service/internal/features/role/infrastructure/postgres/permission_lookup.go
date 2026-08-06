package postgres

import (
	"context"

	"github.com/google/uuid"

	permissionapplication "github.com/aegiscore/user-service/internal/features/permission/application"
	permissiondomain "github.com/aegiscore/user-service/internal/features/permission/domain"
	roleapplication "github.com/aegiscore/user-service/internal/features/role/application"
)

// PermissionLookup 将 permission feature 的查询端口适配为 role application 所需的最小权限引用。
type PermissionLookup struct {
	store permissionapplication.PermissionStore
}

var _ roleapplication.PermissionLookup = (*PermissionLookup)(nil)

// NewPermissionLookup 构造通过 permission feature 端口校验权限的 adapter。
func NewPermissionLookup(store permissionapplication.PermissionStore) *PermissionLookup {
	return &PermissionLookup{store: store}
}

// GetByPermissionID 校验权限存在，并返回角色绑定所需的最小权限视图。
func (l *PermissionLookup) GetByPermissionID(ctx context.Context, permissionID uuid.UUID) (*roleapplication.PermissionReference, error) {
	permission, err := l.store.GetByPermissionID(ctx, permissionID)
	if err != nil {
		return nil, err
	}
	reference := toPermissionLookupReference(*permission)
	return &reference, nil
}

// GetByPermissionIDs 批量校验权限，并按查询端口返回顺序映射最小权限视图。
func (l *PermissionLookup) GetByPermissionIDs(ctx context.Context, permissionIDs []uuid.UUID) ([]roleapplication.PermissionReference, error) {
	permissions, err := l.store.GetByPermissionIDs(ctx, permissionIDs)
	if err != nil {
		return nil, err
	}
	references := make([]roleapplication.PermissionReference, 0, len(permissions))
	for _, permission := range permissions {
		references = append(references, toPermissionLookupReference(permission))
	}
	return references, nil
}

func toPermissionLookupReference(permission permissiondomain.Permission) roleapplication.PermissionReference {
	return roleapplication.PermissionReference{ID: permission.ID, PermissionID: permission.PermissionID, Name: permission.Name, Description: permission.Description, Module: permission.Module, HTTPMethod: permission.HTTPMethod, PathTemplate: permission.PathTemplate, CreatedAt: permission.CreatedAt, UpdatedAt: permission.UpdatedAt}
}
