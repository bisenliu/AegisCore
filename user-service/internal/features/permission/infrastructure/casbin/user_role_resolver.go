package casbin

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"

	"github.com/google/uuid"

	"github.com/aegiscore/common/runtime/localcache"
	"github.com/aegiscore/user-service/internal/persistence/ent"
)

var errUserRoleCacheGenerationStale = errors.New("rbac user role cache generation is stale")

// UserRoleResolver 定义授权热路径按需解析用户启用角色的端口。
type UserRoleResolver interface {
	RolesForUser(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error)
	InvalidateUserRole(userID uuid.UUID)
	InvalidateAllUserRoles()
}

type entUserRoleResolver struct {
	client          *ent.Client
	cache           *localcache.LoadingCache[uuid.UUID, []uuid.UUID]
	generation      userRoleCacheGeneration
	closeOnce       sync.Once
	directLoads     atomic.Uint64
	directLoadError atomic.Uint64
}

type userRoleCacheGeneration struct {
	mu     sync.RWMutex
	global uint64
	users  map[uuid.UUID]uint64
}

type userRoleCacheGenerationToken struct {
	global uint64
	user   uint64
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
	roleIDs, err := r.cache.GetOrLoad(ctx, userID)
	if err != nil {
		return nil, err
	}
	return cloneRoleIDs(roleIDs), nil
}

// InvalidateUserRole 提升单个用户 generation 后删除本地角色缓存。
func (r *entUserRoleResolver) InvalidateUserRole(userID uuid.UUID) {
	if r.cache == nil {
		return
	}
	r.generation.invalidateUser(userID)
	_ = r.cache.Delete(userID)
}

// InvalidateAllUserRoles 提升全量 generation 后清空本实例全部用户角色缓存。
func (r *entUserRoleResolver) InvalidateAllUserRoles() {
	if r.cache == nil {
		return
	}
	r.generation.invalidateAll()
	_ = r.cache.Clear()
}

// Close 释放 resolver 持有的本地缓存资源；共享 Ent client 由调用方拥有。
func (r *entUserRoleResolver) Close() error {
	if r == nil {
		return nil
	}
	r.closeOnce.Do(func() {
		if r.cache != nil {
			r.cache.Close()
		}
	})
	return nil
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
	token := r.generation.snapshot(userID)
	roleIDs, err := r.loadRolesForUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	if !r.generation.valid(userID, token) {
		return nil, errUserRoleCacheGenerationStale
	}
	return cloneRoleIDs(roleIDs), nil
}

func (g *userRoleCacheGeneration) snapshot(userID uuid.UUID) userRoleCacheGenerationToken {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return userRoleCacheGenerationToken{global: g.global, user: g.users[userID]}
}

func (g *userRoleCacheGeneration) valid(userID uuid.UUID, token userRoleCacheGenerationToken) bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.global == token.global && g.users[userID] == token.user
}

func (g *userRoleCacheGeneration) invalidateUser(userID uuid.UUID) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.users == nil {
		g.users = make(map[uuid.UUID]uint64)
	}
	g.users[userID]++
}

func (g *userRoleCacheGeneration) invalidateAll() {
	g.mu.Lock()
	g.global++
	g.mu.Unlock()
}

func cloneRoleIDs(roleIDs []uuid.UUID) []uuid.UUID {
	if len(roleIDs) == 0 {
		return []uuid.UUID{}
	}
	cloned := make([]uuid.UUID, len(roleIDs))
	copy(cloned, roleIDs)
	return cloned
}
