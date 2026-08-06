package redis

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/aegiscore/common/runtime/logger"
	"github.com/aegiscore/common/runtime/redispubsub"
	permissionapplication "github.com/aegiscore/user-service/internal/features/permission/application"
)

const defaultCheckInterval = 15 * time.Second

// WatcherSettings 控制 watcher 的数据库权威校准节奏。
type WatcherSettings struct {
	CheckInterval time.Duration
}

// WatcherParams 包含 RBAC policy watcher 所需依赖。
type WatcherParams struct {
	Subscriber     *redispubsub.Subscriber
	RevisionSource permissionapplication.LatestPolicyRevisionSource
	Engine         permissionapplication.PolicyReloadEngine
	Log            *zap.Logger
	Metrics        permissionapplication.Metrics
	Settings       WatcherSettings
}

// Watcher 监听 RBAC policy 分布式 revision 并执行补偿 reload。
type Watcher struct {
	source         messageSource
	revisionSource permissionapplication.LatestPolicyRevisionSource
	engine         permissionapplication.PolicyReloadEngine
	log            *zap.Logger
	metrics        permissionapplication.Metrics
	settings       WatcherSettings

	mu                     sync.Mutex
	cancel                 context.CancelFunc
	done                   chan struct{}
	stopped                bool
	status                 permissionapplication.PolicyWatcherStatusSnapshot
	lastReconcileFailureAt time.Time
}

type messageSource interface {
	Start() error
	Stop(context.Context) error
	Messages() <-chan redispubsub.Message
	Status() redispubsub.Status
}

// NewWatcher 只构造 RBAC policy watcher；调用方负责显式调用 Start 和 Stop。
func NewWatcher(params WatcherParams) *Watcher {
	return newWatcher(params.Subscriber, params.RevisionSource, params.Engine, params.Log, params.Settings, params.Metrics)
}

func newWatcher(source messageSource, revisionSource permissionapplication.LatestPolicyRevisionSource, engine permissionapplication.PolicyReloadEngine, log *zap.Logger, settings WatcherSettings, metrics permissionapplication.Metrics) *Watcher {
	if settings.CheckInterval <= 0 {
		settings.CheckInterval = defaultCheckInterval
	}
	if metrics == nil {
		metrics = permissionapplication.NopMetrics()
	}
	return &Watcher{source: source, revisionSource: revisionSource, engine: engine, log: log, metrics: metrics, settings: settings,
		status: permissionapplication.PolicyWatcherStatusSnapshot{
			SubscriptionState:         permissionapplication.PolicyWatcherSubscriptionStopped,
			SubscriptionErrorCategory: permissionapplication.PolicyWatcherErrorNone,
			ReconcileErrorCategory:    permissionapplication.PolicyWatcherErrorNone,
		}}
}

