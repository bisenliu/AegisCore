package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	roleapplication "github.com/aegiscore/user-service/internal/features/role/application"
	roledomain "github.com/aegiscore/user-service/internal/features/role/domain"
	"github.com/aegiscore/user-service/internal/persistence/ent"
	entrole "github.com/aegiscore/user-service/internal/persistence/ent/role"
	entuser "github.com/aegiscore/user-service/internal/persistence/ent/user"
	entuserrole "github.com/aegiscore/user-service/internal/persistence/ent/userrole"
	"github.com/aegiscore/user-service/internal/shared/identity"
)

// UserRoleStore 使用 Ent 持久化用户角色绑定，并随写操作记录 policy revision。
type UserRoleStore struct {
	client *ent.Client
}

var _ roleapplication.UserRoleStore = (*UserRoleStore)(nil)

// NewUserRoleStore 构造基于 Ent 的用户角色绑定 store。
func NewUserRoleStore(client *ent.Client) *UserRoleStore {
	return &UserRoleStore{client: client}
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
func (s *UserRoleStore) Add(ctx context.Context, userID uuid.UUID, roleID uuid.UUID, change roleapplication.PolicyChange) (roleapplication.RolesWriteResult, error) {
	roles, write, err := transactPolicyChange(ctx, s.client, "add user role", change, func(tx *ent.Tx) ([]*ent.Role, error) {
		user, role, err := s.getLockedUserAndRole(ctx, tx, userID, roleID)
		if err != nil {
			return nil, err
		}
		_, err = tx.UserRole.Create().SetUserID(user.ID).SetRoleID(role.ID).Save(ctx)
		if ent.IsConstraintError(err) {
			return nil, roledomain.ErrUserRoleAlreadyExists
		}
		if err != nil {
			return nil, fmt.Errorf("add user role user %s role %s: %w", userID.String(), roleID.String(), err)
		}
		return s.listRolesByInternalUserID(ctx, tx, user.ID, userID)
	})
	return roleapplication.RolesWriteResult{Items: toRoleModels(roles), Revision: write.Revision}, err
}

// Replace 幂等替换用户的完整角色绑定集合。
func (s *UserRoleStore) Replace(ctx context.Context, userID uuid.UUID, roleIDs []uuid.UUID, change roleapplication.PolicyChange) (roleapplication.RolesWriteResult, error) {
	roles, write, err := transactPolicyChange(ctx, s.client, "replace user roles", change, func(tx *ent.Tx) ([]*ent.Role, error) {
		user, err := s.getLockedUserByExternalID(ctx, tx, userID)
		if err != nil {
			return nil, err
		}
		roles, err := s.lockedRolesByExternalIDs(ctx, tx, roleIDs)
		if err != nil {
			return nil, err
		}
		if _, err := tx.UserRole.Delete().Where(entuserrole.UserIDEQ(user.ID)).Exec(ctx); err != nil {
			return nil, fmt.Errorf("delete user roles for user %s: %w", userID.String(), err)
		}
		builders := make([]*ent.UserRoleCreate, 0, len(roles))
		for _, role := range roles {
			builders = append(builders, tx.UserRole.Create().SetUserID(user.ID).SetRoleID(role.ID))
		}
		if len(builders) > 0 {
			if _, err := tx.UserRole.CreateBulk(builders...).Save(ctx); err != nil {
				return nil, fmt.Errorf("create replacement user roles for user %s: %w", userID.String(), err)
			}
		}
		return roles, nil
	})
	return roleapplication.RolesWriteResult{Items: toRoleModels(roles), Revision: write.Revision}, err
}

// Remove 删除用户角色绑定。
func (s *UserRoleStore) Remove(ctx context.Context, userID uuid.UUID, roleID uuid.UUID, change roleapplication.PolicyChange) (roleapplication.RolesWriteResult, error) {
	roles, write, err := transactPolicyChange(ctx, s.client, "remove user role", change, func(tx *ent.Tx) ([]*ent.Role, error) {
		user, role, err := s.getLockedUserAndRole(ctx, tx, userID, roleID)
		if err != nil {
			return nil, err
		}
		deleted, err := tx.UserRole.Delete().Where(entuserrole.UserIDEQ(user.ID), entuserrole.RoleIDEQ(role.ID)).Exec(ctx)
		if err != nil {
			return nil, fmt.Errorf("remove user role user %s role %s: %w", userID.String(), roleID.String(), err)
		}
		if deleted == 0 {
			return nil, roledomain.ErrUserRoleNotFound
		}
		return s.listRolesByInternalUserID(ctx, tx, user.ID, userID)
	})
	return roleapplication.RolesWriteResult{Items: toRoleModels(roles), Revision: write.Revision}, err
}

func (s *UserRoleStore) listRolesByInternalUserID(ctx context.Context, tx *ent.Tx, internalUserID int64, userID uuid.UUID) ([]*ent.Role, error) {
	roles, err := tx.Role.Query().
		Where(entrole.HasUserRolesWith(entuserrole.UserIDEQ(internalUserID))).
		Order(entrole.ByRoleID()).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("list roles for user %s in mutation transaction: %w", userID.String(), err)
	}
	return roles, nil
}

// AssignRole 幂等新增用户角色绑定，已存在时返回 added=false。
func (s *UserRoleStore) AssignRole(ctx context.Context, userID uuid.UUID, roleID uuid.UUID) (bool, error) {
	user, role, err := s.getUserAndRole(ctx, userID, roleID)
	if err != nil {
		return false, err
	}
	_, err = s.client.UserRole.Create().SetUserID(user.ID).SetRoleID(role.ID).Save(ctx)
	if err == nil {
		return true, nil
	}
	if ent.IsConstraintError(err) {
		return false, nil
	}
	return false, fmt.Errorf("assign user role user %s role %s: %w", userID.String(), roleID.String(), err)
}

func (s *UserRoleStore) getLockedUserAndRole(ctx context.Context, tx *ent.Tx, userID uuid.UUID, roleID uuid.UUID) (*ent.User, *ent.Role, error) {
	user, err := s.getLockedUserByExternalID(ctx, tx, userID)
	if err != nil {
		return nil, nil, err
	}
	role, err := tx.Role.Query().Where(entrole.RoleIDEQ(roleID)).Only(ctx)
	if ent.IsNotFound(err) {
		return nil, nil, roledomain.ErrRoleNotFound
	}
	if err != nil {
		return nil, nil, fmt.Errorf("query role by role_id %s: %w", roleID.String(), err)
	}
	return user, role, nil
}

func (s *UserRoleStore) getLockedUserByExternalID(ctx context.Context, tx *ent.Tx, userID uuid.UUID) (*ent.User, error) {
	user, err := tx.User.Query().Where(entuser.UserIDEQ(userID), entuser.DeletedAtIsNil()).Only(ctx)
	if ent.IsNotFound(err) {
		return nil, identity.ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query user by user_id %s: %w", userID.String(), err)
	}
	return user, nil
}

func (s *UserRoleStore) lockedRolesByExternalIDs(ctx context.Context, tx *ent.Tx, roleIDs []uuid.UUID) ([]*ent.Role, error) {
	if len(roleIDs) == 0 {
		return []*ent.Role{}, nil
	}
	roles, err := tx.Role.Query().Where(entrole.RoleIDIn(roleIDs...)).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("query roles by role_ids: %w", err)
	}
	if len(roles) != len(roleIDs) {
		return nil, roledomain.ErrRoleNotFound
	}
	return roles, nil
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
		return nil, identity.ErrUserNotFound
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
