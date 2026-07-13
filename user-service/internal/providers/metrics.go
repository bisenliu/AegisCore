package providers

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/fx"

	"github.com/aegiscore/common/runtime/config"
	"github.com/aegiscore/common/runtime/localcache"
	commonmetrics "github.com/aegiscore/common/runtime/observability/metrics"
	authredis "github.com/aegiscore/user-service/internal/features/auth/infrastructure/redis"
	permissionredis "github.com/aegiscore/user-service/internal/features/permission/infrastructure/redis"
	"github.com/aegiscore/user-service/internal/resources"
)

const (
	authSessionPurgePoolMetricsName = "auth_session_purge_pool"
	rbacPolicyWatcherMetricsName    = "rbac_policy_watcher"
	redisMetricsMinProbeInterval    = 15 * time.Second
)

// RuntimeDependencyMetricsParams 包含注册运行时依赖指标所需的服务级依赖。
type RuntimeDependencyMetricsParams struct {
	fx.In

	Config           *config.Config
	Metrics          *commonmetrics.Provider
	UserDB           *sql.DB                 `name:"user_db"`
	CacheRedis       *redis.Client           `name:"cache_redis"`
	SessionPurgePool authredis.PurgeTaskPool `name:"auth_session_purge_pool"`
	PolicyWatcher    permissionredis.WatcherStatus
	AuthTokenCache   localcache.StatsSource `name:"auth_token_version_cache"`
	RBACRolesCache   localcache.StatsSource `name:"rbac_user_roles_cache"`
}

// RegisterRuntimeDependencyMetrics 注册用户服务运行时依赖 Prometheus collector。
func RegisterRuntimeDependencyMetrics(params RuntimeDependencyMetricsParams) error {
	if params.Metrics == nil || !params.Metrics.Enabled() {
		return nil
	}

	userDBCollector, err := commonmetrics.NewSQLDBCollector(commonmetrics.SQLDBCollectorOptions{
		Resource: resources.NameUserDB,
		DB:       params.UserDB,
	})
	if err != nil {
		return fmt.Errorf("create postgres metrics collector: %w", err)
	}
	if err := params.Metrics.Register(userDBCollector); err != nil {
		return err
	}

	redisCfg, _ := params.Config.RedisConfig(resources.NameCacheRedis)
	redisCollector, err := commonmetrics.NewRedisPingCollector(commonmetrics.RedisPingCollectorOptions{
		Resource:    resources.NameCacheRedis,
		Pinger:      commonmetrics.NewRedisClientPinger(params.CacheRedis),
		Timeout:     redisCfg.PingTimeout,
		MinInterval: redisMetricsMinProbeInterval,
	})
	if err != nil {
		return fmt.Errorf("create redis metrics collector: %w", err)
	}
	if err := params.Metrics.Register(redisCollector); err != nil {
		return err
	}

	workerpoolCollector, err := commonmetrics.NewWorkerpoolCollector(commonmetrics.WorkerpoolCollectorOptions{
		Pool:   authSessionPurgePoolMetricsName,
		Source: params.SessionPurgePool,
	})
	if err != nil {
		return fmt.Errorf("create workerpool metrics collector: %w", err)
	}
	if err := params.Metrics.Register(workerpoolCollector); err != nil {
		return err
	}

	authTokenCacheCollector, err := commonmetrics.NewLocalcacheCollector(commonmetrics.LocalcacheCollectorOptions{
		Source: params.AuthTokenCache,
	})
	if err != nil {
		return fmt.Errorf("create auth token version cache metrics collector: %w", err)
	}
	if err := params.Metrics.Register(authTokenCacheCollector); err != nil {
		return err
	}

	rbacRolesCacheCollector, err := commonmetrics.NewLocalcacheCollector(commonmetrics.LocalcacheCollectorOptions{
		Source: params.RBACRolesCache,
	})
	if err != nil {
		return fmt.Errorf("create rbac user roles cache metrics collector: %w", err)
	}
	if err := params.Metrics.Register(rbacRolesCacheCollector); err != nil {
		return err
	}

	watcherCollector, err := commonmetrics.NewComponentStatusCollector(commonmetrics.ComponentStatusCollectorOptions{
		Resource: rbacPolicyWatcherMetricsName,
		Source:   params.PolicyWatcher,
	})
	if err != nil {
		return fmt.Errorf("create rbac watcher metrics collector: %w", err)
	}
	return params.Metrics.Register(watcherCollector)
}
