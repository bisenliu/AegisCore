package redis

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/aegiscore/common/runtime/logger"
	permissionapplication "github.com/aegiscore/user-service/internal/features/permission/application"
)

const defaultCheckInterval = 15 * time.Second

// WatcherStatus 暴露 RBAC policy watcher 的只读运行状态。
type WatcherStatus interface {
	Running() bool
	LastError() error
}

// WatcherParams 包含 RBAC policy watcher 所需依赖。
type WatcherParams struct {
	Store   *Store
	Tracker *VersionTracker
	Engine  permissionapplication.PolicyReloadEngine
	Log     *zap.Logger
	Metrics permissionapplication.Metrics
}

// Watcher 监听 RBAC policy 分布式 revision 并执行补偿 reload。
type Watcher struct {
	store         policySubscriptionStore
	tracker       *VersionTracker
	engine        permissionapplication.PolicyReloadEngine
	log           *zap.Logger
	metrics       permissionapplication.Metrics
	checkInterval time.Duration

	mu      sync.Mutex
	cancel  context.CancelFunc
	done    chan struct{}
	running bool
	lastErr error
}

// NewWatcher 只构造 RBAC policy watcher；调用方负责显式调用 Start 和 Stop。
func NewWatcher(params WatcherParams) *Watcher {
	return newWatcherWithMetrics(params.Store, params.Tracker, params.Engine, params.Log, defaultCheckInterval, params.Metrics)
}

func newWatcherWithMetrics(store policySubscriptionStore, tracker *VersionTracker, engine permissionapplication.PolicyReloadEngine, log *zap.Logger, checkInterval time.Duration, metrics permissionapplication.Metrics) *Watcher {
	if checkInterval <= 0 {
		checkInterval = defaultCheckInterval
	}
	if metrics == nil {
		metrics = permissionapplication.NopMetrics()
	}
	return &Watcher{store: store, tracker: tracker, engine: engine, log: log, metrics: metrics, checkInterval: checkInterval}
}

// Start 启动 Pub/Sub 监听和定时版本补偿检查。
func (w *Watcher) Start() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.done != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	if w.log != nil {
		ctx = logger.ToContext(ctx, w.log)
	}
	w.cancel = cancel
	w.done = make(chan struct{})
	w.running = true
	w.lastErr = nil
	done := w.done
	go w.run(ctx, done)
}

// Stop 停止 Pub/Sub 监听和定时版本补偿检查。
func (w *Watcher) Stop(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	w.mu.Lock()
	cancel := w.cancel
	done := w.done
	w.mu.Unlock()
	if cancel == nil || done == nil {
		return nil
	}
	cancel()
	select {
	case <-done:
		w.mu.Lock()
		if w.done == done {
			w.cancel = nil
			w.done = nil
		}
		w.running = false
		w.mu.Unlock()
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Running 返回 watcher 后台循环当前是否处于运行状态。
func (w *Watcher) Running() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.running
}

// LastError 返回 watcher 最近一次非预期后台错误。
func (w *Watcher) LastError() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.lastErr
}

// CheckVersion 执行一次 Redis revision 补偿检查。
// Pub/Sub 只是快速路径，Redis 缓存已知最大数据库 revision；定时检查负责发现漏消息或重启期间错过的 revision。
func (w *Watcher) CheckVersion(ctx context.Context) {
	remoteVersion, err := w.store.CurrentVersion(ctx)
	if err != nil {
		w.metrics.WatcherCheckFailed(ctx, permissionapplication.MetricsReasonStoreUnavailable)
		logger.Error(ctx, "rbac policy version check failed", logger.StackTrace(zap.Error(err))...)
		return
	}
	localVersion := w.tracker.Applied()
	w.observeLag(ctx, remoteVersion, localVersion)
	if remoteVersion <= localVersion {
		return
	}
	logger.Warn(ctx, "rbac policy revision mismatch detected", zap.Int64("local_policy_revision", localVersion), zap.Int64("remote_policy_revision", remoteVersion), zap.String("reason", "version_check"))
	w.metrics.WatcherVersionMismatch(ctx, permissionapplication.MetricsSourceWatcherVersionCheck)
	w.applyIfNewer(ctx, remoteVersion, permissionapplication.NewPolicyReloadChange("version_check"), "", permissionapplication.MetricsSourceWatcherVersionCheck)
}

