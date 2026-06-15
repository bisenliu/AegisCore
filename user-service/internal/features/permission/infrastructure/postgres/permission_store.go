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
	entuser "github.com/aegiscore/user-service/ent/user"
	entuserrole "github.com/aegiscore/user-service/ent/userrole"
	permissionapplication "github.com/aegiscore/user-service/internal/features/permission/application"
	permissiondomain "github.com/aegiscore/user-service/internal/features/permission/domain"
)

type PermissionStore struct {
	client *ent.Client
}

var _ permissionapplication.PermissionStore = (*PermissionStore)(nil)

// PermissionStoreParams 包含 PostgreSQL-backed 权限 store 所需的 Fx 输入。
type PermissionStoreParams struct {
	fx.In

	Client *ent.Client `name:"user_db"`
}

// NewPermissionStore 构造基于 Ent 的权限 store。
func NewPermissionStore(params PermissionStoreParams) *PermissionStore {
	return &PermissionStore{client: params.Client}
}

// Create 插入权限记录，并将唯一约束冲突映射为 ErrPermissionAlreadyExists。
func (s *PermissionStore) Create(ctx context.Context, input permissionapplication.CreatePermissionInput) (*permissiondomain.Permission, error) {
	created, err := s.client.Permission.Create().
		SetPermissionID(input.PermissionID).
		SetName(input.Name).
		SetDescription(input.Description).
		SetModule(input.Module).
		SetHTTPMethod(input.HTTPMethod).
		SetPathTemplate(input.PathTemplate).
		SetActive(input.Active).
		SetIsSystem(input.IsSystem).
		Save(ctx)
	if err == nil {
		return toModel(created), nil
	}
	if ent.IsConstraintError(err) {
		return nil, permissiondomain.ErrPermissionAlreadyExists
	}
	return nil, fmt.Errorf("create permission %s %s: %w", input.HTTPMethod, input.PathTemplate, err)
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

// List 返回一页权限记录，以及是否存在下一页。
func (s *PermissionStore) List(ctx context.Context, input permissionapplication.ListPermissionsInput) ([]permissiondomain.Permission, bool, error) {
	predicates := buildListPredicates(input)
	if input.AfterPermissionID != nil {
		predicates = append(predicates, entpermission.PermissionIDGT(*input.AfterPermissionID))
	}
	permissions, err := s.client.Permission.Query().
		Where(predicates...).
		Order(entpermission.ByPermissionID()).
		Limit(input.Limit + 1).
		All(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("list permissions: %w", err)
	}
	hasNext := len(permissions) > input.Limit
	if hasNext {
		permissions = permissions[:input.Limit]
	}
	return toModels(permissions), hasNext, nil
}

// ListAll 返回全部权限目录记录，用于只读 route diff。
func (s *PermissionStore) ListAll(ctx context.Context) ([]permissiondomain.Permission, error) {
	permissions, err := s.client.Permission.Query().Order(entpermission.ByHTTPMethod(), entpermission.ByPathTemplate()).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("list all permissions: %w", err)
	}
	return toModels(permissions), nil
}

// ListEffectiveByUserID 返回用户经由现有角色绑定获得的启用权限。
func (s *PermissionStore) ListEffectiveByUserID(ctx context.Context, userID uuid.UUID) ([]permissiondomain.Permission, error) {
	permissions, err := s.client.Permission.Query().
		Where(
			entpermission.ActiveEQ(true),
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

// Update 更新权限记录，并将唯一约束冲突映射为 ErrPermissionAlreadyExists。
func (s *PermissionStore) Update(ctx context.Context, input permissionapplication.UpdatePermissionInput) (*permissiondomain.Permission, error) {
	updated, err := s.client.Permission.Update().
		Where(entpermission.PermissionIDEQ(input.PermissionID)).
		SetName(input.Name).
		SetDescription(input.Description).
		SetModule(input.Module).
		SetHTTPMethod(input.HTTPMethod).
		SetPathTemplate(input.PathTemplate).
		SetActive(input.Active).
		Save(ctx)
	if err == nil && updated == 0 {
		return nil, permissiondomain.ErrPermissionNotFound
	}
	if err != nil {
		if ent.IsConstraintError(err) {
			return nil, permissiondomain.ErrPermissionAlreadyExists
		}
		return nil, fmt.Errorf("update permission %s: %w", input.PermissionID.String(), err)
	}
	return s.GetByPermissionID(ctx, input.PermissionID)
}

// SetActive 启用或停用权限记录。
func (s *PermissionStore) SetActive(ctx context.Context, permissionID uuid.UUID, active bool) (*permissiondomain.Permission, error) {
	updated, err := s.client.Permission.Update().Where(entpermission.PermissionIDEQ(permissionID)).SetActive(active).Save(ctx)
	if err == nil && updated == 0 {
		return nil, permissiondomain.ErrPermissionNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("set permission active %s: %w", permissionID.String(), err)
	}
	return s.GetByPermissionID(ctx, permissionID)
}

func toModel(entPermission *ent.Permission) *permissiondomain.Permission {
	if entPermission == nil {
		return nil
	}
	return &permissiondomain.Permission{ID: entPermission.ID, PermissionID: entPermission.PermissionID, Name: entPermission.Name, Description: entPermission.Description, Module: entPermission.Module, HTTPMethod: entPermission.HTTPMethod, PathTemplate: entPermission.PathTemplate, Active: entPermission.Active, IsSystem: entPermission.IsSystem, CreatedAt: entPermission.CreatedAt, UpdatedAt: entPermission.UpdatedAt}
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
