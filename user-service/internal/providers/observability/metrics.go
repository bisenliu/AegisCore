package observability

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"
	"go.uber.org/fx"

	"github.com/aegiscore/common/runtime/localcache"
	commonmetrics "github.com/aegiscore/common/runtime/observability/metrics"
	serviceconfig "github.com/aegiscore/user-service/internal/config"
	authredis "github.com/aegiscore/user-service/internal/features/auth/infrastructure/redis"
	permissionapplication "github.com/aegiscore/user-service/internal/features/permission/application"
	"github.com/aegiscore/user-service/internal/resources"
)

const (
	authSessionPurgePoolMetricsName = "auth_session_purge_pool"
	redisMetricsMinProbeInterval    = 15 * time.Second
	watcherRunningMetricName        = "aegiscore_user_service_rbac_policy_watcher_running"
	watcherSubscriptionMetricName   = "aegiscore_user_service_rbac_policy_watcher_subscription_state"
	watcherSubscriptionSuccessName  = "aegiscore_user_service_rbac_policy_watcher_last_subscription_success_timestamp_seconds"
	watcherReconcileSuccessName     = "aegiscore_user_service_rbac_policy_watcher_last_reconcile_success_timestamp_seconds"
	watcherStalenessMetricName      = "aegiscore_user_service_rbac_policy_watcher_reconcile_staleness_seconds"
	watcherMaxStalenessMetricName   = "aegiscore_user_service_rbac_policy_watcher_max_staleness_seconds"
	watcherReconnectAttemptsName    = "aegiscore_user_service_rbac_policy_watcher_reconnect_attempts_total"
)

var watcherSubscriptionStates = []permissionapplication.PolicyWatcherSubscriptionState{
	permissionapplication.PolicyWatcherSubscriptionStarting,
	permissionapplication.PolicyWatcherSubscriptionConnected,
	permissionapplication.PolicyWatcherSubscriptionReconnecting,
	permissionapplication.PolicyWatcherSubscriptionStopped,
}

type policyWatcherCollector struct {
	source                  permissionapplication.PolicyWatcherStatus
	now                     func() time.Time
	maxStaleness            time.Duration
	running                 *prometheus.Desc
	subscriptionState       *prometheus.Desc
	lastSubscriptionSuccess *prometheus.Desc
	lastReconcileSuccess    *prometheus.Desc
	staleness               *prometheus.Desc
	maxStalenessDesc        *prometheus.Desc
	reconnectAttempts       *prometheus.Desc
}

func newPolicyWatcherCollector(source permissionapplication.PolicyWatcherStatus, maxStaleness time.Duration) (*policyWatcherCollector, error) {
	if source == nil {
		return nil, fmt.Errorf("rbac policy watcher status source is required")
	}
	if maxStaleness <= 0 {
		return nil, fmt.Errorf("rbac policy watcher max staleness must be positive")
	}
	return &policyWatcherCollector{
		source:       source,
		now:          time.Now,
		maxStaleness: maxStaleness,
		running: prometheus.NewDesc(watcherRunningMetricName,
			"Whether the RBAC policy watcher root lifecycle is running.", nil, nil),
		subscriptionState: prometheus.NewDesc(watcherSubscriptionMetricName,
			"Current RBAC policy watcher subscription state as a one-hot gauge.", []string{"state"}, nil),
		lastSubscriptionSuccess: prometheus.NewDesc(watcherSubscriptionSuccessName,
			"Unix timestamp of the last successful RBAC policy subscription confirmation.", nil, nil),
		lastReconcileSuccess: prometheus.NewDesc(watcherReconcileSuccessName,
			"Unix timestamp of the last successful authoritative RBAC policy reconciliation.", nil, nil),
		staleness: prometheus.NewDesc(watcherStalenessMetricName,
			"Seconds since the last successful authoritative RBAC policy reconciliation.", nil, nil),
		maxStalenessDesc: prometheus.NewDesc(watcherMaxStalenessMetricName,
			"Configured maximum allowed RBAC policy reconciliation staleness in seconds.", nil, nil),
		reconnectAttempts: prometheus.NewDesc(watcherReconnectAttemptsName,
			"Total RBAC policy watcher subscription reconnect attempts.", nil, nil),
	}, nil
}

