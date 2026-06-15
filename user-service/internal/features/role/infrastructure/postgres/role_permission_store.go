package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"go.uber.org/fx"

	"github.com/aegiscore/user-service/ent"
	entpermission "github.com/aegiscore/user-service/ent/permission"
	entrole "github.com/aegiscore/user-service/ent/role"
	entrolepermission "github.com/aegiscore/user-service/ent/rolepermission"
	roleapplication "github.com/aegiscore/user-service/internal/features/role/application"
	roledomain "github.com/aegiscore/user-service/internal/features/role/domain"
)

type RolePermissionStore struct {
	client *ent.Client
}

var _ roleapplication.RolePermissionStore = (*RolePermissionStore)(nil)

// RolePermissionStoreParams 包含 PostgreSQL-backed 角色权限绑定 store 所需的 Fx 输入。
type RolePermissionStoreParams struct {
	fx.In

	Client *ent.Client `name:"user_db"`
}

// NewRolePermissionStore 构造基于 Ent 的角色权限绑定 store。
func NewRolePermissionStore(params RolePermissionStoreParams) *RolePermissionStore {
	return &RolePermissionStore{client: params.Client}
}

// ListByRoleID 按角色外部 ID 返回已绑定权限列表。
func (s *RolePermissionStore) ListByRoleID(ctx context.Context, roleID uuid.UUID) ([]roleapplication.PermissionReference, error) {
	role, err := s.getRoleByExternalID(ctx, roleID)
	if err != nil {
		return nil, err
	}
	permissions, err := s.client.Permission.Query().
		Where(entpermission.HasRolePermissionsWith(entrolepermission.RoleIDEQ(role.ID))).
		Order(entpermission.ByHTTPMethod(), entpermission.ByPathTemplate()).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("list permissions for role %s: %w", roleID.String(), err)
	}
	return toPermissionReferences(permissions), nil
}

// Add 新增角色权限绑定。
func (s *RolePermissionStore) Add(ctx context.Context, roleID uuid.UUID, permission roleapplication.PermissionReference) error {
	role, err := s.getRoleByExternalID(ctx, roleID)
	if err != nil {
		return err
	}
	_, err = s.client.RolePermission.Create().SetRoleID(role.ID).SetPermissionID(permission.ID).Save(ctx)
	if err == nil {
		return nil
	}
	if ent.IsConstraintError(err) {
		return roledomain.ErrRolePermissionAlreadyExists
	}
	return fmt.Errorf("add role permission role %s permission %s: %w", roleID.String(), permission.PermissionID.String(), err)
}

// Replace 幂等替换角色的完整权限绑定集合。
func (s *RolePermissionStore) Replace(ctx context.Context, roleID uuid.UUID, permissions []roleapplication.PermissionReference) ([]roleapplication.PermissionReference, error) {
	role, err := s.getRoleByExternalID(ctx, roleID)
	if err != nil {
		return nil, err
	}
	tx, err := s.client.Tx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin replace role permissions: %w", err)
	}
	if _, err := tx.RolePermission.Delete().Where(entrolepermission.RoleIDEQ(role.ID)).Exec(ctx); err != nil {
		return nil, rollback(tx, fmt.Errorf("delete role permissions for role %s: %w", roleID.String(), err))
	}
	for _, permission := range permissions {
		if _, err := tx.RolePermission.Create().SetRoleID(role.ID).SetPermissionID(permission.ID).Save(ctx); err != nil {
			return nil, rollback(tx, fmt.Errorf("create replacement role permission role %s permission %s: %w", roleID.String(), permission.PermissionID.String(), err))
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit replace role permissions: %w", err)
	}
	return permissions, nil
}

// Remove 删除角色权限绑定。
func (s *RolePermissionStore) Remove(ctx context.Context, roleID uuid.UUID, permissionID uuid.UUID) error {
	role, err := s.getRoleByExternalID(ctx, roleID)
	if err != nil {
		return err
	}
	permission, err := s.getPermissionByExternalID(ctx, permissionID)
	if err != nil {
		return err
	}
	deleted, err := s.client.RolePermission.Delete().Where(entrolepermission.RoleIDEQ(role.ID), entrolepermission.PermissionIDEQ(permission.ID)).Exec(ctx)
	if err != nil {
		return fmt.Errorf("remove role permission role %s permission %s: %w", roleID.String(), permissionID.String(), err)
	}
	if deleted == 0 {
		return roledomain.ErrRolePermissionNotFound
	}
	return nil
}

func (s *RolePermissionStore) getRoleByExternalID(ctx context.Context, roleID uuid.UUID) (*ent.Role, error) {
	role, err := s.client.Role.Query().Where(entrole.RoleIDEQ(roleID)).Only(ctx)
	if err == nil {
		return role, nil
	}
	if ent.IsNotFound(err) {
		return nil, roledomain.ErrRoleNotFound
	}
	return nil, fmt.Errorf("query role by role_id %s: %w", roleID.String(), err)
}

func (s *RolePermissionStore) getPermissionByExternalID(ctx context.Context, permissionID uuid.UUID) (*ent.Permission, error) {
	permission, err := s.client.Permission.Query().Where(entpermission.PermissionIDEQ(permissionID)).Only(ctx)
	if err == nil {
		return permission, nil
	}
	if ent.IsNotFound(err) {
		return nil, roledomain.ErrRolePermissionNotFound
	}
	return nil, fmt.Errorf("query permission by permission_id %s: %w", permissionID.String(), err)
}

func toPermissionReference(permission *ent.Permission) *roleapplication.PermissionReference {
	if permission == nil {
		return nil
	}
	return &roleapplication.PermissionReference{ID: permission.ID, PermissionID: permission.PermissionID, Name: permission.Name, Description: permission.Description, Module: permission.Module, HTTPMethod: permission.HTTPMethod, PathTemplate: permission.PathTemplate, Active: permission.Active, IsSystem: permission.IsSystem, CreatedAt: permission.CreatedAt, UpdatedAt: permission.UpdatedAt}
}

func toPermissionReferences(permissions []*ent.Permission) []roleapplication.PermissionReference {
	result := make([]roleapplication.PermissionReference, 0, len(permissions))
	for _, permission := range permissions {
		if mapped := toPermissionReference(permission); mapped != nil {
			result = append(result, *mapped)
		}
	}
	return result
}
