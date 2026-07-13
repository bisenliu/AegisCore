package providers

import (
	"context"
	"database/sql"

	"github.com/redis/go-redis/v9"
	"go.uber.org/fx"

	permissioncasbin "github.com/aegiscore/user-service/internal/features/permission/infrastructure/casbin"
	permissionredis "github.com/aegiscore/user-service/internal/features/permission/infrastructure/redis"
	"github.com/aegiscore/user-service/internal/resources"
	"github.com/aegiscore/user-service/internal/router"
)

// HealthCheckParams 包含服务健康探针检查项所需依赖。
type HealthCheckParams struct {
	fx.In

	UserDB        *sql.DB       `name:"user_db"`
	CacheRedis    *redis.Client `name:"cache_redis"`
	CasbinPolicy  *permissioncasbin.Engine
	PolicyWatcher permissionredis.WatcherStatus
}

type postgresHealthChecker struct {
	name string
	db   *sql.DB
}

type redisHealthChecker struct {
	name   string
	client *redis.Client
}

type casbinPolicyHealthChecker struct {
	engine interface {
		LastError() error
	}
}

type watcherHealthChecker struct {
	watcher permissionredis.WatcherStatus
}

// ProvideHealthChecks 构造用户服务启动和流量接入探针检查项。
// readiness 与 startup 当前使用同一组关键依赖；Casbin policy 和 watcher 不可用时拒绝接入流量，避免授权未就绪时服务放行或误拒。
func ProvideHealthChecks(params HealthCheckParams) router.HealthChecks {
	checks := []router.HealthChecker{
		postgresHealthChecker{name: "postgres." + resources.NameUserDB, db: params.UserDB},
		redisHealthChecker{name: "redis." + resources.NameCacheRedis, client: params.CacheRedis},
		casbinPolicyHealthChecker{engine: params.CasbinPolicy},
		watcherHealthChecker{watcher: params.PolicyWatcher},
	}
	return router.HealthChecks{Readiness: checks, Startup: checks}
}

func (c postgresHealthChecker) Name() string {
	return c.name
}

func (c postgresHealthChecker) Check(ctx context.Context) router.HealthCheckResult {
	if c.db == nil {
		return unavailableHealthResult(c.name, "postgres unavailable")
	}
	if err := c.db.PingContext(ctx); err != nil {
		return unavailableHealthResult(c.name, "postgres unavailable")
	}
	return okHealthResult(c.name)
}

func (c redisHealthChecker) Name() string {
	return c.name
}

func (c redisHealthChecker) Check(ctx context.Context) router.HealthCheckResult {
	if c.client == nil {
		return unavailableHealthResult(c.name, "redis unavailable")
	}
	if err := c.client.Ping(ctx).Err(); err != nil {
		return unavailableHealthResult(c.name, "redis unavailable")
	}
	return okHealthResult(c.name)
}

func (c casbinPolicyHealthChecker) Name() string {
	return "rbac.casbin_policy"
}

func (c casbinPolicyHealthChecker) Check(context.Context) router.HealthCheckResult {
	name := c.Name()
	if c.engine == nil {
		return unavailableHealthResult(name, "casbin policy unavailable")
	}
	if err := c.engine.LastError(); err != nil {
		return unavailableHealthResult(name, "casbin policy unavailable")
	}
	return okHealthResult(name)
}

func (c watcherHealthChecker) Name() string {
	return "rbac.policy_watcher"
}

func (c watcherHealthChecker) Check(context.Context) router.HealthCheckResult {
	name := c.Name()
	if c.watcher == nil {
		return unavailableHealthResult(name, "rbac policy watcher unavailable")
	}
	if !c.watcher.Running() {
		return unavailableHealthResult(name, "rbac policy watcher stopped")
	}
	if err := c.watcher.LastError(); err != nil {
		return unavailableHealthResult(name, "rbac policy watcher error")
	}
	return okHealthResult(name)
}

func okHealthResult(name string) router.HealthCheckResult {
	return router.HealthCheckResult{Name: name, Status: router.HealthCheckStatusOK}
}

func unavailableHealthResult(name string, message string) router.HealthCheckResult {
	if message == "" {
		message = "dependency unavailable"
	}
	return router.HealthCheckResult{Name: name, Status: router.HealthCheckStatusUnavailable, Message: message}
}
