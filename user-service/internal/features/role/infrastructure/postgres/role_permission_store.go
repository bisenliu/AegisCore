package postgres

import (
	"context"
	"errors"
	"fmt"

	"entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"github.com/aegiscore/common/runtime/datastore"
	permissiondomain "github.com/aegiscore/user-service/internal/features/permission/domain"
	roleapplication "github.com/aegiscore/user-service/internal/features/role/application"
	roledomain "github.com/aegiscore/user-service/internal/features/role/domain"
	"github.com/aegiscore/user-service/internal/persistence/ent"
	entpermission "github.com/aegiscore/user-service/internal/persistence/ent/permission"
	entrole "github.com/aegiscore/user-service/internal/persistence/ent/role"
	entrolepermission "github.com/aegiscore/user-service/internal/persistence/ent/rolepermission"
)

type RolePermissionStore struct {
	client *ent.Client
}

var _ roleapplication.RolePermissionStore = (*RolePermissionStore)(nil)
var _ roleapplication.SeedRolePermissionStore = (*RolePermissionStore)(nil)

// NewRolePermissionStore 构造基于 Ent 的角色权限绑定 store。
func NewRolePermissionStore(client *ent.Client) *RolePermissionStore {
	return &RolePermissionStore{client: client}
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
func (s *RolePermissionStore) Add(ctx context.Context, roleID uuid.UUID, permission roleapplication.PermissionReference, change roleapplication.PolicyChange) (roleapplication.PermissionsWriteResult, error) {
	items, write, err := transactPolicyChange(ctx, s.client, "add role permission", change, func(tx *ent.Tx) ([]roleapplication.PermissionReference, error) {
		role, err := lockOrdinaryRole(ctx, tx, roleID)
		if err != nil {
			return nil, err
		}
		lockedPermission, err := s.getLockedPermissionByExternalID(ctx, tx, permission.PermissionID)
		if err != nil {
			return nil, err
		}
		if _, err := tx.RolePermission.Create().SetRoleID(role.ID).SetPermissionID(lockedPermission.ID).Save(ctx); err != nil {
			if ent.IsConstraintError(err) {
				return nil, roledomain.ErrRolePermissionAlreadyExists
			}
			return nil, fmt.Errorf("add role permission role %s permission %s: %w", roleID.String(), permission.PermissionID.String(), err)
		}
		return s.listPermissionsByInternalRoleID(ctx, tx, role.ID, roleID)
	})
	return roleapplication.PermissionsWriteResult{Items: items, Revision: write.Revision}, err
}

// Replace 幂等替换角色的完整权限绑定集合。
// 事务内先校验角色和目标权限，再删除旧绑定并批量创建新绑定。
func (s *RolePermissionStore) Replace(ctx context.Context, roleID uuid.UUID, permissions []roleapplication.PermissionReference, change roleapplication.PolicyChange) (roleapplication.PermissionsWriteResult, error) {
	items, write, err := transactPolicyChange(ctx, s.client, "replace role permissions", change, func(tx *ent.Tx) ([]roleapplication.PermissionReference, error) {
		role, err := lockOrdinaryRole(ctx, tx, roleID)
		if err != nil {
			return nil, err
		}
		lockedPermissions, err := s.lockedPermissionsByExternalIDs(ctx, tx, permissionReferenceIDs(permissions))
		if err != nil {
			return nil, err
		}
		if _, err := tx.RolePermission.Delete().Where(entrolepermission.RoleIDEQ(role.ID)).Exec(ctx); err != nil {
			return nil, fmt.Errorf("delete role permissions for role %s: %w", roleID.String(), err)
		}
		builders := make([]*ent.RolePermissionCreate, 0, len(lockedPermissions))
		for _, permission := range lockedPermissions {
			builders = append(builders, tx.RolePermission.Create().SetRoleID(role.ID).SetPermissionID(permission.ID))
		}
		if len(builders) > 0 {
			if _, err := tx.RolePermission.CreateBulk(builders...).Save(ctx); err != nil {
				return nil, fmt.Errorf("create replacement role permissions for role %s: %w", roleID.String(), err)
			}
		}
		return lockedPermissions, nil
	})
	return roleapplication.PermissionsWriteResult{Items: items, Revision: write.Revision}, err
}

// Remove 删除角色权限绑定。
func (s *RolePermissionStore) Remove(ctx context.Context, roleID uuid.UUID, permissionID uuid.UUID, change roleapplication.PolicyChange) (roleapplication.PermissionsWriteResult, error) {
	items, write, err := transactPolicyChange(ctx, s.client, "remove role permission", change, func(tx *ent.Tx) ([]roleapplication.PermissionReference, error) {
		role, err := lockOrdinaryRole(ctx, tx, roleID)
		if err != nil {
			return nil, err
		}
		permission, err := s.getLockedPermissionByExternalID(ctx, tx, permissionID)
		if err != nil {
			if errors.Is(err, permissiondomain.ErrPermissionNotFound) {
				return nil, roledomain.ErrRolePermissionNotFound
			}
			return nil, err
		}
		deleted, err := tx.RolePermission.Delete().Where(entrolepermission.RoleIDEQ(role.ID), entrolepermission.PermissionIDEQ(permission.ID)).Exec(ctx)
		if err != nil {
			return nil, fmt.Errorf("remove role permission role %s permission %s: %w", roleID.String(), permissionID.String(), err)
		}
		if deleted == 0 {
			return nil, roledomain.ErrRolePermissionNotFound
		}
		return s.listPermissionsByInternalRoleID(ctx, tx, role.ID, roleID)
	})
	return roleapplication.PermissionsWriteResult{Items: items, Revision: write.Revision}, err
}

func (s *RolePermissionStore) listPermissionsByInternalRoleID(ctx context.Context, tx *ent.Tx, internalRoleID int64, roleID uuid.UUID) ([]roleapplication.PermissionReference, error) {
	permissions, err := tx.Permission.Query().
		Where(entpermission.HasRolePermissionsWith(entrolepermission.RoleIDEQ(internalRoleID))).
		Order(entpermission.ByHTTPMethod(), entpermission.ByPathTemplate()).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("list permissions for role %s in mutation transaction: %w", roleID.String(), err)
	}
	return toPermissionReferences(permissions), nil
}

