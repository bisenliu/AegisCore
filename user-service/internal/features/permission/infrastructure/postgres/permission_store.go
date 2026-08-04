package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	permissionapplication "github.com/aegiscore/user-service/internal/features/permission/application"
	permissiondomain "github.com/aegiscore/user-service/internal/features/permission/domain"
	"github.com/aegiscore/user-service/internal/persistence/ent"
	entpermission "github.com/aegiscore/user-service/internal/persistence/ent/permission"
	entrole "github.com/aegiscore/user-service/internal/persistence/ent/role"
	entrolepermission "github.com/aegiscore/user-service/internal/persistence/ent/rolepermission"
	entuser "github.com/aegiscore/user-service/internal/persistence/ent/user"
	entuserrole "github.com/aegiscore/user-service/internal/persistence/ent/userrole"
)

type PermissionStore struct {
	client *ent.Client
}

var _ permissionapplication.PermissionStore = (*PermissionStore)(nil)
var _ permissionapplication.SeedPermissionStore = (*PermissionStore)(nil)

// NewPermissionStore 构造基于 Ent 的权限 store。
func NewPermissionStore(client *ent.Client) *PermissionStore {
	return &PermissionStore{client: client}
}

// GetByPermissionID 按外部 UUID 返回权限记录。
func (s *PermissionStore) GetByPermissionID(ctx context.Context, permissionID uuid.UUID) (*permissiondomain.Permission, error) {
	found, err := s.client.Permission.Query().Where(entpermission.PermissionIDEQ(permissionID)).Only(ctx)
	if err == nil {
		return toModel(found), nil
	}
	if ent.IsNotFound(err) {
		return nil, permissiondomain.ErrPermissionNotFound
	}
	return nil, fmt.Errorf("query permission by permission_id %s: %w", permissionID.String(), err)
}

// GetByPermissionIDs 按首次出现顺序批量返回去重后的权限记录。
func (s *PermissionStore) GetByPermissionIDs(ctx context.Context, permissionIDs []uuid.UUID) ([]permissiondomain.Permission, error) {
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
		return []permissiondomain.Permission{}, nil
	}

	found, err := s.client.Permission.Query().Where(entpermission.PermissionIDIn(uniqueIDs...)).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("query permissions by permission_ids: %w", err)
	}
	byPermissionID := make(map[uuid.UUID]permissiondomain.Permission, len(found))
	for _, entPermission := range found {
		if mapped := toModel(entPermission); mapped != nil {
			byPermissionID[mapped.PermissionID] = *mapped
		}
	}

	permissions := make([]permissiondomain.Permission, 0, len(uniqueIDs))
	for _, permissionID := range uniqueIDs {
		permission, ok := byPermissionID[permissionID]
		if !ok {
			return nil, fmt.Errorf("%w: permission_id %s", permissiondomain.ErrPermissionNotFound, permissionID.String())
		}
		permissions = append(permissions, permission)
	}
	return permissions, nil
}

// List 返回全部匹配权限记录。
func (s *PermissionStore) List(ctx context.Context, input permissionapplication.ListPermissionsInput) ([]permissiondomain.Permission, error) {
	predicates := buildListPredicates(input)
	permissions, err := s.client.Permission.Query().
		Where(predicates...).
		Order(entpermission.ByPermissionID()).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("list permissions: %w", err)
	}
	return toModels(permissions), nil
}

// ListEffectiveByUserID 返回用户经由启用角色绑定获得的权限。
func (s *PermissionStore) ListEffectiveByUserID(ctx context.Context, userID uuid.UUID) ([]permissiondomain.Permission, error) {
	permissions, err := s.client.Permission.Query().
		Where(
			entpermission.HasRolePermissionsWith(
				entrolepermission.HasRoleWith(
					entrole.ActiveEQ(true),
					entrole.HasUserRolesWith(
						entuserrole.HasUserWith(entuser.UserIDEQ(userID), entuser.DeletedAtIsNil()),
					),
				),
			),
		).
		Order(entpermission.ByHTTPMethod(), entpermission.ByPathTemplate()).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("list effective permissions for user %s: %w", userID.String(), err)
	}
	return toModels(permissions), nil
}

