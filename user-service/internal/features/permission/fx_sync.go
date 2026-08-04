package permission

import (
	"context"

	"go.uber.org/fx"
	"go.uber.org/zap"

	serviceconfig "github.com/aegiscore/user-service/internal/config"
	permissionapplication "github.com/aegiscore/user-service/internal/features/permission/application"
	permissionredis "github.com/aegiscore/user-service/internal/features/permission/infrastructure/redis"
)

// Fx 选项

// permissionPolicySyncOptions 组装跨副本 policy revision 发布和 watcher 同步能力。
var permissionPolicySyncOptions = fx.Options(
	fx.Provide(
		provideRedisStore,
		providePolicyChangeNotifier,
		provideWatcher,
		provideOutboxDispatcher,
		fx.Private,
	),
)

// Fx 参数与结果：Policy 同步

// WatcherParams 汇集 watcher 运行时依赖，watcher 只通过 reload 端口触发内存策略刷新。
type WatcherParams struct {
	fx.In

	Store          *permissionredis.Store
	RevisionSource permissionapplication.LatestPolicyRevisionSource
	Engine         permissionapplication.PolicyReloadEngine `name:"permission_policy_reload_engine"`
	Settings       serviceconfig.RBACSettings
	Log            *zap.Logger
	Metrics        permissionapplication.Metrics
}

// PolicyChangeNotifierParams 汇集本实例写操作后的本地 reload 与远端版本通知依赖。
type PolicyChangeNotifierParams struct {
	fx.In

	Engine  permissionapplication.PolicyReloadEngine `name:"permission_policy_reload_engine"`
	Log     *zap.Logger
	Metrics permissionapplication.Metrics
}

type PolicyChangeNotifierResult struct {
	fx.Out

	Notifier permissionapplication.PolicyChangeNotifier `name:"permission_policy_change_notifier"`
}

type PolicyRedisStoreResult struct {
	fx.Out

	Store     *permissionredis.Store
	Publisher permissionapplication.PolicyRevisionPublisher
}

type PolicyWatcherResult struct {
	fx.Out

	Watcher *permissionredis.Watcher
	Runner  policyWatcherRunner                       `name:"permission_policy_watcher_runner"`
	Status  permissionapplication.PolicyWatcherStatus `name:"permission_policy_watcher_status"`
}

type OutboxDispatcherParams struct {
	fx.In

	Store     permissionapplication.OutboxStore
	Publisher permissionapplication.PolicyRevisionPublisher
	Settings  serviceconfig.RBACSettings
	Log       *zap.Logger
	Metrics   permissionapplication.Metrics
}

type OutboxDispatcherResult struct {
	fx.Out

	Runner permissionapplication.OutboxDispatcherRunner `name:"permission_outbox_dispatcher_runner"`
	Status permissionapplication.OutboxDispatcherStatus `name:"permission_outbox_dispatcher_status"`
}

// policyWatcherRunner 是 lifecycle 对 policy watcher 的最小控制面。
type policyWatcherRunner interface {
	Start()
	Stop(context.Context) error
}

// Provider：Policy 同步

func provideRedisStore(params CacheRedisParams, settings serviceconfig.RBACSettings, log *zap.Logger) (PolicyRedisStoreResult, error) {
	store, err := permissionredis.NewStore(params.Client, settings.AppName, log)
	if err != nil {
		return PolicyRedisStoreResult{}, err
	}
	return PolicyRedisStoreResult{Store: store, Publisher: store}, nil
}

// providePolicyChangeNotifier 只编排本实例 revision-aware reload 和缓存失效。
func providePolicyChangeNotifier(params PolicyChangeNotifierParams) PolicyChangeNotifierResult {
	return PolicyChangeNotifierResult{Notifier: permissionapplication.NewPolicyRefreshCoordinator(params.Engine, params.Log, params.Metrics)}
}

// provideWatcher 将 Redis watcher 同时投影为 lifecycle runner 和健康状态来源。
func provideWatcher(params WatcherParams) PolicyWatcherResult {
	settings := params.Settings.PolicyWatcher
	watcher := permissionredis.NewWatcher(permissionredis.WatcherParams{
		Store: params.Store, RevisionSource: params.RevisionSource, Engine: params.Engine, Log: params.Log, Metrics: params.Metrics,
		Settings: permissionredis.WatcherSettings{
			CheckInterval: settings.CheckInterval, SubscribeTimeout: settings.SubscribeTimeout,
			BackoffInitial: settings.RetryBackoff.Initial, BackoffMax: settings.RetryBackoff.Max,
		},
	})
	return PolicyWatcherResult{Watcher: watcher, Runner: watcher, Status: watcher}
}

// provideOutboxDispatcher 将同一个 dispatcher 投影为 lifecycle runner 和只读状态来源。
func provideOutboxDispatcher(params OutboxDispatcherParams) (OutboxDispatcherResult, error) {
	config := params.Settings.OutboxDispatcher
	dispatcher, err := permissionapplication.NewDispatcher(params.Store, params.Publisher, permissionapplication.DispatcherSettings{
		PollInterval:   config.PollInterval,
		BatchSize:      config.BatchSize,
		ClaimTimeout:   config.ClaimTimeout,
		BackoffInitial: config.RetryBackoff.Initial,
		BackoffMax:     config.RetryBackoff.Max,
	}, nil, params.Log, params.Metrics)
	if err != nil {
		return OutboxDispatcherResult{}, err
	}
	return OutboxDispatcherResult{Runner: dispatcher, Status: dispatcher}, nil
}