// HandlePayload 处理一条 RBAC policy Pub/Sub payload。
// payload 可能只要求失效某个用户角色缓存，也可能要求全量 reload；最终是否执行取决于远端版本是否新于本地已应用版本。
func (w *Watcher) HandlePayload(ctx context.Context, payload string) {
	message, err := decodePolicyRefreshMessage(payload)
	if err != nil {
		logger.Error(ctx, "rbac policy refresh message invalid", logger.StackTrace(zap.Error(err))...)
		return
	}
	localVersion := w.tracker.Applied()
	w.observeLag(ctx, message.Version, localVersion)
	logger.Info(ctx, "rbac policy refresh received", zap.Int64("remote_policy_revision", message.Version), zap.Int64("local_policy_revision", localVersion), zap.String("instance_id", message.InstanceID), zap.String("reason", message.Reason))
	w.applyIfNewer(ctx, message.Version, message.policyChange(), message.InstanceID, permissionapplication.MetricsSourceWatcherPubSub)
}

func (w *Watcher) run(ctx context.Context, done chan struct{}) {
	defer func() {
		w.mu.Lock()
		if w.done == done {
			w.cancel = nil
			w.done = nil
		}
		w.running = false
		w.mu.Unlock()
		close(done)
	}()
	pubsub := w.store.Subscribe(ctx)
	defer func() { _ = pubsub.Close() }()
	if _, err := pubsub.Receive(ctx); err != nil && !errors.Is(err, context.Canceled) {
		w.recordError(err)
		logger.Error(ctx, "rbac policy refresh subscribe failed", logger.StackTrace(zap.Error(err))...)
	}
	ticker := time.NewTicker(w.checkInterval)
	defer ticker.Stop()
	channel := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case message, ok := <-channel:
			if !ok {
				if ctx.Err() == nil {
					w.recordError(errors.New("rbac policy refresh channel closed"))
				}
				return
			}
			w.HandlePayload(ctx, message.Payload)
		case <-ticker.C:
			w.CheckVersion(ctx)
		}
	}
}

func (w *Watcher) applyIfNewer(ctx context.Context, version int64, change permissionapplication.PolicyChange, instanceID string, source string) {
	localVersion := w.tracker.Applied()
	w.observeLag(ctx, version, localVersion)
	// revision 是跨实例幂等门禁；已经应用过的 revision 必须跳过，避免旧 Pub/Sub 消息覆盖后续 reload 状态。
	if version <= localVersion {
		return
	}
	reason := change.ReasonText()
	if change.RequiresReload() {
		if err := w.engine.Reload(ctx); err != nil {
			w.metrics.WatcherReloadFailed(ctx, source, permissionapplication.MetricsReasonReloadFailed)
			logger.Error(ctx, "rbac policy remote refresh failed", logger.StackTrace(zap.Int64("policy_revision", version), zap.Int64("local_policy_revision", localVersion), zap.String("instance_id", instanceID), zap.String("reason", reason), zap.Error(err))...)
			return
		}
		w.engine.InvalidateAllUserRoles()
		w.metrics.WatcherReloadSucceeded(ctx, source)
	} else if change.UserID != uuid.Nil {
		w.engine.InvalidateUserRole(change.UserID)
	} else {
		w.engine.InvalidateAllUserRoles()
	}
	w.tracker.MarkApplied(version)
	w.observeLag(ctx, version, w.tracker.Applied())
	logger.Info(ctx, "rbac policy remote refresh succeeded", zap.Int64("policy_revision", version), zap.Int64("local_policy_revision", w.tracker.Applied()), zap.String("instance_id", instanceID), zap.String("reason", reason))
}

func (w *Watcher) observeLag(ctx context.Context, remoteVersion int64, localVersion int64) {
	lag := remoteVersion - localVersion
	if lag < 0 {
		lag = 0
	}
	w.metrics.PolicyReloadLagObserved(ctx, lag)
}

func (w *Watcher) recordError(err error) {
	w.mu.Lock()
	w.lastErr = err
	w.mu.Unlock()
}
