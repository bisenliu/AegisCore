package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"go.uber.org/fx"

	"github.com/aegiscore/user-service/ent"
	entrole "github.com/aegiscore/user-service/ent/role"
	entuser "github.com/aegiscore/user-service/ent/user"
	entuserrole "github.com/aegiscore/user-service/ent/userrole"
	roleapplication "github.com/aegiscore/user-service/internal/features/role/application"
	roledomain "github.com/aegiscore/user-service/internal/features/role/domain"
	userdomain "github.com/aegiscore/user-service/internal/features/user/domain"
)

type UserRoleStore struct {
	client *ent.Client
}

var _ roleapplication.UserRoleStore = (*UserRoleStore)(nil)
var _ roleapplication.SeedUserRoleStore = (*UserRoleStore)(nil)

// UserRoleStoreParams 包含 PostgreSQL-backed 用户角色绑定 store 所需的 Fx 输入。
type UserRoleStoreParams struct {
	fx.In

	Client *ent.Client `name:"user_db"`
}

// NewUserRoleStore 构造基于 Ent 的用户角色绑定 store。
func NewUserRoleStore(params UserRoleStoreParams) *UserRoleStore {
	return &UserRoleStore{client: params.Client}
}

// ListByUserID 按用户外部 ID 返回已绑定角色列表。
func (s *UserRoleStore) ListByUserID(ctx context.Context, userID uuid.UUID) ([]roledomain.Role, error) {
	user, err := s.getUserByExternalID(ctx, userID)
	if err != nil {
		return nil, err
	}
	roles, err := s.client.Role.Query().
		Where(entrole.HasUserRolesWith(entuserrole.UserIDEQ(user.ID))).
		Order(entrole.ByRoleID()).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("list roles for user %s: %w", userID.String(), err)
	}
	return toRoleModels(roles), nil
}

// Add 新增用户角色绑定。
func (s *UserRoleStore) Add(ctx context.Context, userID uuid.UUID, roleID uuid.UUID) error {
	user, role, err := s.getUserAndRole(ctx, userID, roleID)
	if err != nil {
		return err
	}
	_, err = s.client.UserRole.Create().SetUserID(user.ID).SetRoleID(role.ID).Save(ctx)
	if err == nil {
		return nil
	}
	if ent.IsConstraintError(err) {
		return roledomain.ErrUserRoleAlreadyExists
	}
	return fmt.Errorf("add user role user %s role %s: %w", userID.String(), roleID.String(), err)
}

// Replace 幂等替换用户的完整角色绑定集合。
func (s *UserRoleStore) Replace(ctx context.Context, userID uuid.UUID, roleIDs []uuid.UUID) ([]roledomain.Role, error) {
	user, err := s.getUserByExternalID(ctx, userID)
	if err != nil {
		return nil, err
	}
	roles, err := s.rolesByExternalIDs(ctx, roleIDs)
	if err != nil {
		return nil, err
	}
	tx, err := s.client.Tx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin replace user roles: %w", err)
	}
	if _, err := tx.UserRole.Delete().Where(entuserrole.UserIDEQ(user.ID)).Exec(ctx); err != nil {
		return nil, rollback(tx, fmt.Errorf("delete user roles for user %s: %w", userID.String(), err))
	}
	for _, role := range roles {
		if _, err := tx.UserRole.Create().SetUserID(user.ID).SetRoleID(role.ID).Save(ctx); err != nil {
			return nil, rollback(tx, fmt.Errorf("create replacement user role user %s role %s: %w", userID.String(), role.RoleID.String(), err))
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit replace user roles: %w", err)
	}
	return toRoleModels(roles), nil
}

// Remove 删除用户角色绑定。
func (s *UserRoleStore) Remove(ctx context.Context, userID uuid.UUID, roleID uuid.UUID) error {
	user, role, err := s.getUserAndRole(ctx, userID, roleID)
	if err != nil {
		return err
	}
	deleted, err := s.client.UserRole.Delete().Where(entuserrole.UserIDEQ(user.ID), entuserrole.RoleIDEQ(role.ID)).Exec(ctx)
	if err != nil {
		return fmt.Errorf("remove user role user %s role %s: %w", userID.String(), roleID.String(), err)
	}
	if deleted == 0 {
		return roledomain.ErrUserRoleNotFound
	}
	return nil
}

// AssignRole 幂等新增用户角色绑定，已存在时返回 added=false。
func (s *UserRoleStore) AssignRole(ctx context.Context, userID uuid.UUID, roleID uuid.UUID) (bool, error) {
	err := s.Add(ctx, userID, roleID)
	if err == nil {
		return true, nil
	}
	if err == roledomain.ErrUserRoleAlreadyExists {
		return false, nil
	}
	return false, err
}

func (s *UserRoleStore) getUserAndRole(ctx context.Context, userID uuid.UUID, roleID uuid.UUID) (*ent.User, *ent.Role, error) {
	user, err := s.getUserByExternalID(ctx, userID)
	if err != nil {
		return nil, nil, err
	}
	role, err := s.getRoleByExternalID(ctx, roleID)
	if err != nil {
		return nil, nil, err
	}
	return user, role, nil
}

func (s *UserRoleStore) getUserByExternalID(ctx context.Context, userID uuid.UUID) (*ent.User, error) {
	user, err := s.client.User.Query().Where(entuser.UserIDEQ(userID), entuser.DeletedAtIsNil()).Only(ctx)
	if err == nil {
		return user, nil
	}
	if ent.IsNotFound(err) {
		return nil, userdomain.ErrUserNotFound
	}
	return nil, fmt.Errorf("query user by user_id %s: %w", userID.String(), err)
}

func (s *UserRoleStore) getRoleByExternalID(ctx context.Context, roleID uuid.UUID) (*ent.Role, error) {
	role, err := s.client.Role.Query().Where(entrole.RoleIDEQ(roleID)).Only(ctx)
	if err == nil {
		return role, nil
	}
	if ent.IsNotFound(err) {
		return nil, roledomain.ErrRoleNotFound
	}
	return nil, fmt.Errorf("query role by role_id %s: %w", roleID.String(), err)
}

func (s *UserRoleStore) rolesByExternalIDs(ctx context.Context, roleIDs []uuid.UUID) ([]*ent.Role, error) {
	roles := make([]*ent.Role, 0, len(roleIDs))
	for _, roleID := range roleIDs {
		role, err := s.getRoleByExternalID(ctx, roleID)
		if err != nil {
			return nil, err
		}
		roles = append(roles, role)
	}
	return roles, nil
}
