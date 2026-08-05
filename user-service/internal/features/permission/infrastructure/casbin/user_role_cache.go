package casbin

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/aegiscore/common/runtime/localcache"
	serviceconfig "github.com/aegiscore/user-service/internal/config"
	"github.com/aegiscore/user-service/internal/persistence/ent"
)

const rbacUserRolesCacheName = "rbac_user_roles"

// UserRoleResolverParams 包含用户角色 resolver 的 Fx 输入。
type UserRoleResolverParams struct {
	Settings serviceconfig.RBACSettings
	Client   *ent.Client
}

// UserRoleResolverResult 暴露 resolver 和对应的缓存统计源。
type UserRoleResolverResult struct {
	Resolver UserRoleResolver
	Stats    localcache.StatsSource `name:"rbac_user_roles_cache"`
}

// NewUserRoleResolver 构造按用户 bounded TTL 缓存的角色解析器。
func NewUserRoleResolver(params UserRoleResolverParams) (UserRoleResolverResult, error) {
	cfg := params.Settings.UserRoleCache
	resolver := &entUserRoleResolver{client: params.Client}
	if !cfg.Enabled {
		return UserRoleResolverResult{Resolver: resolver, Stats: resolver}, nil
	}
	cache, err := localcache.NewLoadingCache(cfg.Localcache(rbacUserRolesCacheName), func(ctx context.Context, key string) ([]uuid.UUID, error) {
		userID, err := uuid.Parse(key)
		if err != nil {
			return nil, fmt.Errorf("parse rbac user role cache user id: %w", err)
		}
		return resolver.loadCacheableRolesForUser(ctx, userID)
	})
	if err != nil {
		return UserRoleResolverResult{}, fmt.Errorf("create rbac user roles localcache: %w", err)
	}
	resolver.cache = cache
	return UserRoleResolverResult{Resolver: resolver, Stats: cache}, nil
}
