package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/aegiscore/common/runtime/logger"
)

// PolicyReloadEngine 定义 RBAC policy revision-aware reload 与只读投影状态能力。
type PolicyReloadEngine interface {
	ObserveTargetRevision(targetRevision int64)
	ReloadToRevision(ctx context.Context, targetRevision int64) (int64, error)
	RefreshToRevision(ctx context.Context, targetRevision int64) (int64, error)
	ProjectionStatus() PolicyProjectionStatus
	AppliedRevision() int64
	InvalidateUserRole(userID uuid.UUID)
	InvalidateAllUserRoles()
}

// PolicyChangeNotifier 定义已提交 RBAC policy 变更后的刷新通知端口。
type PolicyChangeNotifier interface {
	NotifyPolicyChanged(ctx context.Context, revision int64, change PolicyChange) error
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

// PolicyRefreshCoordinator 负责本实例 policy reload 和缓存失效编排。
type PolicyRefreshCoordinator struct {
	engine  PolicyReloadEngine
	log     *zap.Logger
	metrics Metrics
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
func NewPolicyRefreshCoordinator(engine PolicyReloadEngine, log *zap.Logger, metrics Metrics) *PolicyRefreshCoordinator {
	if metrics == nil {
		metrics = NopMetrics()
	}
	return &PolicyRefreshCoordinator{engine: engine, log: log, metrics: metrics}
}

// NotifyPolicyChanged 使用已提交数据库 revision 立即刷新本实例。
// 跨实例发布只由 outbox dispatcher 承担，Redis 故障不会改变已提交 mutation 的本地同步结果。
func (c *PolicyRefreshCoordinator) NotifyPolicyChanged(ctx context.Context, revision int64, change PolicyChange) error {
	if c == nil {
		return errors.New("rbac policy refresh coordinator is required")
	}
	if c.log != nil {
		ctx = logger.ToContext(ctx, c.log)
	}
	return c.ObserveTargetRevision(ctx, revision, change)
}

// ObserveTargetRevision 以数据库 revision 驱动本地 policy 投影和用户角色缓存失效。
func (c *PolicyRefreshCoordinator) ObserveTargetRevision(ctx context.Context, targetRevision int64, change PolicyChange) error {
	reason := change.ReasonText()
	appliedRevision, err := c.engine.ReloadToRevision(ctx, targetRevision)
	if err != nil {
		c.metrics.PolicyReloadFailed(ctx, MetricsSourceLocalChange, MetricsReasonReloadFailed)
		logger.Error(ctx, "rbac policy local refresh failed", logger.StackTrace(zap.Int64("target_revision", targetRevision), zap.Int64("applied_revision", appliedRevision), zap.String("reason", reason), zap.Error(err))...)
		return fmt.Errorf("reload rbac policy after %s: %w", reason, err)
	}
	status := c.engine.ProjectionStatus()
	if !status.Ready() || status.AppliedRevision < targetRevision {
		err = fmt.Errorf("rbac policy projection did not reach target revision: target=%d applied=%d", targetRevision, status.AppliedRevision)
		c.metrics.PolicyReloadFailed(ctx, MetricsSourceLocalChange, MetricsReasonReloadFailed)
		logger.Error(ctx, "rbac policy local refresh incomplete", logger.StackTrace(zap.Int64("target_revision", targetRevision), zap.Int64("applied_revision", status.AppliedRevision), zap.String("reason", reason), zap.Error(err))...)
		return err
	}
	if change.RequiresReload() {
		c.engine.InvalidateAllUserRoles()
	} else if change.UserID != uuid.Nil {
		c.engine.InvalidateUserRole(change.UserID)
	} else {
		c.engine.InvalidateAllUserRoles()
	}
	c.metrics.PolicyReloadSucceeded(ctx, MetricsSourceLocalChange)
	logger.Info(ctx, "rbac policy local refresh succeeded", zap.Int64("target_revision", targetRevision), zap.Int64("applied_revision", c.engine.AppliedRevision()), zap.String("reason", reason))
	return nil
}