// Start 启动 Pub/Sub 监听和定时版本补偿检查。
func (w *Watcher) Start() error {
	w.mu.Lock()
	if w.stopped {
		w.mu.Unlock()
		return redispubsub.ErrStopped
	}
	if w.done != nil {
		w.mu.Unlock()
		return nil
	}
	if err := w.source.Start(); err != nil {
		w.mu.Unlock()
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	if w.log != nil {
		ctx = logger.ToContext(ctx, w.log)
	}
	w.cancel = cancel
	w.done = make(chan struct{})
	w.status.Running = true
	done := w.done
	w.mu.Unlock()
	go w.run(ctx, done)
	return nil
}

// Stop 停止 Pub/Sub 监听和定时版本补偿检查。
func (w *Watcher) Stop(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	w.mu.Lock()
	cancel := w.cancel
	done := w.done
	if done == nil {
		w.stopped = true
		w.status.Running = false
		w.mu.Unlock()
		return w.source.Stop(ctx)
	}
	w.stopped = true
	w.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	sourceErr := w.source.Stop(ctx)
	var watcherErr error
	select {
	case <-done:
	case <-ctx.Done():
		watcherErr = ctx.Err()
	}
	return errors.Join(sourceErr, watcherErr)
}

// Status 返回 watcher 当前结构化状态的只读快照。
func (w *Watcher) Status() permissionapplication.PolicyWatcherStatusSnapshot {
	w.mu.Lock()
	status := w.status
	lastReconcileFailureAt := w.lastReconcileFailureAt
	w.mu.Unlock()

	subscription := w.source.Status()
	status.SubscriptionState = mapSubscriptionState(subscription.State)
	status.SubscriptionErrorCategory = mapSubscriptionError(subscription.ErrorCategory)
	status.LastSubscriptionSuccessAt = subscription.LastConnectedAt
	status.ReconnectAttempts = subscription.Reconnects
	status.LastFailureAt = subscription.LastFailureAt
	if lastReconcileFailureAt.After(status.LastFailureAt) {
		status.LastFailureAt = lastReconcileFailureAt
	}
	return status
}

// CheckVersion 执行一次数据库 revision 补偿检查。
// Pub/Sub 只是唤醒 hint；定时检查直接读取数据库提交事实，负责发现漏消息或 Redis 状态丢失。
func (w *Watcher) CheckVersion(ctx context.Context) {
	databaseLatest, err := w.latestRevision(ctx, permissionapplication.MetricsSourceWatcherRevisionCheck, 0)
	if err != nil {
		if ctx.Err() != nil {
			return
		}
		w.markReconcileFailure(ctx, permissionapplication.PolicyWatcherErrorRevisionSource)
		return
	}
	localApplied := w.engine.AppliedRevision()
	w.observeLag(ctx, databaseLatest, localApplied)
	status := w.engine.ProjectionStatus()
	if databaseLatest <= localApplied && status.Ready() {
		w.markReconcileSuccess()
		return
	}
	logger.Warn(ctx, "rbac policy revision mismatch detected", zap.Int64("database_latest_policy_revision", databaseLatest), zap.Int64("local_applied_policy_revision", localApplied), zap.String("source", permissionapplication.MetricsSourceWatcherRevisionCheck), zap.String("reason", permissionapplication.MetricsReasonRevisionMismatch))
	w.metrics.WatcherVersionMismatch(ctx, permissionapplication.MetricsSourceWatcherRevisionCheck, permissionapplication.MetricsReasonRevisionMismatch)
	if err := w.ObserveTargetRevision(ctx, databaseLatest, permissionapplication.NewPolicyReloadChange("database_revision_check"), "", permissionapplication.MetricsSourceWatcherRevisionCheck); err != nil {
		if ctx.Err() != nil {
			return
		}
		w.markReconcileFailure(ctx, permissionapplication.PolicyWatcherErrorReload)
		return
	}
	w.markReconcileSuccess()
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
		if ctx.Err() != nil {
			return
		}
		w.markReconcileFailure(ctx, permissionapplication.PolicyWatcherErrorRevisionSource)
		w.invalidateChange(message.policyChange())
		return
	}
	w.observeLag(ctx, databaseLatest, localApplied)
	if databaseLatest > localApplied {
		logger.Warn(ctx, "rbac policy revision mismatch detected", zap.Int64("database_latest_policy_revision", databaseLatest), zap.Int64("local_applied_policy_revision", localApplied), zap.Int64("hint_revision", message.PolicyRevision), zap.String("source", permissionapplication.MetricsSourceWatcherPubSub), zap.String("reason", permissionapplication.MetricsReasonRevisionMismatch))
		w.metrics.WatcherVersionMismatch(ctx, permissionapplication.MetricsSourceWatcherPubSub, permissionapplication.MetricsReasonRevisionMismatch)
	}
	if err := w.ObserveTargetRevision(ctx, databaseLatest, message.policyChange(), message.InstanceID, permissionapplication.MetricsSourceWatcherPubSub); err != nil {
		if ctx.Err() != nil {
			return
		}
		w.markReconcileFailure(ctx, permissionapplication.PolicyWatcherErrorReload)
		return
	}
	w.markReconcileSuccess()
}

func (w *Watcher) run(ctx context.Context, done chan struct{}) {
	// 消息与周期校准在同一循环串行执行，避免授权投影副作用乱序。
	messages := w.source.Messages()
	ticker := time.NewTicker(w.settings.CheckInterval)
	defer ticker.Stop()
	defer func() {
		w.mu.Lock()
		if w.done == done {
			w.cancel = nil
		}
		w.status.Running = false
		w.status.ReconcileErrorCategory = permissionapplication.PolicyWatcherErrorNone
		w.mu.Unlock()
		close(done)
	}()

	w.CheckVersion(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case message, ok := <-messages:
			if !ok {
				// 订阅 source 停止后禁用该 select 分支，数据库补偿仍由 ticker 继续。
				messages = nil
				continue
			}
			w.HandlePayload(ctx, message.Payload)
		case <-ticker.C:
			w.CheckVersion(ctx)
		}
	}
}

