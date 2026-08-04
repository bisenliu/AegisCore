package redis

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"sync"
	"time"

	"github.com/google/uuid"
	rediscmd "github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/aegiscore/common/runtime/logger"
	permissionapplication "github.com/aegiscore/user-service/internal/features/permission/application"
)

const (
	defaultCheckInterval    = 15 * time.Second
	defaultSubscribeTimeout = 5 * time.Second
	defaultBackoffInitial   = 250 * time.Millisecond
	defaultBackoffMax       = 30 * time.Second
	policyPayloadBuffer     = 64
)

// WatcherSettings 控制 watcher 的数据库校准、订阅确认和重连节奏。
type WatcherSettings struct {
	CheckInterval    time.Duration
	SubscribeTimeout time.Duration
	BackoffInitial   time.Duration
	BackoffMax       time.Duration
}

// WatcherParams 包含 RBAC policy watcher 所需依赖。
type WatcherParams struct {
	Store          *Store
	RevisionSource permissionapplication.LatestPolicyRevisionSource
	Engine         permissionapplication.PolicyReloadEngine
	Log            *zap.Logger
	Metrics        permissionapplication.Metrics
	Settings       WatcherSettings
}

// Watcher 监听 RBAC policy 分布式 revision 并执行补偿 reload。
type Watcher struct {
	store          policySubscriptionStore
	revisionSource permissionapplication.LatestPolicyRevisionSource
	engine         permissionapplication.PolicyReloadEngine
	log            *zap.Logger
	metrics        permissionapplication.Metrics
	settings       WatcherSettings

	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
	active *subscriptionAttempt
	status permissionapplication.PolicyWatcherStatusSnapshot
}

type subscriptionAttempt struct {
	subscriber policySubscriber
	closeOnce  sync.Once
}

func (a *subscriptionAttempt) close() {
	if a == nil || a.subscriber == nil {
		return
	}
	a.closeOnce.Do(func() { _ = a.subscriber.Close() })
}

// NewWatcher 只构造 RBAC policy watcher；调用方负责显式调用 Start 和 Stop。
func NewWatcher(params WatcherParams) *Watcher {
	return newWatcher(params.Store, params.RevisionSource, params.Engine, params.Log, params.Settings, params.Metrics)
}

func newWatcherWithMetrics(store policySubscriptionStore, revisionSource permissionapplication.LatestPolicyRevisionSource, engine permissionapplication.PolicyReloadEngine, log *zap.Logger, checkInterval time.Duration, metrics permissionapplication.Metrics) *Watcher {
	return newWatcher(store, revisionSource, engine, log, WatcherSettings{
		CheckInterval: checkInterval, SubscribeTimeout: defaultSubscribeTimeout,
		BackoffInitial: defaultBackoffInitial, BackoffMax: defaultBackoffMax,
	}, metrics)
}

func newWatcher(store policySubscriptionStore, revisionSource permissionapplication.LatestPolicyRevisionSource, engine permissionapplication.PolicyReloadEngine, log *zap.Logger, settings WatcherSettings, metrics permissionapplication.Metrics) *Watcher {
	settings.applyDefaults()
	if metrics == nil {
		metrics = permissionapplication.NopMetrics()
	}
	return &Watcher{store: store, revisionSource: revisionSource, engine: engine, log: log, metrics: metrics, settings: settings,
		status: permissionapplication.PolicyWatcherStatusSnapshot{
			SubscriptionState:         permissionapplication.PolicyWatcherSubscriptionStopped,
			SubscriptionErrorCategory: permissionapplication.PolicyWatcherErrorNone,
			ReconcileErrorCategory:    permissionapplication.PolicyWatcherErrorNone,
		}}
}

