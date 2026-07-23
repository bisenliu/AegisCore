package permission

import (
	"context"

	"go.uber.org/fx"
	"go.uber.org/zap"

	commonconfig "github.com/aegiscore/common/runtime/config"
	permissionapplication "github.com/aegiscore/user-service/internal/features/permission/application"
	permissionredis "github.com/aegiscore/user-service/internal/features/permission/infrastructure/redis"
)

// Fx 选项

// permissionPolicySyncOptions 组装跨副本 policy version 发布、追踪和 watcher 同步能力。
var permissionPolicySyncOptions = fx.Options(
	fx.Provide(
		provideRedisStore,
		provideVersionTracker,
		providePolicyChangeNotifier,
		provideWatcher,
		fx.Private,
	),
)

// Fx 参数与结果：Policy 同步

// WatcherParams 汇集 watcher 运行时依赖，watcher 只通过 reload 端口触发内存策略刷新。
type WatcherParams struct {
	fx.In

	Store   *permissionredis.Store
	Tracker *permissionredis.VersionTracker
	Engine  permissionapplication.PolicyReloadEngine `name:"permission_policy_reload_engine"`
	Log     *zap.Logger
	Metrics permissionapplication.Metrics
}

// PolicyChangeNotifierParams 汇集本实例写操作后的本地 reload 与远端版本通知依赖。
type PolicyChangeNotifierParams struct {
	fx.In

	Engine    permissionapplication.PolicyReloadEngine `name:"permission_policy_reload_engine"`
	Publisher permissionapplication.PolicyVersionPublisher
	Tracker   permissionapplication.PolicyVersionTracker
	Log       *zap.Logger
	Metrics   permissionapplication.Metrics
}

type PolicyChangeNotifierResult struct {
	fx.Out

	Notifier permissionapplication.PolicyChangeNotifier `name:"permission_policy_change_notifier"`
}

type PolicyRedisStoreResult struct {
	fx.Out

	Store     *permissionredis.Store
	Publisher permissionapplication.PolicyVersionPublisher
}

type PolicyVersionTrackerResult struct {
	fx.Out

	Tracker *permissionredis.VersionTracker
	Port    permissionapplication.PolicyVersionTracker
}

type PolicyWatcherResult struct {
	fx.Out

	Watcher *permissionredis.Watcher
	Runner  permissionApplicationWatcher              `name:"permission_policy_watcher_runner"`
	Status  permissionapplication.PolicyWatcherStatus `name:"permission_policy_watcher_status"`
}

// permissionApplicationWatcher 是 lifecycle 对 policy watcher 的最小控制面。
type permissionApplicationWatcher interface {
	Start()
	Stop(context.Context) error
}

// Provider：Policy 同步

func provideRedisStore(params CacheRedisParams, cfg *commonconfig.Config, log *zap.Logger) (PolicyRedisStoreResult, error) {
	store, err := permissionredis.NewStore(params.Client, cfg, log)
	if err != nil {
		return PolicyRedisStoreResult{}, err
	}
	return PolicyRedisStoreResult{Store: store, Publisher: store}, nil
}

func provideVersionTracker() PolicyVersionTrackerResult {
	tracker := permissionredis.NewVersionTracker()
	return PolicyVersionTrackerResult{Tracker: tracker, Port: tracker}
}

// providePolicyChangeNotifier 复用同一 coordinator 串联本地 reload、Redis publish 和版本追踪。
func providePolicyChangeNotifier(params PolicyChangeNotifierParams) PolicyChangeNotifierResult {
	return PolicyChangeNotifierResult{Notifier: permissionapplication.NewPolicyRefreshCoordinator(params.Engine, params.Publisher, params.Tracker, params.Log, params.Metrics)}
}

// provideWatcher 将 Redis watcher 同时投影为 lifecycle runner 和健康状态来源。
func provideWatcher(params WatcherParams) PolicyWatcherResult {
	watcher := permissionredis.NewWatcher(permissionredis.WatcherParams{Store: params.Store, Tracker: params.Tracker, Engine: params.Engine, Log: params.Log, Metrics: params.Metrics})
	return PolicyWatcherResult{Watcher: watcher, Runner: watcher, Status: watcher}
}
