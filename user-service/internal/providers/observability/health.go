package observability

import (
	"context"
	"database/sql"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/fx"

	commonresources "github.com/aegiscore/common/runtime/resources"
	serviceconfig "github.com/aegiscore/user-service/internal/config"
	permissionapplication "github.com/aegiscore/user-service/internal/features/permission/application"
	permissionauthorization "github.com/aegiscore/user-service/internal/features/permission/application/authorization"
	"github.com/aegiscore/user-service/internal/resources"
	"github.com/aegiscore/user-service/internal/router"
)

// HealthCheckParams 包含服务健康探针检查项所需依赖。
type HealthCheckParams struct {
	fx.In

	Resources     serviceconfig.ResourceSettings
	RBAC          serviceconfig.RBACSettings
	PrimaryDB     *sql.DB               `name:"primary_db"`
	CacheRedis    redis.UniversalClient `name:"cache_redis"`
	CasbinPolicy  permissionauthorization.PolicyHealth
	PolicyWatcher permissionapplication.PolicyWatcherStatus
}

type postgresHealthChecker struct {
	name    string
	db      *sql.DB
	timeout time.Duration
}

type redisHealthChecker struct {
	name    string
	client  redis.UniversalClient
	timeout time.Duration
}

type casbinPolicyHealthChecker struct {
	engine permissionauthorization.PolicyHealth
}

type watcherHealthChecker struct {
	watcher      permissionapplication.PolicyWatcherStatus
	maxStaleness time.Duration
	now          func() time.Time
}

// ProvideHealthChecks 构造用户服务启动和流量接入探针检查项。
// readiness 与 startup 当前使用同一组关键依赖；Casbin policy 和 watcher 不可用时拒绝接入流量，避免授权未就绪时服务放行或误拒。
func ProvideHealthChecks(params HealthCheckParams) router.HealthChecks {
	redisCfg := params.Resources.Redis[resources.NameCacheRedis]
	redisCfg.ApplyDefaults()
	checks := []router.HealthChecker{
		postgresHealthChecker{name: "postgres." + resources.NamePrimaryDB, db: params.PrimaryDB, timeout: commonresources.DefaultPostgresPingTimeout()},
		redisHealthChecker{name: "redis." + resources.NameCacheRedis, client: params.CacheRedis, timeout: redisCfg.Timeout},
		casbinPolicyHealthChecker{engine: params.CasbinPolicy},
		watcherHealthChecker{watcher: params.PolicyWatcher, maxStaleness: params.RBAC.PolicyWatcher.MaxStaleness},
	}
	return router.HealthChecks{Readiness: checks, Startup: checks}
}

// Name 返回数据库健康检查项的唯一名称。
func (c postgresHealthChecker) Name() string {
	return c.name
}

// Check 在独立依赖超时内探测 PostgreSQL 连接池。
func (c postgresHealthChecker) Check(ctx context.Context) router.HealthCheckResult {
	if c.db == nil {
		return unavailableHealthResult(c.name, "postgres unavailable")
	}
	pingCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	if err := c.db.PingContext(pingCtx); err != nil {
		return unavailableHealthResult(c.name, "postgres unavailable")
	}
	return okHealthResult(c.name)
}

// Name 返回缓存服务健康检查项的唯一名称。
func (c redisHealthChecker) Name() string {
	return c.name
}

// Check 在独立依赖超时内探测 Redis 客户端。
func (c redisHealthChecker) Check(ctx context.Context) router.HealthCheckResult {
	if c.client == nil {
		return unavailableHealthResult(c.name, "redis unavailable")
	}
	pingCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	if err := c.client.Ping(pingCtx).Err(); err != nil {
		return unavailableHealthResult(c.name, "redis unavailable")
	}
	return okHealthResult(c.name)
}

// Name 返回权限规则状态检查项的唯一名称。
func (c casbinPolicyHealthChecker) Name() string {
	return "rbac.casbin_policy"
}

// Check 确认 Casbin policy 投影已经达到可授权状态。
func (c casbinPolicyHealthChecker) Check(context.Context) router.HealthCheckResult {
	name := c.Name()
	if c.engine == nil {
		return unavailableHealthResult(name, "casbin policy unavailable")
	}
	if !c.engine.ProjectionStatus().Ready() {
		return unavailableHealthResult(name, "casbin policy unavailable")
	}
	return okHealthResult(name)
}

// Name 返回权限更新广播监听检查项的唯一名称。
func (c watcherHealthChecker) Name() string {
	return "rbac.policy_watcher"
}

// Check 确认 watcher 正在运行，且最近一次权威校准未超过允许的新鲜度窗口。
func (c watcherHealthChecker) Check(context.Context) router.HealthCheckResult {
	name := c.Name()
	if c.watcher == nil {
		return unavailableHealthResult(name, "rbac policy watcher unavailable")
	}
	status := c.watcher.Status()
	if !status.Running {
		return unavailableHealthResult(name, "rbac policy watcher stopped")
	}
	if status.LastReconcileSuccessAt.IsZero() {
		return unavailableHealthResult(name, "rbac policy watcher not synchronized")
	}
	now := time.Now
	if c.now != nil {
		now = c.now
	}
	if now().Sub(status.LastReconcileSuccessAt) > c.maxStaleness {
		return unavailableHealthResult(name, "rbac policy watcher stale")
	}
	return okHealthResult(name)
}

// okHealthResult 构造一个检查正常的成功检查结果。
func okHealthResult(name string) router.HealthCheckResult {
	return router.HealthCheckResult{Name: name, Status: router.HealthCheckStatusOK}
}

// unavailableHealthResult 构造一个检查不合格的故障检查结果，并附带直观的故障原因说明。
func unavailableHealthResult(name string, message string) router.HealthCheckResult {
	if message == "" {
		message = "dependency unavailable"
	}
	return router.HealthCheckResult{Name: name, Status: router.HealthCheckStatusUnavailable, Message: message}
}
