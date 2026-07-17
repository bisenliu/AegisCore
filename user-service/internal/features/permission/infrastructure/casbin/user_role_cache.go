package casbin

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"go.uber.org/fx"

	"github.com/aegiscore/common/runtime/localcache"
	"github.com/aegiscore/user-service/ent"
	serviceconfig "github.com/aegiscore/user-service/internal/config"
)

const rbacUserRolesCacheName = "rbac_user_roles"

// UserRoleResolverParams 包含用户角色 resolver 的 Fx 输入。
type UserRoleResolverParams struct {
	fx.In

	Lifecycle fx.Lifecycle
	Config    *serviceconfig.Config
	Client    *ent.Client `name:"primary_db"`
}

// UserRoleResolverResult 暴露 resolver 和对应的缓存统计源。
type UserRoleResolverResult struct {
	fx.Out

	Resolver UserRoleResolver
	Stats    localcache.StatsSource `name:"rbac_user_roles_cache"`
}

// NewUserRoleResolver 构造按用户 bounded TTL 缓存的角色解析器。
func NewUserRoleResolver(params UserRoleResolverParams) (UserRoleResolverResult, error) {
	cfg := params.Config.RBAC.UserRoleCache
	resolver := &entUserRoleResolver{client: params.Client}
	if !cfg.IsEnabled() {
		return UserRoleResolverResult{Resolver: resolver, Stats: resolver}, nil
	}
	cache, err := localcache.New[uuid.UUID, []uuid.UUID](localcache.Config[uuid.UUID]{
		Name:        rbacUserRolesCacheName,
		Capacity:    cfg.SizeValue(),
		TTL:         cfg.TTLValue(),
		LoadTimeout: cfg.LoadTimeoutValue(),
		KeyString:   func(userID uuid.UUID) string { return userID.String() },
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