// EnsureSystemBindings 补齐系统角色权限绑定，不删除额外绑定。
// 适用于安全的增量 seed：不会移除人工追加或历史残留绑定。
func (s *RolePermissionStore) EnsureSystemBindings(ctx context.Context, roleID uuid.UUID, permissionIDs []uuid.UUID) (int, error) {
	role, err := s.getRoleByExternalID(ctx, roleID)
	if err != nil {
		return 0, err
	}
	permissions, err := s.permissionsByExternalIDs(ctx, permissionIDs)
	if err != nil {
		return 0, err
	}
	existing, err := s.existingPermissionIDs(ctx, role.ID)
	if err != nil {
		return 0, err
	}
	builders := make([]*ent.RolePermissionCreate, 0, len(permissions))
	for _, permission := range permissions {
		if _, ok := existing[permission.ID]; ok {
			continue
		}
		builders = append(builders, s.client.RolePermission.Create().SetRoleID(role.ID).SetPermissionID(permission.ID))
	}
	if len(builders) == 0 {
		return 0, nil
	}
	if err := s.client.RolePermission.CreateBulk(builders...).
		OnConflict(sql.ConflictColumns(entrolepermission.FieldRoleID, entrolepermission.FieldPermissionID)).
		DoNothing().
		Exec(ctx); err != nil {
		return 0, fmt.Errorf("create seed role permissions for role %s: %w", roleID.String(), err)
	}
	return len(builders), nil
}

// SyncSystemBindings 精确同步系统角色权限绑定。
// 与 EnsureSystemBindings 不同，该方法会删除不在基线中的系统绑定，用于显式收敛超级管理员以外的系统角色权限集合。
func (s *RolePermissionStore) SyncSystemBindings(ctx context.Context, roleID uuid.UUID, permissionIDs []uuid.UUID) (int, int, error) {
	role, err := s.getRoleByExternalID(ctx, roleID)
	if err != nil {
		return 0, 0, err
	}
	permissions, err := s.permissionsByExternalIDs(ctx, permissionIDs)
	if err != nil {
		return 0, 0, err
	}
	desired := make(map[int64]*ent.Permission, len(permissions))
	for _, permission := range permissions {
		desired[permission.ID] = permission
	}
	existing, err := s.existingPermissionIDs(ctx, role.ID)
	if err != nil {
		return 0, 0, err
	}

	tx, finish, err := datastore.BeginTransaction(ctx, entTxStarter{client: s.client})
	if err != nil {
		return 0, 0, fmt.Errorf("begin sync seed role permissions: %w", err)
	}
	defer func() { _ = finish.RollbackUnlessCommitted() }()
	removed := 0
	for permissionID := range existing {
		if _, ok := desired[permissionID]; ok {
			continue
		}
		deleted, err := tx.RolePermission.Delete().Where(entrolepermission.RoleIDEQ(role.ID), entrolepermission.PermissionIDEQ(permissionID)).Exec(ctx)
		if err != nil {
			return 0, 0, finish.Fail(fmt.Errorf("delete seed role permission role %s permission %d: %w", roleID.String(), permissionID, err))
		}
		removed += deleted
	}
	builders := make([]*ent.RolePermissionCreate, 0, len(desired))
	for permissionID, permission := range desired {
		if _, ok := existing[permissionID]; ok {
			continue
		}
		builders = append(builders, tx.RolePermission.Create().SetRoleID(role.ID).SetPermissionID(permission.ID))
	}
	added := len(builders)
	if len(builders) > 0 {
		if err := tx.RolePermission.CreateBulk(builders...).
			OnConflict(sql.ConflictColumns(entrolepermission.FieldRoleID, entrolepermission.FieldPermissionID)).
			DoNothing().
			Exec(ctx); err != nil {
			return 0, 0, finish.Fail(fmt.Errorf("create seed role permissions for role %s: %w", roleID.String(), err))
		}
	}
	if err := finish.Commit(ctx); err != nil {
		return 0, 0, fmt.Errorf("commit sync seed role permissions: %w", err)
	}
	return added, removed, nil
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

func (s *RolePermissionStore) getLockedPermissionByExternalID(ctx context.Context, tx *ent.Tx, permissionID uuid.UUID) (*ent.Permission, error) {
	permission, err := tx.Permission.Query().Where(entpermission.PermissionIDEQ(permissionID)).Only(ctx)
	if err == nil {
		return permission, nil
	}
	if ent.IsNotFound(err) {
		return nil, fmt.Errorf("%w: permission_id %s", permissiondomain.ErrPermissionNotFound, permissionID.String())
	}
	return nil, fmt.Errorf("query permission by permission_id %s: %w", permissionID.String(), err)
}

func (s *RolePermissionStore) lockedPermissionsByExternalIDs(ctx context.Context, tx *ent.Tx, permissionIDs []uuid.UUID) ([]roleapplication.PermissionReference, error) {
	if len(permissionIDs) == 0 {
		return []roleapplication.PermissionReference{}, nil
	}
	found, err := tx.Permission.Query().Where(entpermission.PermissionIDIn(permissionIDs...)).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("query permissions by permission_ids: %w", err)
	}
	byExternalID := make(map[uuid.UUID]*ent.Permission, len(found))
	for _, permission := range found {
		byExternalID[permission.PermissionID] = permission
	}
	permissions := make([]roleapplication.PermissionReference, 0, len(permissionIDs))
	for _, permissionID := range permissionIDs {
		permission, ok := byExternalID[permissionID]
		if !ok {
			return nil, fmt.Errorf("%w: permission_id %s", permissiondomain.ErrPermissionNotFound, permissionID.String())
		}
		permissions = append(permissions, *toPermissionReference(permission))
	}
	return permissions, nil
}