func (s *WatcherSettings) applyDefaults() {
	if s.CheckInterval <= 0 {
		s.CheckInterval = defaultCheckInterval
	}
	if s.SubscribeTimeout <= 0 {
		s.SubscribeTimeout = defaultSubscribeTimeout
	}
	if s.BackoffInitial <= 0 {
		s.BackoffInitial = defaultBackoffInitial
	}
	if s.BackoffMax < s.BackoffInitial {
		s.BackoffMax = defaultBackoffMax
	}
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
	lastFailureAt := w.status.LastFailureAt
	reconnectAttempts := w.status.ReconnectAttempts
	w.status = permissionapplication.PolicyWatcherStatusSnapshot{
		Running:                   true,
		SubscriptionState:         permissionapplication.PolicyWatcherSubscriptionStarting,
		LastFailureAt:             lastFailureAt,
		SubscriptionErrorCategory: permissionapplication.PolicyWatcherErrorNone,
		ReconcileErrorCategory:    permissionapplication.PolicyWatcherErrorNone,
		ReconnectAttempts:         reconnectAttempts,
	}
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
	w.closeActiveSubscription()
	select {
	case <-done:
		w.mu.Lock()
		if w.done == done {
			w.cancel = nil
			w.done = nil
		}
		w.mu.Unlock()
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Status 返回 watcher 当前结构化状态的只读快照。
func (w *Watcher) Status() permissionapplication.PolicyWatcherStatusSnapshot {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.status
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
	payloads := make(chan string, policyPayloadBuffer)
	subscriptionDone := make(chan struct{})
	go func() {
		defer close(subscriptionDone)
		w.runSubscriptionSupervisor(ctx, payloads)
	}()
	ticker := time.NewTicker(w.settings.CheckInterval)
	defer ticker.Stop()
	defer func() {
		w.closeActiveSubscription()
		<-subscriptionDone
		w.mu.Lock()
		if w.done == done {
			w.cancel = nil
			w.done = nil
		}
		w.active = nil
		w.status.Running = false
		w.status.SubscriptionState = permissionapplication.PolicyWatcherSubscriptionStopped
		w.status.SubscriptionErrorCategory = permissionapplication.PolicyWatcherErrorNone
		w.status.ReconcileErrorCategory = permissionapplication.PolicyWatcherErrorNone
		w.mu.Unlock()
		close(done)
	}()

	w.CheckVersion(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case payload := <-payloads:
			w.HandlePayload(ctx, payload)
		case <-ticker.C:
			w.CheckVersion(ctx)
		}
	}
}

func (w *Watcher) runSubscriptionSupervisor(ctx context.Context, payloads chan<- string) {
	backoff := w.settings.BackoffInitial
	for ctx.Err() == nil {
		attempt := &subscriptionAttempt{subscriber: w.store.Subscribe(ctx)}
		w.setActiveSubscription(attempt)
		if ctx.Err() != nil {
			attempt.close()
			w.clearActiveSubscription(attempt)
			return
		}

		confirmCtx, cancel := context.WithTimeout(ctx, w.settings.SubscribeTimeout)
		message, err := attempt.subscriber.Receive(confirmCtx)
		cancel()
		if err == nil {
			if _, ok := message.(*rediscmd.Subscription); !ok {
				err = fmt.Errorf("unexpected subscription confirmation: %T", message)
			}
		}
		if err != nil {
			attempt.close()
			w.clearActiveSubscription(attempt)
			if ctx.Err() != nil || errors.Is(err, context.Canceled) {
				return
			}
			w.markSubscriptionFailure(permissionapplication.PolicyWatcherErrorSubscribe)
			logger.Error(ctx, "rbac policy refresh subscribe failed", logger.StackTrace(zap.Error(err))...)
			if !waitForRetry(ctx, jitteredBackoff(backoff)) {
				return
			}
			backoff = nextBackoff(backoff, w.settings.BackoffMax)
			continue
		}

		w.markSubscriptionSuccess()
		backoff = w.settings.BackoffInitial
		err = w.receiveSubscription(ctx, attempt, payloads)
		attempt.close()
		w.clearActiveSubscription(attempt)
		if ctx.Err() != nil || errors.Is(err, context.Canceled) {
			return
		}
		w.markSubscriptionFailure(permissionapplication.PolicyWatcherErrorReceive)
		logger.Error(ctx, "rbac policy refresh receive failed", logger.StackTrace(zap.Error(err))...)
		if !waitForRetry(ctx, jitteredBackoff(backoff)) {
			return
		}
		backoff = nextBackoff(backoff, w.settings.BackoffMax)
	}
}

func (w *Watcher) receiveSubscription(ctx context.Context, attempt *subscriptionAttempt, payloads chan<- string) error {
	for {
		message, err := attempt.subscriber.Receive(ctx)
		if err != nil {
			return err
		}
		switch value := message.(type) {
		case *rediscmd.Message:
			select {
			case payloads <- value.Payload:
			case <-ctx.Done():
				return ctx.Err()
			}
		case *rediscmd.Subscription:
			w.markSubscriptionSuccess()
		case *rediscmd.Pong:
		default:
			return fmt.Errorf("unexpected pubsub message: %T", message)
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

func (w *Watcher) markSubscriptionSuccess() {
	w.mu.Lock()
	w.status.SubscriptionState = permissionapplication.PolicyWatcherSubscriptionConnected
	w.status.SubscriptionErrorCategory = permissionapplication.PolicyWatcherErrorNone
	w.status.LastSubscriptionSuccessAt = time.Now()
	w.mu.Unlock()
}

func (w *Watcher) markSubscriptionFailure(category permissionapplication.PolicyWatcherErrorCategory) {
	w.mu.Lock()
	w.status.SubscriptionState = permissionapplication.PolicyWatcherSubscriptionReconnecting
	w.status.SubscriptionErrorCategory = category
	w.status.LastFailureAt = time.Now()
	w.status.ReconnectAttempts++
	w.mu.Unlock()
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
	w.status.LastFailureAt = time.Now()
	w.mu.Unlock()
}

func (w *Watcher) setActiveSubscription(attempt *subscriptionAttempt) {
	w.mu.Lock()
	w.active = attempt
	w.mu.Unlock()
}

func (w *Watcher) clearActiveSubscription(attempt *subscriptionAttempt) {
	w.mu.Lock()
	if w.active == attempt {
		w.active = nil
	}
	w.mu.Unlock()
}

func (w *Watcher) closeActiveSubscription() {
	w.mu.Lock()
	attempt := w.active
	w.mu.Unlock()
	attempt.close()
}

func waitForRetry(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func nextBackoff(current time.Duration, maximum time.Duration) time.Duration {
	if current >= maximum || current > maximum/2 {
		return maximum
	}
	return current * 2
}

func jitteredBackoff(delay time.Duration) time.Duration {
	half := delay / 2
	if half <= 0 {
		return delay
	}
	return half + time.Duration(rand.Int64N(int64(delay-half)+1))
}
