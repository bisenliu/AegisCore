package casbin

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"go.uber.org/fx"

	"github.com/aegiscore/common/runtime/config"
	"github.com/aegiscore/common/runtime/localcache"
	"github.com/aegiscore/user-service/ent"
)

const rbacUserRolesCacheName = "rbac_user_roles"

// UserRoleResolverParams 包含用户角色 resolver 的 Fx 输入。
type UserRoleResolverParams struct {
	fx.In

	Lifecycle fx.Lifecycle
	Config    *config.Config
	Client    *ent.Client `name:"user_db"`
}

// UserRoleResolverResult 暴露 resolver 和对应的缓存统计源。
type UserRoleResolverResult struct {
	fx.Out

	Resolver UserRoleResolver
	Stats    localcache.StatsSource `name:"rbac_user_roles_cache"`
}

// NewUserRoleResolver 构造按用户 bounded TTL 缓存的角色解析器。
func NewUserRoleResolver(params UserRoleResolverParams) (UserRoleResolverResult, error) {
	cfg, ok := params.Config.LocalCache.Instance(rbacUserRolesCacheName)
	if !ok {
		return UserRoleResolverResult{}, fmt.Errorf("local_cache.%s is required", rbacUserRolesCacheName)
	}
	resolver := &entUserRoleResolver{client: params.Client}
	cache, err := localcache.New[uuid.UUID, []uuid.UUID](localcache.Config[uuid.UUID]{
		Name:        rbacUserRolesCacheName,
		Capacity:    cfg.Capacity,
		TTL:         cfg.TTL,
		LoadTimeout: cfg.LoadTimeout,
		KeyString:   func(userID uuid.UUID) string { return userID.String() },
		NumCounters: cfg.NumCounters,
		BufferItems: cfg.BufferItems,
	}, func(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error) {
		return resolver.loadRolesForUser(ctx, userID)
	}, cloneRoleIDs)
	if err != nil {
		return UserRoleResolverResult{}, fmt.Errorf("create rbac user roles localcache: %w", err)
	}
	resolver.cache = cache

	params.Lifecycle.Append(fx.Hook{OnStop: func(context.Context) error {
		cache.Close()
		return nil
	}})
	return UserRoleResolverResult{Resolver: resolver, Stats: cache}, nil
}
