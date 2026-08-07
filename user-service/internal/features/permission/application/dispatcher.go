package application

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/aegiscore/common/runtime/logger"
)

type systemClock struct{}

type systemTicker struct{ ticker *time.Ticker }

func (systemClock) Now() time.Time { return time.Now() }
func (systemClock) NewTicker(interval time.Duration) Ticker {
	return systemTicker{ticker: time.NewTicker(interval)}
}
func (t systemTicker) C() <-chan time.Time { return t.ticker.C }
func (t systemTicker) Stop()               { t.ticker.Stop() }

// Dispatcher 轮询并可靠投递 RBAC policy outbox event。
type Dispatcher struct {
	store     OutboxStore
	publisher PolicyRevisionPublisher
	settings  DispatcherSettings
	clock     Clock
	log       *zap.Logger
	metrics   Metrics

	mu                     sync.RWMutex
	cancel                 context.CancelFunc
	done                   chan struct{}
	running                bool
	lastSuccessfulDispatch *time.Time
	lastErrorCategory      string
}

var _ OutboxDispatcherRunner = (*Dispatcher)(nil)
var _ OutboxDispatcherStatus = (*Dispatcher)(nil)

// NewDispatcher 只构造 dispatcher，不启动 goroutine 或访问外部资源。
func NewDispatcher(store OutboxStore, publisher PolicyRevisionPublisher, settings DispatcherSettings, clock Clock, log *zap.Logger, metrics Metrics) (*Dispatcher, error) {
	if clock == nil {
		clock = systemClock{}
	}
	if log == nil {
		log = zap.NewNop()
	}
	if metrics == nil {
		metrics = NopMetrics()
	}
	if err := validateDispatcherDependencies(store, publisher, clock); err != nil {
		return nil, err
	}
	if err := settings.Validate(); err != nil {
		return nil, err
	}
	return &Dispatcher{store: store, publisher: publisher, settings: settings, clock: clock, log: log, metrics: metrics}, nil
}

