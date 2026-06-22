package casbin

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/aegiscore/common/runtime/localcache"
	"github.com/aegiscore/user-service/ent"
	entrole "github.com/aegiscore/user-service/ent/role"
	entuser "github.com/aegiscore/user-service/ent/user"
	entuserrole "github.com/aegiscore/user-service/ent/userrole"
)

// UserRoleResolver 定义授权热路径按需解析用户启用角色的端口。
type UserRoleResolver interface {
	RolesForUser(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error)
	InvalidateUserRole(userID uuid.UUID)
	InvalidateAllUserRoles()
}

type entUserRoleResolver struct {
	cache *localcache.Cache[uuid.UUID, []uuid.UUID]
}

// RolesForUser 返回用户当前绑定的启用角色 ID。
func (r *entUserRoleResolver) RolesForUser(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error) {
	roleIDs, err := r.cache.GetOrLoad(ctx, userID)
	if err != nil {
		return nil, err
	}
	return roleIDs, nil
}

// InvalidateUserRole 删除单个用户的本地角色缓存。
func (r *entUserRoleResolver) InvalidateUserRole(userID uuid.UUID) {
	_ = r.cache.Delete(userID)
}

// InvalidateAllUserRoles 清空本实例全部用户角色缓存。
func (r *entUserRoleResolver) InvalidateAllUserRoles() {
	_ = r.cache.Clear()
}

func loadRolesForUser(ctx context.Context, client *ent.Client, userID uuid.UUID) ([]uuid.UUID, error) {
	var rows []struct {
		RoleID uuid.UUID `json:"role_id,omitempty"`
	}
	err := client.Role.Query().
		Where(
			entrole.ActiveEQ(true),
			entrole.HasUserRolesWith(
				entuserrole.HasUserWith(entuser.UserIDEQ(userID), entuser.DeletedAtIsNil()),
			),
		).
		Order(entrole.ByRoleID()).
		Select(entrole.FieldRoleID).
		Scan(ctx, &rows)
	if err != nil {
		return nil, fmt.Errorf("load user role policy subjects for user %s: %w", userID.String(), err)
	}
	roleIDs := make([]uuid.UUID, 0, len(rows))
	for _, row := range rows {
		roleIDs = append(roleIDs, row.RoleID)
	}
	return roleIDs, nil
}

func cloneRoleIDs(roleIDs []uuid.UUID) []uuid.UUID {
	if len(roleIDs) == 0 {
		return []uuid.UUID{}
	}
	cloned := make([]uuid.UUID, len(roleIDs))
	copy(cloned, roleIDs)
	return cloned
}
