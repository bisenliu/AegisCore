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
	Store          *Store
	RevisionSource permissionapplication.LatestPolicyRevisionSource
	Engine         permissionapplication.PolicyReloadEngine
	Log            *zap.Logger
	Metrics        permissionapplication.Metrics
}

// Watcher 监听 RBAC policy 分布式 revision 并执行补偿 reload。
type Watcher struct {
	store          policySubscriptionStore
	revisionSource permissionapplication.LatestPolicyRevisionSource
	engine         permissionapplication.PolicyReloadEngine
	log            *zap.Logger
	metrics        permissionapplication.Metrics
	checkInterval  time.Duration

	mu      sync.Mutex
	cancel  context.CancelFunc
	done    chan struct{}
	running bool
	lastErr error
}

// NewWatcher 只构造 RBAC policy watcher；调用方负责显式调用 Start 和 Stop。
func NewWatcher(params WatcherParams) *Watcher {
	return newWatcherWithMetrics(params.Store, params.RevisionSource, params.Engine, params.Log, defaultCheckInterval, params.Metrics)
}

func newWatcherWithMetrics(store policySubscriptionStore, revisionSource permissionapplication.LatestPolicyRevisionSource, engine permissionapplication.PolicyReloadEngine, log *zap.Logger, checkInterval time.Duration, metrics permissionapplication.Metrics) *Watcher {
	if checkInterval <= 0 {
		checkInterval = defaultCheckInterval
	}
	if metrics == nil {
		metrics = permissionapplication.NopMetrics()
	}
	return &Watcher{store: store, revisionSource: revisionSource, engine: engine, log: log, metrics: metrics, checkInterval: checkInterval}
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

// CheckVersion 执行一次数据库 revision 补偿检查。
// Pub/Sub 只是唤醒 hint；定时检查直接读取数据库提交事实，负责发现漏消息或 Redis 状态丢失。
func (w *Watcher) CheckVersion(ctx context.Context) {
	databaseLatest, err := w.latestRevision(ctx, permissionapplication.MetricsSourceWatcherRevisionCheck, 0)
	if err != nil {
		return
	}
	localApplied := w.engine.AppliedRevision()
	w.observeLag(ctx, databaseLatest, localApplied)
	status := w.engine.ProjectionStatus()
	if databaseLatest <= localApplied && status.Ready() {
		return
	}
	logger.Warn(ctx, "rbac policy revision mismatch detected", zap.Int64("database_latest_policy_revision", databaseLatest), zap.Int64("local_applied_policy_revision", localApplied), zap.String("source", permissionapplication.MetricsSourceWatcherRevisionCheck), zap.String("reason", permissionapplication.MetricsReasonRevisionMismatch))
	w.metrics.WatcherVersionMismatch(ctx, permissionapplication.MetricsSourceWatcherRevisionCheck, permissionapplication.MetricsReasonRevisionMismatch)
	w.ObserveTargetRevision(ctx, databaseLatest, permissionapplication.NewPolicyReloadChange("database_revision_check"), "", permissionapplication.MetricsSourceWatcherRevisionCheck)
}

// HandlePayload 处理一条 RBAC policy Pub/Sub payload。
// 每条有效消息都必须执行副作用；notification revision 只作为唤醒 hint，不代表数据库目标或已应用投影。
func (w *Watcher) HandlePayload(ctx context.Context, payload string) {
	message, err := decodePolicyRefreshMessage(payload)
	if err != nil {
		logger.Error(ctx, "rbac policy refresh message invalid", logger.StackTrace(zap.Error(err))...)
		return
	}
	localApplied := w.engine.AppliedRevision()
	logger.Info(ctx, "rbac policy refresh hint received", zap.Int64("hint_revision", message.PolicyRevision), zap.Int64("local_applied_policy_revision", localApplied), zap.String("instance_id", message.InstanceID), zap.String("source", permissionapplication.MetricsSourceWatcherPubSub), zap.String("reason", message.Reason))
	databaseLatest, err := w.latestRevision(ctx, permissionapplication.MetricsSourceWatcherPubSub, message.PolicyRevision)
	if err != nil {
		w.invalidateChange(message.policyChange())
		return
	}
	w.observeLag(ctx, databaseLatest, localApplied)
	if databaseLatest > localApplied {
		logger.Warn(ctx, "rbac policy revision mismatch detected", zap.Int64("database_latest_policy_revision", databaseLatest), zap.Int64("local_applied_policy_revision", localApplied), zap.Int64("hint_revision", message.PolicyRevision), zap.String("source", permissionapplication.MetricsSourceWatcherPubSub), zap.String("reason", permissionapplication.MetricsReasonRevisionMismatch))
		w.metrics.WatcherVersionMismatch(ctx, permissionapplication.MetricsSourceWatcherPubSub, permissionapplication.MetricsReasonRevisionMismatch)
	}
	w.ObserveTargetRevision(ctx, databaseLatest, message.policyChange(), message.InstanceID, permissionapplication.MetricsSourceWatcherPubSub)
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

// ObserveTargetRevision 处理数据库 revision 目标，并始终保留消息要求的缓存失效副作用。
func (w *Watcher) ObserveTargetRevision(ctx context.Context, targetRevision int64, change permissionapplication.PolicyChange, instanceID string, source string) {
	localApplied := w.engine.AppliedRevision()
	reason := change.ReasonText()
	status := w.engine.ProjectionStatus()
	if targetRevision > localApplied || !status.Ready() {
		appliedRevision, err := w.engine.ReloadToRevision(ctx, targetRevision)
		if err != nil {
			w.metrics.WatcherReloadFailed(ctx, source, permissionapplication.MetricsReasonReloadFailed)
			w.observeLag(ctx, targetRevision, appliedRevision)
			logger.Error(ctx, "rbac policy projection reload failed", logger.StackTrace(zap.Int64("target_revision", targetRevision), zap.Int64("local_applied_policy_revision", appliedRevision), zap.String("instance_id", instanceID), zap.String("source", source), zap.String("reason", reason), zap.Error(err))...)
			return
		}
		status = w.engine.ProjectionStatus()
		if !status.Ready() || status.AppliedRevision < targetRevision {
			w.metrics.WatcherReloadFailed(ctx, source, permissionapplication.MetricsReasonReloadFailed)
			w.observeLag(ctx, targetRevision, status.AppliedRevision)
			logger.Error(ctx, "rbac policy projection reload incomplete", zap.Int64("target_revision", targetRevision), zap.Int64("local_applied_policy_revision", status.AppliedRevision), zap.String("instance_id", instanceID), zap.String("source", source), zap.String("reason", reason))
			return
		}
		w.metrics.WatcherReloadSucceeded(ctx, source)
		w.observeLag(ctx, targetRevision, status.AppliedRevision)
	}
	w.invalidateChange(change)
	appliedRevision := w.engine.AppliedRevision()
	logger.Info(ctx, "rbac policy projection synchronized", zap.Int64("target_revision", targetRevision), zap.Int64("local_applied_policy_revision", appliedRevision), zap.Int64("previous_applied_policy_revision", localApplied), zap.String("instance_id", instanceID), zap.String("source", source), zap.String("reason", reason))
}

func (w *Watcher) invalidateChange(change permissionapplication.PolicyChange) {
	if change.UserID != uuid.Nil {
		w.engine.InvalidateUserRole(change.UserID)
	} else if change.RequiresReload() {
		w.engine.InvalidateAllUserRoles()
	}
}

func (w *Watcher) latestRevision(ctx context.Context, source string, hintRevision int64) (int64, error) {
	if w.revisionSource == nil {
		err := errors.New("rbac policy revision source is required")
		w.metrics.WatcherCheckFailed(ctx, source, permissionapplication.MetricsReasonRevisionStoreUnavailable)
		logger.Error(ctx, "rbac policy revision query failed", logger.StackTrace(zap.Int64("hint_revision", hintRevision), zap.Int64("local_applied_policy_revision", w.engine.AppliedRevision()), zap.String("source", source), zap.String("reason", permissionapplication.MetricsReasonRevisionStoreUnavailable), zap.Error(err))...)
		return 0, err
	}
	revision, err := w.revisionSource.LatestPolicyRevision(ctx)
	if err != nil {
		w.metrics.WatcherCheckFailed(ctx, source, permissionapplication.MetricsReasonRevisionStoreUnavailable)
		logger.Error(ctx, "rbac policy revision query failed", logger.StackTrace(zap.Int64("hint_revision", hintRevision), zap.Int64("local_applied_policy_revision", w.engine.AppliedRevision()), zap.String("source", source), zap.String("reason", permissionapplication.MetricsReasonRevisionStoreUnavailable), zap.Error(err))...)
		return 0, err
	}
	return revision, nil
}

func (w *Watcher) observeLag(ctx context.Context, databaseLatest int64, localApplied int64) {
	lag := databaseLatest - localApplied
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