func (s *RolePermissionStore) permissionsByExternalIDs(ctx context.Context, permissionIDs []uuid.UUID) ([]*ent.Permission, error) {
	uniqueIDs := make([]uuid.UUID, 0, len(permissionIDs))
	seen := make(map[uuid.UUID]struct{}, len(permissionIDs))
	for _, permissionID := range permissionIDs {
		if _, ok := seen[permissionID]; ok {
			continue
		}
		seen[permissionID] = struct{}{}
		uniqueIDs = append(uniqueIDs, permissionID)
	}
	if len(uniqueIDs) == 0 {
		return []*ent.Permission{}, nil
	}
	found, err := s.client.Permission.Query().Where(entpermission.PermissionIDIn(uniqueIDs...)).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("query permissions by permission_ids: %w", err)
	}
	byExternalID := make(map[uuid.UUID]*ent.Permission, len(found))
	for _, permission := range found {
		byExternalID[permission.PermissionID] = permission
	}
	permissions := make([]*ent.Permission, 0, len(uniqueIDs))
	for _, permissionID := range uniqueIDs {
		permission, ok := byExternalID[permissionID]
		if !ok {
			return nil, fmt.Errorf("%w: permission_id %s", roledomain.ErrRolePermissionNotFound, permissionID.String())
		}
		permissions = append(permissions, permission)
	}
	return permissions, nil
}

func permissionReferenceIDs(permissions []roleapplication.PermissionReference) []uuid.UUID {
	permissionIDs := make([]uuid.UUID, 0, len(permissions))
	seen := make(map[uuid.UUID]struct{}, len(permissions))
	for _, permission := range permissions {
		if _, ok := seen[permission.PermissionID]; ok {
			continue
		}
		seen[permission.PermissionID] = struct{}{}
		permissionIDs = append(permissionIDs, permission.PermissionID)
	}
	return permissionIDs
}

func (s *RolePermissionStore) existingPermissionIDs(ctx context.Context, roleID int64) (map[int64]struct{}, error) {
	bindings, err := s.client.RolePermission.Query().Where(entrolepermission.RoleIDEQ(roleID)).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("list seed role permissions for role %d: %w", roleID, err)
	}
	existing := make(map[int64]struct{}, len(bindings))
	for _, binding := range bindings {
		existing[binding.PermissionID] = struct{}{}
	}
	return existing, nil
}

func toPermissionReference(permission *ent.Permission) *roleapplication.PermissionReference {
	if permission == nil {
		return nil
	}
	return &roleapplication.PermissionReference{ID: permission.ID, PermissionID: permission.PermissionID, Name: permission.Name, Description: permission.Description, Module: permission.Module, HTTPMethod: permission.HTTPMethod, PathTemplate: permission.PathTemplate, CreatedAt: permission.CreatedAt, UpdatedAt: permission.UpdatedAt}
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