// ObserveTargetRevision 处理数据库 revision 目标，并始终保留消息要求的缓存失效副作用。
func (w *Watcher) ObserveTargetRevision(ctx context.Context, targetRevision int64, change permissionapplication.PolicyChange, instanceID string, source string) error {
	localApplied := w.engine.AppliedRevision()
	hasRevisionGap := targetRevision > localApplied
	reason := change.ReasonText()
	status := w.engine.ProjectionStatus()
	requiresReload := change.RequiresReload()
	if requiresReload || targetRevision > localApplied || !status.Ready() {
		var (
			appliedRevision int64
			err             error
		)
		if requiresReload {
			appliedRevision, err = w.engine.RefreshToRevision(ctx, targetRevision)
		} else {
			appliedRevision, err = w.engine.ReloadToRevision(ctx, targetRevision)
		}
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			w.metrics.WatcherReloadFailed(ctx, source, permissionapplication.MetricsReasonReloadFailed)
			w.observeLag(ctx, targetRevision, appliedRevision)
			logger.Error(ctx, "rbac policy projection reload failed", logger.StackTrace(zap.Int64("target_revision", targetRevision), zap.Int64("local_applied_policy_revision", appliedRevision), zap.String("instance_id", instanceID), zap.String("source", source), zap.String("reason", reason), zap.Error(err))...)
			return err
		}
		status = w.engine.ProjectionStatus()
		if !status.Ready() || status.AppliedRevision < targetRevision {
			w.metrics.WatcherReloadFailed(ctx, source, permissionapplication.MetricsReasonReloadFailed)
			w.observeLag(ctx, targetRevision, status.AppliedRevision)
			logger.Error(ctx, "rbac policy projection reload incomplete", zap.Int64("target_revision", targetRevision), zap.Int64("local_applied_policy_revision", status.AppliedRevision), zap.String("instance_id", instanceID), zap.String("source", source), zap.String("reason", reason))
			return errors.New("rbac policy projection reload incomplete")
		}
		w.metrics.WatcherReloadSucceeded(ctx, source)
		w.observeLag(ctx, targetRevision, status.AppliedRevision)
	}
	if hasRevisionGap {
		// 跨过 revision 表示期间可能漏收了任意用户的绑定事件，精确失效不足以恢复这些缓存。
		w.engine.InvalidateAllUserRoles()
	} else {
		w.invalidateChange(change)
	}
	appliedRevision := w.engine.AppliedRevision()
	logger.Info(ctx, "rbac policy projection synchronized", zap.Int64("target_revision", targetRevision), zap.Int64("local_applied_policy_revision", appliedRevision), zap.Int64("previous_applied_policy_revision", localApplied), zap.String("instance_id", instanceID), zap.String("source", source), zap.String("reason", reason))
	return nil
}

func (w *Watcher) invalidateChange(change permissionapplication.PolicyChange) {
	if change.UserID != uuid.Nil {
		w.engine.InvalidateUserRole(change.UserID)
	} else if change.RequiresReload() {
		w.engine.InvalidateAllUserRoles()
	}
}

func (w *Watcher) latestRevision(ctx context.Context, source string, hintRevision int64) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if w.revisionSource == nil {
		err := errors.New("rbac policy revision source is required")
		w.metrics.WatcherCheckFailed(ctx, source, permissionapplication.MetricsReasonRevisionStoreUnavailable)
		logger.Error(ctx, "rbac policy revision query failed", logger.StackTrace(zap.Int64("hint_revision", hintRevision), zap.Int64("local_applied_policy_revision", w.engine.AppliedRevision()), zap.String("source", source), zap.String("reason", permissionapplication.MetricsReasonRevisionStoreUnavailable), zap.Error(err))...)
		return 0, err
	}
	revision, err := w.revisionSource.LatestPolicyRevision(ctx)
	if err != nil {
		if ctx.Err() != nil {
			return 0, ctx.Err()
		}
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

func (w *Watcher) markReconcileSuccess() {
	w.mu.Lock()
	w.status.ReconcileErrorCategory = permissionapplication.PolicyWatcherErrorNone
	w.status.LastReconcileSuccessAt = time.Now()
	w.mu.Unlock()
}

func (w *Watcher) markReconcileFailure(ctx context.Context, category permissionapplication.PolicyWatcherErrorCategory) {
	if ctx != nil && ctx.Err() != nil {
		return
	}
	w.mu.Lock()
	w.status.ReconcileErrorCategory = category
	w.lastReconcileFailureAt = time.Now()
	w.mu.Unlock()
}

func mapSubscriptionState(state redispubsub.State) permissionapplication.PolicyWatcherSubscriptionState {
	switch state {
	case redispubsub.StateStarting:
		return permissionapplication.PolicyWatcherSubscriptionStarting
	case redispubsub.StateConnected:
		return permissionapplication.PolicyWatcherSubscriptionConnected
	case redispubsub.StateReconnecting:
		return permissionapplication.PolicyWatcherSubscriptionReconnecting
	case redispubsub.StateCreated, redispubsub.StateStopping, redispubsub.StateStopped:
		return permissionapplication.PolicyWatcherSubscriptionStopped
	default:
		return permissionapplication.PolicyWatcherSubscriptionStopped
	}
}

func mapSubscriptionError(category redispubsub.ErrorCategory) permissionapplication.PolicyWatcherErrorCategory {
	switch category {
	case redispubsub.ErrorNone:
		return permissionapplication.PolicyWatcherErrorNone
	case redispubsub.ErrorSubscribe:
		return permissionapplication.PolicyWatcherErrorSubscribe
	case redispubsub.ErrorReceive:
		return permissionapplication.PolicyWatcherErrorReceive
	case redispubsub.ErrorProtocol:
		return permissionapplication.PolicyWatcherErrorProtocol
	default:
		return permissionapplication.PolicyWatcherErrorProtocol
	}
}
