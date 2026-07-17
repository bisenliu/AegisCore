package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	permissionapplication "github.com/aegiscore/user-service/internal/features/permission/application"
	permissiondomain "github.com/aegiscore/user-service/internal/features/permission/domain"
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

// GetActiveByPermissionID 校验权限存在且已启用，并返回角色绑定所需的最小权限视图。
func (l *PermissionLookup) GetActiveByPermissionID(ctx context.Context, permissionID uuid.UUID) (*roleapplication.PermissionReference, error) {
	permission, err := l.store.GetByPermissionID(ctx, permissionID)
	if err != nil {
		return nil, err
	}
	if !permission.Active {
		return nil, fmt.Errorf("%w: permission inactive", permissiondomain.ErrPermissionNotFound)
	}
	return &roleapplication.PermissionReference{ID: permission.ID, PermissionID: permission.PermissionID, Name: permission.Name, Description: permission.Description, Module: permission.Module, HTTPMethod: permission.HTTPMethod, PathTemplate: permission.PathTemplate, Active: permission.Active, IsSystem: permission.IsSystem, CreatedAt: permission.CreatedAt, UpdatedAt: permission.UpdatedAt}, nil
}
