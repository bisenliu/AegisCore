package casbin

import (
	"context"

	"github.com/google/uuid"

	"github.com/aegiscore/common/runtime/localcache"
	"github.com/aegiscore/user-service/ent"
)

// UserRoleResolver 定义授权热路径按需解析用户启用角色的端口。
type UserRoleResolver interface {
	RolesForUser(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error)
	InvalidateUserRole(userID uuid.UUID)
	InvalidateAllUserRoles()
}

type entUserRoleResolver struct {
	client *ent.Client
	cache  *localcache.Cache[uuid.UUID, []uuid.UUID]
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

func cloneRoleIDs(roleIDs []uuid.UUID) []uuid.UUID {
	if len(roleIDs) == 0 {
		return []uuid.UUID{}
	}
	cloned := make([]uuid.UUID, len(roleIDs))
	copy(cloned, roleIDs)
	return cloned
}