// UpsertPermission 按 permission_id 幂等写入权限 seed 数据。
func (s *PermissionStore) UpsertPermission(ctx context.Context, input permissionapplication.SeedPermissionInput) (*permissiondomain.Permission, bool, error) {
	existing, err := s.client.Permission.Query().Where(entpermission.PermissionIDEQ(input.PermissionID)).Only(ctx)
	if err != nil && !ent.IsNotFound(err) {
		return nil, false, fmt.Errorf("query seed permission %s: %w", input.PermissionID.String(), err)
	}
	if ent.IsNotFound(err) {
		created, err := s.client.Permission.Create().
			SetPermissionID(input.PermissionID).
			SetName(input.Name).
			SetDescription(input.Description).
			SetModule(input.Module).
			SetHTTPMethod(input.HTTPMethod).
			SetPathTemplate(input.PathTemplate).
			Save(ctx)
		if err == nil {
			return toModel(created), true, nil
		}
		if !ent.IsConstraintError(err) {
			return nil, false, fmt.Errorf("create seed permission %s %s: %w", input.HTTPMethod, input.PathTemplate, err)
		}

		// 并发 seed 可能已经创建同一 permission_id；只对该情况继续幂等更新。
		existing, err = s.client.Permission.Query().Where(entpermission.PermissionIDEQ(input.PermissionID)).Only(ctx)
		if err == nil {
			// 继续执行下方更新。
		} else if !ent.IsNotFound(err) {
			return nil, false, fmt.Errorf("query seed permission %s after create conflict: %w", input.PermissionID.String(), err)
		} else {
			conflicting, conflictErr := s.client.Permission.Query().
				Where(entpermission.HTTPMethodEQ(input.HTTPMethod), entpermission.PathTemplateEQ(input.PathTemplate)).
				Only(ctx)
			if conflictErr == nil {
				return nil, false, fmt.Errorf("seed permission %s route %s %s conflicts with permission_id %s", input.PermissionID.String(), input.HTTPMethod, input.PathTemplate, conflicting.PermissionID.String())
			}
			if !ent.IsNotFound(conflictErr) {
				return nil, false, fmt.Errorf("query conflicting seed permission %s %s: %w", input.HTTPMethod, input.PathTemplate, conflictErr)
			}
			return nil, false, fmt.Errorf("create seed permission %s %s: %w", input.HTTPMethod, input.PathTemplate, err)
		}
	}

	updated, err := s.client.Permission.UpdateOneID(existing.ID).
		SetName(input.Name).
		SetDescription(input.Description).
		SetModule(input.Module).
		SetHTTPMethod(input.HTTPMethod).
		SetPathTemplate(input.PathTemplate).
		Save(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("update seed permission %s %s: %w", input.HTTPMethod, input.PathTemplate, err)
	}
	return toModel(updated), false, nil
}

func toModel(entPermission *ent.Permission) *permissiondomain.Permission {
	if entPermission == nil {
		return nil
	}
	return &permissiondomain.Permission{ID: entPermission.ID, PermissionID: entPermission.PermissionID, Name: entPermission.Name, Description: entPermission.Description, Module: entPermission.Module, HTTPMethod: entPermission.HTTPMethod, PathTemplate: entPermission.PathTemplate, CreatedAt: entPermission.CreatedAt, UpdatedAt: entPermission.UpdatedAt}
}

func toModels(permissions []*ent.Permission) []permissiondomain.Permission {
	result := make([]permissiondomain.Permission, 0, len(permissions))
	for _, entPermission := range permissions {
		if mapped := toModel(entPermission); mapped != nil {
			result = append(result, *mapped)
		}
	}
	return result
}
