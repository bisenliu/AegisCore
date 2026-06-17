package application

import (
	"context"

	"go.uber.org/zap"

	"github.com/aegiscore/common/runtime/logger"
)

// PolicyReloadEngine 定义 RBAC policy 全量 reload 能力。
type PolicyReloadEngine interface {
	Reload(ctx context.Context) error
}

// PolicyVersionPublisher 定义 RBAC policy 分布式版本通知端口。
type PolicyVersionPublisher interface {
	PublishPolicyChanged(ctx context.Context, reason string) (int64, error)
}

// PolicyVersionTracker 定义本实例已应用 RBAC policy 版本跟踪端口。
type PolicyVersionTracker interface {
	MarkApplied(version int64)
	Applied() int64
}

// PolicyChangeNotifier 定义 RBAC policy 变更后的刷新通知端口。
type PolicyChangeNotifier interface {
	NotifyPolicyChanged(ctx context.Context, reason string)
}

// PolicyRefreshCoordinator 负责本实例 policy reload 和分布式版本通知编排。
type PolicyRefreshCoordinator struct {
	engine    PolicyReloadEngine
	publisher PolicyVersionPublisher
	tracker   PolicyVersionTracker
	log       *zap.Logger
	metrics   Metrics
}

// NewPolicyRefreshCoordinator 构造 RBAC policy 刷新编排器。
func NewPolicyRefreshCoordinator(engine PolicyReloadEngine, publisher PolicyVersionPublisher, tracker PolicyVersionTracker, log *zap.Logger, metrics Metrics) *PolicyRefreshCoordinator {
	if metrics == nil {
		metrics = NopMetrics()
	}
	return &PolicyRefreshCoordinator{engine: engine, publisher: publisher, tracker: tracker, log: log, metrics: metrics}
}

// NotifyPolicyChanged 在 RBAC 数据变更成功后刷新本实例并通知其他实例。
func (c *PolicyRefreshCoordinator) NotifyPolicyChanged(ctx context.Context, reason string) {
	if c == nil {
		return
	}
	if err := c.engine.Reload(ctx); err != nil {
		c.metrics.PolicyReloadFailed(ctx, MetricsSourceLocalChange, MetricsReasonReloadFailed)
		logger.Error(ctx, "rbac policy local refresh failed", logger.StackTrace(zap.String("reason", reason), zap.Error(err))...)
		return
	}
	c.metrics.PolicyReloadSucceeded(ctx, MetricsSourceLocalChange)
	version, err := c.publisher.PublishPolicyChanged(ctx, reason)
	if err != nil {
		c.metrics.PolicyPublishFailed(ctx, MetricsReasonPublishFailed)
		logger.Error(ctx, "rbac policy version publish failed", logger.StackTrace(zap.String("reason", reason), zap.Error(err))...)
		return
	}
	c.metrics.PolicyPublishSucceeded(ctx)
	c.tracker.MarkApplied(version)
	logger.Info(ctx, "rbac policy local refresh succeeded", zap.Int64("policy_version", version), zap.String("reason", reason))
}
