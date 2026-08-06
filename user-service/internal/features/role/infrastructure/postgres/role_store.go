package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	roleapplication "github.com/aegiscore/user-service/internal/features/role/application"
	roledomain "github.com/aegiscore/user-service/internal/features/role/domain"
	"github.com/aegiscore/user-service/internal/persistence/ent"
	entrole "github.com/aegiscore/user-service/internal/persistence/ent/role"
)

// RoleStore 使用 Ent 持久化普通角色写操作和系统角色 seed。
type RoleStore struct {
	client *ent.Client
}

var _ roleapplication.RoleStore = (*RoleStore)(nil)
var _ roleapplication.SeedRoleStore = (*RoleStore)(nil)

// NewRoleStore 构造基于 Ent 的角色 store。
func NewRoleStore(client *ent.Client) *RoleStore {
	return &RoleStore{client: client}
}

// Create 插入角色记录，并将唯一约束冲突映射为 ErrRoleAlreadyExists。
func (s *RoleStore) Create(ctx context.Context, input roleapplication.CreateRoleInput, change roleapplication.PolicyChange) (*roleapplication.RoleWriteResult, error) {
	created, write, err := transactPolicyChange(ctx, s.client, "create role", change, func(tx *ent.Tx) (*ent.Role, error) {
		created, err := tx.Role.Create().
			SetRoleID(input.RoleID).
			SetName(input.Name).
			SetDescription(input.Description).
			SetActive(input.Active).
			SetIsSystem(false).
			Save(ctx)
		if ent.IsConstraintError(err) {
			return nil, roledomain.ErrRoleAlreadyExists
		}
		if err != nil {
			return nil, fmt.Errorf("create role %s: %w", input.RoleID.String(), err)
		}
		return created, nil
	})
	if err != nil {
		return nil, err
	}
	return &roleapplication.RoleWriteResult{Role: *toRoleModel(created), Revision: write.Revision}, nil
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
func (s *RoleStore) Update(ctx context.Context, input roleapplication.UpdateRoleInput, change roleapplication.PolicyChange) (roleapplication.PolicyWriteResult, error) {
	_, write, err := transactPolicyChange(ctx, s.client, "update role", change, func(tx *ent.Tx) (struct{}, error) {
		role, err := lockOrdinaryRole(ctx, tx, input.RoleID)
		if err != nil {
			return struct{}{}, err
		}
		_, err = tx.Role.UpdateOneID(role.ID).
			Where(entrole.IsSystemEQ(false)).
			SetName(input.Name).
			SetDescription(input.Description).
			SetActive(input.Active).
			Save(ctx)
		if ent.IsConstraintError(err) {
			return struct{}{}, roledomain.ErrRoleAlreadyExists
		}
		if ent.IsNotFound(err) {
			return struct{}{}, roledomain.ErrSystemRoleProtected
		}
		if err != nil {
			return struct{}{}, fmt.Errorf("update role %s: %w", input.RoleID.String(), err)
		}
		return struct{}{}, nil
	})
	return write, err
}

// SetActive 启用或停用角色记录。
func (s *RoleStore) SetActive(ctx context.Context, roleID uuid.UUID, active bool, change roleapplication.PolicyChange) (roleapplication.PolicyWriteResult, error) {
	_, write, err := transactPolicyChange(ctx, s.client, "set role active", change, func(tx *ent.Tx) (struct{}, error) {
		role, err := lockOrdinaryRole(ctx, tx, roleID)
		if err != nil {
			return struct{}{}, err
		}
		_, err = tx.Role.UpdateOneID(role.ID).
			Where(entrole.IsSystemEQ(false)).
			SetActive(active).
			Save(ctx)
		if ent.IsNotFound(err) {
			return struct{}{}, roledomain.ErrSystemRoleProtected
		}
		if err != nil {
			return struct{}{}, fmt.Errorf("set role active %s: %w", roleID.String(), err)
		}
		return struct{}{}, nil
	})
	return write, err
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
				// 并发 seed 可能已插入同一 role_id；重新读取并走更新分支即可收敛到基线。
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
