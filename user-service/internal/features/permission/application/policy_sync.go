package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/aegiscore/common/runtime/logger"
)

// PolicyReloadEngine 定义 RBAC policy 全量 reload 能力。
type PolicyReloadEngine interface {
	Reload(ctx context.Context) error
	InvalidateUserRole(userID uuid.UUID)
	InvalidateAllUserRoles()
}

// PolicyVersionPublisher 定义 RBAC policy 分布式版本通知端口。
type PolicyVersionPublisher interface {
	PublishPolicyChanged(ctx context.Context, change PolicyChange) (int64, error)
}

// PolicyVersionTracker 定义本实例已应用 RBAC policy 版本跟踪端口。
type PolicyVersionTracker interface {
	MarkApplied(version int64)
	Applied() int64
}

// PolicyChangeNotifier 定义 RBAC policy 变更后的刷新通知端口。
type PolicyChangeNotifier interface {
	NotifyPolicyChanged(ctx context.Context, change PolicyChange) error
}

// PolicyChangeKind 表示 RBAC policy 同步变更类别。
type PolicyChangeKind string

const (
	// PolicyChangeKindPolicy 表示角色权限策略需要全量重建。
	PolicyChangeKindPolicy PolicyChangeKind = "policy"
	// PolicyChangeKindUserRole 表示用户角色缓存需要按用户失效。
	PolicyChangeKindUserRole PolicyChangeKind = "user_role"
)

// PolicyChange 描述一次 RBAC policy 同步变更。
type PolicyChange struct {
	Kind         PolicyChangeKind
	Reason       string
	UserID       uuid.UUID
	RoleID       uuid.UUID
	PermissionID uuid.UUID
}

// PolicyRefreshCoordinator 负责本实例 policy reload 和分布式版本通知编排。
type PolicyRefreshCoordinator struct {
	engine    PolicyReloadEngine
	publisher PolicyVersionPublisher
	tracker   PolicyVersionTracker
	log       *zap.Logger
	metrics   Metrics
}

// NewPolicyReloadChange 构造需要重建角色权限策略的变更。
func NewPolicyReloadChange(reason string) PolicyChange {
	return PolicyChange{Kind: PolicyChangeKindPolicy, Reason: reason}
}

// NewUserRoleChange 构造指定用户角色缓存失效变更。
func NewUserRoleChange(reason string, userID uuid.UUID, roleID uuid.UUID) PolicyChange {
	return PolicyChange{Kind: PolicyChangeKindUserRole, Reason: reason, UserID: userID, RoleID: roleID}
}

// RequiresReload 返回变更是否需要重建 Casbin 角色权限 policy。
func (c PolicyChange) RequiresReload() bool {
	return c.Kind == "" || c.Kind == PolicyChangeKindPolicy
}

// ReasonText 返回用于日志和指标上下文的稳定原因。
func (c PolicyChange) ReasonText() string {
	if c.Reason != "" {
		return c.Reason
	}
	if c.Kind != "" {
		return string(c.Kind)
	}
	return "policy_changed"
}

// NewPolicyRefreshCoordinator 构造 RBAC policy 刷新编排器。
func NewPolicyRefreshCoordinator(engine PolicyReloadEngine, publisher PolicyVersionPublisher, tracker PolicyVersionTracker, log *zap.Logger, metrics Metrics) *PolicyRefreshCoordinator {
	if metrics == nil {
		metrics = NopMetrics()
	}
	return &PolicyRefreshCoordinator{engine: engine, publisher: publisher, tracker: tracker, log: log, metrics: metrics}
}

// NotifyPolicyChanged 在 RBAC 数据变更成功后刷新本实例并通知其他实例。
// 本实例先应用本地 reload 或缓存失效，再发布分布式版本；如果本地应用失败，即使发布成功也不能标记本版本已应用。
func (c *PolicyRefreshCoordinator) NotifyPolicyChanged(ctx context.Context, change PolicyChange) error {
	if c == nil {
		return errors.New("rbac policy refresh coordinator is required")
	}
	reason := change.ReasonText()
	localApplied := true
	var syncErr error
	if change.RequiresReload() {
		if err := c.engine.Reload(ctx); err != nil {
			localApplied = false
			c.metrics.PolicyReloadFailed(ctx, MetricsSourceLocalChange, MetricsReasonReloadFailed)
			logger.Error(ctx, "rbac policy local refresh failed", logger.StackTrace(zap.String("reason", reason), zap.Error(err))...)
			syncErr = errors.Join(syncErr, fmt.Errorf("reload rbac policy after %s: %w", reason, err))
		} else {
			c.engine.InvalidateAllUserRoles()
			c.metrics.PolicyReloadSucceeded(ctx, MetricsSourceLocalChange)
		}
	} else if change.UserID != uuid.Nil {
		c.engine.InvalidateUserRole(change.UserID)
	} else {
		c.engine.InvalidateAllUserRoles()
	}
	version, err := c.publisher.PublishPolicyChanged(ctx, change)
	if err != nil {
		c.metrics.PolicyPublishFailed(ctx, MetricsReasonPublishFailed)
		logger.Error(ctx, "rbac policy version publish failed", logger.StackTrace(zap.String("reason", reason), zap.Error(err))...)
		syncErr = errors.Join(syncErr, fmt.Errorf("publish rbac policy change after %s: %w", reason, err))
	} else {
		c.metrics.PolicyPublishSucceeded(ctx)
		if localApplied {
			c.tracker.MarkApplied(version)
		}
	}
	if syncErr != nil {
		return syncErr
	}
	logger.Info(ctx, "rbac policy local refresh succeeded", zap.Int64("policy_version", version), zap.String("reason", reason))
	return nil
}