// Start 幂等启动后台轮询循环。
func (d *Dispatcher) Start(ctx context.Context) error {
	if ctx == nil {
		return errors.New("dispatcher start context is required")
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.done != nil {
		return nil
	}
	runCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	d.cancel = cancel
	d.done = make(chan struct{})
	d.running = true
	d.lastErrorCategory = DispatcherErrorNone
	d.metrics.DispatcherRunningObserved(runCtx, true)
	logger.FromContext(d.logContext(runCtx)).Info("rbac policy outbox dispatcher started")
	done := d.done
	go d.run(runCtx, done)
	return nil
}

// Stop 取消轮询并在调用方期限内等待当前投递结束。
func (d *Dispatcher) Stop(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	d.mu.RLock()
	cancel, done := d.cancel, d.done
	d.mu.RUnlock()
	if cancel == nil || done == nil {
		return nil
	}
	cancel()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// DispatchOnce claim 并逐条处理一个 batch；单条失败不会阻塞同 batch 后续事件。
func (d *Dispatcher) DispatchOnce(ctx context.Context) error {
	now := d.clock.Now()
	claims, err := d.store.Claim(ctx, now, d.settings.BatchSize, d.settings.ClaimTimeout)
	if err != nil {
		d.recordError(DispatcherErrorClaim)
		d.metrics.DispatcherOperationObserved(ctx, MetricsOperationDispatcherClaim, MetricsResultFailure, MetricsReasonClaimFailed, MetricsKindNone)
		logger.Error(d.logContext(ctx), "rbac policy outbox claim failed", logger.StackTrace(zap.String("error_category", DispatcherErrorClaim))...)
		return fmt.Errorf("claim rbac policy outbox events: %w", err)
	}
	d.metrics.DispatcherOperationObserved(ctx, MetricsOperationDispatcherClaim, MetricsResultSuccess, MetricsReasonNone, MetricsKindNone)
	d.log.Debug("rbac policy outbox events claimed", zap.Int("claimed_count", len(claims)))
	var dispatchErr error
	for _, claim := range claims {
		if err := ctx.Err(); err != nil {
			return errors.Join(dispatchErr, err)
		}
		if err := d.dispatchClaim(ctx, claim); err != nil {
			dispatchErr = errors.Join(dispatchErr, err)
		}
	}
	if _, err := d.Status(ctx); err != nil {
		dispatchErr = errors.Join(dispatchErr, err)
	}
	if dispatchErr == nil && len(claims) > 0 {
		d.clearError()
	}
	return dispatchErr
}

// Status 合并进程内运行状态与 store 的只读 backlog 快照。
func (d *Dispatcher) Status(ctx context.Context) (DispatcherStatus, error) {
	now := d.clock.Now()
	backlog, err := d.store.Backlog(ctx, now)
	d.mu.RLock()
	status := DispatcherStatus{
		Running:                d.running,
		LastSuccessfulDispatch: cloneTime(d.lastSuccessfulDispatch),
		LastErrorCategory:      d.lastErrorCategory,
	}
	d.mu.RUnlock()
	if err != nil {
		d.recordError(DispatcherErrorBacklog)
		logger.Error(d.logContext(ctx), "rbac policy outbox backlog read failed", logger.StackTrace(zap.String("error_category", DispatcherErrorBacklog))...)
		status.LastErrorCategory = DispatcherErrorBacklog
		return status, fmt.Errorf("read rbac policy outbox backlog: %w", err)
	}
	status.DueCount = backlog.DueCount
	if backlog.OldestCreatedAt != nil && now.After(*backlog.OldestCreatedAt) {
		status.OldestUnfinishedAge = now.Sub(*backlog.OldestCreatedAt)
	}
	d.metrics.DispatcherBacklogObserved(ctx, status.DueCount, status.OldestUnfinishedAge)
	return status, nil
}

func (d *Dispatcher) dispatchClaim(ctx context.Context, claim OutboxClaim) error {
	if err := d.publisher.PublishPolicyRevision(ctx, claim.Event); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		failedAt := d.clock.Now()
		d.metrics.DispatcherOperationObserved(ctx, MetricsOperationDispatcherPublish, MetricsResultFailure, MetricsReasonPublishFailed, claim.Event.Kind)
		logger.Error(d.logContext(ctx), "rbac policy outbox publish failed", logger.StackTrace(zap.Int64("policy_revision", claim.Event.Revision), zap.Int("attempt_count", claim.AttemptCount+1), zap.String("error_category", DispatcherErrorPublish))...)
		nextAttemptAt := failedAt.Add(d.settings.RetryBackoff(claim.AttemptCount + 1))
		updated, failErr := d.store.Fail(ctx, claim.Event.EventID, claim.ClaimToken, failedAt, nextAttemptAt, DispatcherErrorPublish)
		if failErr != nil {
			d.recordError(DispatcherErrorFailureRecord)
			d.metrics.DispatcherOperationObserved(ctx, MetricsOperationDispatcherFailure, MetricsResultFailure, MetricsReasonFailureRecordFailed, claim.Event.Kind)
			d.metrics.DispatcherOperationObserved(ctx, MetricsOperationDispatcherRetry, MetricsResultFailure, MetricsReasonFailureRecordFailed, claim.Event.Kind)
			logger.Error(d.logContext(ctx), "rbac policy outbox failure record failed", logger.StackTrace(zap.Int64("policy_revision", claim.Event.Revision), zap.Int("attempt_count", claim.AttemptCount+1), zap.String("error_category", DispatcherErrorFailureRecord))...)
			return errors.Join(fmt.Errorf("publish rbac policy revision %d: %w", claim.Event.Revision, err), fmt.Errorf("record rbac policy outbox failure: %w", failErr))
		}
		if !updated {
			d.recordError(DispatcherErrorClaimLost)
			d.metrics.DispatcherOperationObserved(ctx, MetricsOperationDispatcherFailure, MetricsResultFailure, MetricsReasonClaimLost, claim.Event.Kind)
			d.metrics.DispatcherOperationObserved(ctx, MetricsOperationDispatcherRetry, MetricsResultFailure, MetricsReasonClaimLost, claim.Event.Kind)
			logger.Warn(d.logContext(ctx), "rbac policy outbox failure claim lost", zap.Int64("policy_revision", claim.Event.Revision), zap.Int("attempt_count", claim.AttemptCount+1), zap.String("error_category", DispatcherErrorClaimLost))
			return errors.Join(fmt.Errorf("publish rbac policy revision %d: %w", claim.Event.Revision, err), claimLostError(claim.Event.EventID))
		}
		d.metrics.DispatcherOperationObserved(ctx, MetricsOperationDispatcherFailure, MetricsResultSuccess, MetricsReasonNone, claim.Event.Kind)
		d.metrics.DispatcherOperationObserved(ctx, MetricsOperationDispatcherRetry, MetricsResultSuccess, MetricsReasonPublishFailed, claim.Event.Kind)
		logger.Warn(d.logContext(ctx), "rbac policy outbox retry scheduled", zap.Int64("policy_revision", claim.Event.Revision), zap.Int("attempt_count", claim.AttemptCount+1), zap.String("kind", claim.Event.Kind), zap.String("reason", MetricsReasonPublishFailed), zap.Duration("retry_delay", nextAttemptAt.Sub(failedAt)))
		d.recordError(DispatcherErrorPublish)
		return fmt.Errorf("publish rbac policy revision %d: %w", claim.Event.Revision, err)
	}
	d.metrics.DispatcherOperationObserved(ctx, MetricsOperationDispatcherPublish, MetricsResultSuccess, MetricsReasonNone, claim.Event.Kind)

	// publish 成功而 Ack 失败时事件会在 lease 过期后重投，因此整体是至少一次投递；revision-aware 消费方必须能重复处理同一事件。
	deliveredAt := d.clock.Now()
	updated, err := d.store.Ack(ctx, claim.Event.EventID, claim.ClaimToken, deliveredAt)
	if err != nil {
		d.recordError(DispatcherErrorAck)
		d.metrics.DispatcherOperationObserved(ctx, MetricsOperationDispatcherAck, MetricsResultFailure, MetricsReasonAckFailed, claim.Event.Kind)
		logger.Error(d.logContext(ctx), "rbac policy outbox acknowledgement failed", logger.StackTrace(zap.Int64("policy_revision", claim.Event.Revision), zap.Int("attempt_count", claim.AttemptCount+1), zap.String("error_category", DispatcherErrorAck))...)
		return fmt.Errorf("ack rbac policy outbox event %s: %w", claim.Event.EventID.String(), err)
	}
	if !updated {
		d.recordError(DispatcherErrorClaimLost)
		d.metrics.DispatcherOperationObserved(ctx, MetricsOperationDispatcherAck, MetricsResultFailure, MetricsReasonClaimLost, claim.Event.Kind)
		logger.Warn(d.logContext(ctx), "rbac policy outbox acknowledgement claim lost", zap.Int64("policy_revision", claim.Event.Revision), zap.Int("attempt_count", claim.AttemptCount+1), zap.String("error_category", DispatcherErrorClaimLost))
		return claimLostError(claim.Event.EventID)
	}
	d.metrics.DispatcherOperationObserved(ctx, MetricsOperationDispatcherAck, MetricsResultSuccess, MetricsReasonNone, claim.Event.Kind)
	d.recordSuccess(deliveredAt)
	logger.Info(d.logContext(ctx), "rbac policy outbox event delivered", zap.Int64("policy_revision", claim.Event.Revision), zap.Int("attempt_count", claim.AttemptCount+1), zap.String("kind", claim.Event.Kind), zap.String("reason", claim.Event.Reason))
	return nil
}

func (d *Dispatcher) run(ctx context.Context, done chan struct{}) {
	var ticker Ticker
	defer func() {
		if recovered := recover(); recovered != nil {
			d.recordError(DispatcherErrorUnexpectedExit)
			d.log.Error("rbac policy outbox dispatcher exited unexpectedly",
				zap.String("error_category", DispatcherErrorUnexpectedExit),
			)
		}
		if ticker != nil {
			ticker.Stop()
		}
		d.mu.Lock()
		// 仅在字段仍指向本轮句柄时清理，使生命周期状态更新与传入的 done 保持绑定。
		if d.done == done {
			d.cancel = nil
			d.done = nil
		}
		d.running = false
		d.mu.Unlock()
		d.metrics.DispatcherRunningObserved(ctx, false)
		logger.FromContext(d.logContext(ctx)).Info("rbac policy outbox dispatcher stopped")
		close(done)
	}()
	ticker = d.clock.NewTicker(d.settings.PollInterval)

	for {
		if err := d.DispatchOnce(ctx); err != nil && ctx.Err() != nil {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C():
		}
	}
}

func (d *Dispatcher) logContext(ctx context.Context) context.Context {
	return logger.ToContext(ctx, d.log)
}

func (d *Dispatcher) recordSuccess(at time.Time) {
	d.mu.Lock()
	d.lastSuccessfulDispatch = cloneTime(&at)
	d.mu.Unlock()
}

func (d *Dispatcher) clearError() {
	d.mu.Lock()
	d.lastErrorCategory = DispatcherErrorNone
	d.mu.Unlock()
}

func (d *Dispatcher) recordError(category string) {
	d.mu.Lock()
	d.lastErrorCategory = category
	d.mu.Unlock()
}

func cloneTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}