func (c *policyWatcherCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.running
	ch <- c.subscriptionState
	ch <- c.lastSubscriptionSuccess
	ch <- c.lastReconcileSuccess
	ch <- c.staleness
	ch <- c.maxStalenessDesc
	ch <- c.reconnectAttempts
}

func (c *policyWatcherCollector) Collect(ch chan<- prometheus.Metric) {
	status := c.source.Status()
	ch <- prometheus.MustNewConstMetric(c.running, prometheus.GaugeValue, boolMetric(status.Running))
	for _, state := range watcherSubscriptionStates {
		ch <- prometheus.MustNewConstMetric(c.subscriptionState, prometheus.GaugeValue, boolMetric(status.SubscriptionState == state), string(state))
	}
	ch <- prometheus.MustNewConstMetric(c.lastSubscriptionSuccess, prometheus.GaugeValue, timestampMetric(status.LastSubscriptionSuccessAt))
	ch <- prometheus.MustNewConstMetric(c.lastReconcileSuccess, prometheus.GaugeValue, timestampMetric(status.LastReconcileSuccessAt))
	staleness := 0.0
	if !status.LastReconcileSuccessAt.IsZero() {
		age := c.now().Sub(status.LastReconcileSuccessAt)
		if age > 0 {
			staleness = age.Seconds()
		}
	}
	ch <- prometheus.MustNewConstMetric(c.staleness, prometheus.GaugeValue, staleness)
	ch <- prometheus.MustNewConstMetric(c.maxStalenessDesc, prometheus.GaugeValue, c.maxStaleness.Seconds())
	ch <- prometheus.MustNewConstMetric(c.reconnectAttempts, prometheus.CounterValue, float64(status.ReconnectAttempts))
}

func boolMetric(value bool) float64 {
	if value {
		return 1
	}
	return 0
}

func timestampMetric(value time.Time) float64 {
	if value.IsZero() {
		return 0
	}
	return float64(value.Unix())
}

// RuntimeDependencyMetricsParams 包含注册运行时依赖指标所需的服务级依赖。
type RuntimeDependencyMetricsParams struct {
	fx.In

	Resources        serviceconfig.ResourceSettings
	RBAC             serviceconfig.RBACSettings
	Metrics          *commonmetrics.Provider
	PrimaryDB        *sql.DB                 `name:"primary_db"`
	CacheRedis       redis.UniversalClient   `name:"cache_redis"`
	SessionPurgePool authredis.PurgeTaskPool `name:"auth_session_purge_pool"`
	PolicyWatcher    permissionapplication.PolicyWatcherStatus
	AuthTokenCache   localcache.StatsSource `name:"auth_token_version_cache"`
	RBACRolesCache   localcache.StatsSource `name:"rbac_user_roles_cache"`
}

// RegisterRuntimeDependencyMetrics 注册用户服务运行时依赖 Prometheus collector。
func RegisterRuntimeDependencyMetrics(params RuntimeDependencyMetricsParams) error {
	if params.Metrics == nil || !params.Metrics.Enabled() {
		return nil
	}

	primaryDBCollector, err := commonmetrics.NewSQLDBCollector(commonmetrics.SQLDBCollectorOptions{
		Resource: resources.NamePrimaryDB,
		DB:       params.PrimaryDB,
	})
	if err != nil {
		return fmt.Errorf("create postgres metrics collector: %w", err)
	}
	if err := params.Metrics.Register(primaryDBCollector); err != nil {
		return err
	}

	redisCfg := params.Resources.Redis[resources.NameCacheRedis]
	redisCollector, err := commonmetrics.NewRedisPingCollector(commonmetrics.RedisPingCollectorOptions{
		Resource:    resources.NameCacheRedis,
		Pinger:      commonmetrics.NewRedisClientPinger(params.CacheRedis),
		Timeout:     redisCfg.Timeout,
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

	watcherCollector, err := newPolicyWatcherCollector(params.PolicyWatcher, params.RBAC.PolicyWatcher.MaxStaleness)
	if err != nil {
		return fmt.Errorf("create rbac watcher metrics collector: %w", err)
	}
	return params.Metrics.Register(watcherCollector)
}
