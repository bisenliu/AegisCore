package postgres

import (
	"context"

	"github.com/google/uuid"

	permissionapplication "github.com/aegiscore/user-service/internal/features/permission/application"
	roleapplication "github.com/aegiscore/user-service/internal/features/role/application"
)

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
	return &roleapplication.PermissionReference{ID: permission.ID, PermissionID: permission.PermissionID, Name: permission.Name, Description: permission.Description, Module: permission.Module, HTTPMethod: permission.HTTPMethod, PathTemplate: permission.PathTemplate, CreatedAt: permission.CreatedAt, UpdatedAt: permission.UpdatedAt}, nil
}
