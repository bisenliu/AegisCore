package casbin

import (
	"context"
	"sync/atomic"

	"github.com/google/uuid"

	"github.com/aegiscore/common/runtime/localcache"
	"github.com/aegiscore/user-service/internal/persistence/ent"
)

// UserRoleResolver 定义授权热路径按需解析用户启用角色的端口。
type UserRoleResolver interface {
	RolesForUser(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error)
	InvalidateUserRole(userID uuid.UUID)
	InvalidateAllUserRoles()
}

type entUserRoleResolver struct {
	client          *ent.Client
	cache           *localcache.LoadingCache[[]uuid.UUID]
	directLoads     atomic.Uint64
	directLoadError atomic.Uint64
}

// RolesForUser 返回用户当前绑定的启用角色 ID。
func (r *entUserRoleResolver) RolesForUser(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error) {
	if r.cache == nil {
		r.directLoads.Add(1)
		roleIDs, err := r.loadRolesForUser(ctx, userID)
		if err != nil {
			r.directLoadError.Add(1)
		}
		return cloneRoleIDs(roleIDs), err
	}
	roleIDs, err := r.cache.Get(ctx, userID.String())
	if err != nil {
		return nil, err
	}
	return cloneRoleIDs(roleIDs), nil
}

// InvalidateUserRole 失效单个用户的本地角色缓存。
func (r *entUserRoleResolver) InvalidateUserRole(userID uuid.UUID) {
	if r.cache == nil {
		return
	}
	r.cache.Invalidate(userID.String())
}

// InvalidateAllUserRoles 清空本实例全部用户角色缓存。
func (r *entUserRoleResolver) InvalidateAllUserRoles() {
	if r.cache == nil {
		return
	}
	r.cache.InvalidateAll()
}

// Name 返回供 metrics 使用的稳定缓存实例名。
func (r *entUserRoleResolver) Name() string {
	return rbacUserRolesCacheName
}

// Stats 返回关闭本地缓存时的逐次回源统计。
func (r *entUserRoleResolver) Stats() localcache.Stats {
	loads := r.directLoads.Load()
	errors := r.directLoadError.Load()
	return localcache.Stats{Miss: loads, LoadSuccess: loads - errors, LoadError: errors}
}

func (r *entUserRoleResolver) loadCacheableRolesForUser(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error) {
	roleIDs, err := r.loadRolesForUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	return cloneRoleIDs(roleIDs), nil
}

func cloneRoleIDs(roleIDs []uuid.UUID) []uuid.UUID {
	if len(roleIDs) == 0 {
		return []uuid.UUID{}
	}
	cloned := make([]uuid.UUID, len(roleIDs))
	copy(cloned, roleIDs)
	return cloned
}
