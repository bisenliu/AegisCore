package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"go.uber.org/fx"

	"github.com/aegiscore/user-service/ent"
	entrole "github.com/aegiscore/user-service/ent/role"
	roleapplication "github.com/aegiscore/user-service/internal/features/role/application"
	roledomain "github.com/aegiscore/user-service/internal/features/role/domain"
)

type RoleStore struct {
	client *ent.Client
}

var _ roleapplication.RoleStore = (*RoleStore)(nil)
var _ roleapplication.SeedRoleStore = (*RoleStore)(nil)

// RoleStoreParams 包含 PostgreSQL-backed 角色 store 所需的 Fx 输入。
type RoleStoreParams struct {
	fx.In

	Client *ent.Client `name:"primary_db"`
}

// NewRoleStore 构造基于 Ent 的角色 store。
func NewRoleStore(params RoleStoreParams) *RoleStore {
	return &RoleStore{client: params.Client}
}

// Create 插入角色记录，并将唯一约束冲突映射为 ErrRoleAlreadyExists。
func (s *RoleStore) Create(ctx context.Context, input roleapplication.CreateRoleInput) (*roledomain.Role, error) {
	created, err := s.client.Role.Create().
		SetRoleID(input.RoleID).
		SetName(input.Name).
		SetDescription(input.Description).
		SetActive(input.Active).
		SetIsSystem(false).
		Save(ctx)
	if err == nil {
		return toRoleModel(created), nil
	}
	if ent.IsConstraintError(err) {
		return nil, roledomain.ErrRoleAlreadyExists
	}
	return nil, fmt.Errorf("create role %s: %w", input.RoleID.String(), err)
}

// GetByRoleID 按外部 UUID 返回角色记录。
func (s *RoleStore) GetByRoleID(ctx context.Context, roleID uuid.UUID) (*roledomain.Role, error) {
	found, err := s.client.Role.Query().Where(entrole.RoleIDEQ(roleID)).Only(ctx)
	if err == nil {
		return toRoleModel(found), nil
	}
	if ent.IsNotFound(err) {
		return nil, roledomain.ErrRoleNotFound
	}
	return nil, fmt.Errorf("query role by role_id %s: %w", roleID.String(), err)
}

// GetByRoleIDs 按外部 UUID 集合一次性返回角色记录。
func (s *RoleStore) GetByRoleIDs(ctx context.Context, roleIDs []uuid.UUID) ([]roledomain.Role, error) {
	if len(roleIDs) == 0 {
		return []roledomain.Role{}, nil
	}
	roles, err := s.client.Role.Query().Where(entrole.RoleIDIn(roleIDs...)).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("query roles by role_ids: %w", err)
	}
	if len(roles) != len(roleIDs) {
		return nil, roledomain.ErrRoleNotFound
	}
	return toRoleModels(roles), nil
}

// List 返回一页角色记录，以及是否存在下一页。
func (s *RoleStore) List(ctx context.Context, input roleapplication.ListRolesInput) ([]roledomain.Role, bool, error) {
	predicates := buildListPredicates(input)
	if input.AfterRoleID != nil {
		predicates = append(predicates, entrole.RoleIDGT(*input.AfterRoleID))
	}
	roles, err := s.client.Role.Query().
		Where(predicates...).
		Order(entrole.ByRoleID()).
		Limit(input.Limit + 1).
		All(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("list roles: %w", err)
	}
	hasNext := len(roles) > input.Limit
	if hasNext {
		roles = roles[:input.Limit]
	}
	return toRoleModels(roles), hasNext, nil
}

// Update 更新角色记录，并将唯一约束冲突映射为 ErrRoleAlreadyExists。
func (s *RoleStore) Update(ctx context.Context, input roleapplication.UpdateRoleInput) error {
	updated, err := s.client.Role.Update().
		Where(entrole.RoleIDEQ(input.RoleID)).
		SetName(input.Name).
		SetDescription(input.Description).
		SetActive(input.Active).
		Save(ctx)
	if err == nil && updated == 0 {
		return roledomain.ErrRoleNotFound
	}
	if err != nil {
		if ent.IsConstraintError(err) {
			return roledomain.ErrRoleAlreadyExists
		}
		return fmt.Errorf("update role %s: %w", input.RoleID.String(), err)
	}
	return nil
}

// SetActive 启用或停用角色记录。
func (s *RoleStore) SetActive(ctx context.Context, roleID uuid.UUID, active bool) error {
	updated, err := s.client.Role.Update().Where(entrole.RoleIDEQ(roleID)).SetActive(active).Save(ctx)
	if err == nil && updated == 0 {
		return roledomain.ErrRoleNotFound
	}
	if err != nil {
		return fmt.Errorf("set role active %s: %w", roleID.String(), err)
	}
	return nil
}

// UpsertSystemRole 按 role_id 幂等写入系统角色 seed 数据。
func (s *RoleStore) UpsertSystemRole(ctx context.Context, input roleapplication.SeedRoleInput) (*roledomain.Role, bool, error) {
	existing, err := s.client.Role.Query().Where(entrole.RoleIDEQ(input.RoleID)).Only(ctx)
	if err != nil && !ent.IsNotFound(err) {
		return nil, false, fmt.Errorf("query seed role %s: %w", input.RoleID.String(), err)
	}
	if ent.IsNotFound(err) {
		created, err := s.client.Role.Create().
			SetRoleID(input.RoleID).
			SetName(input.Name).
			SetDescription(input.Description).
			SetActive(input.Active).
			SetIsSystem(true).
			Save(ctx)
		if err != nil {
			if ent.IsConstraintError(err) {
				return s.UpsertSystemRole(ctx, input)
			}
			return nil, false, fmt.Errorf("create seed role %s: %w", input.RoleID.String(), err)
		}
		return toRoleModel(created), true, nil
	}

	active := existing.Active
	if input.ReactivateSystem {
		active = input.Active
	}
	updated, err := s.client.Role.UpdateOneID(existing.ID).
		SetName(input.Name).
		SetDescription(input.Description).
		SetActive(active).
		SetIsSystem(true).
		Save(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("update seed role %s: %w", input.RoleID.String(), err)
	}
	return toRoleModel(updated), false, nil
}

func toRoleModel(entRole *ent.Role) *roledomain.Role {
	if entRole == nil {
		return nil
	}
	return &roledomain.Role{ID: entRole.ID, RoleID: entRole.RoleID, Name: entRole.Name, Description: entRole.Description, Active: entRole.Active, IsSystem: entRole.IsSystem, CreatedAt: entRole.CreatedAt, UpdatedAt: entRole.UpdatedAt}
}

func toRoleModels(roles []*ent.Role) []roledomain.Role {
	result := make([]roledomain.Role, 0, len(roles))
	for _, entRole := range roles {
		if mapped := toRoleModel(entRole); mapped != nil {
			result = append(result, *mapped)
		}
	}
	return result
}
