package casbin

import (
	"context"
	"errors"
	"fmt"
	"sync"

	casbinlib "github.com/casbin/casbin/v3"
	"github.com/google/uuid"

	commonmetrics "github.com/aegiscore/common/runtime/observability/metrics"
	commoncasbin "github.com/aegiscore/common/security/casbin"
	permissionapplication "github.com/aegiscore/user-service/internal/features/permission/application"
)

var errPolicyRevisionNotReached = errors.New("casbin policy target revision was not reached")

// Engine 使用内存 Casbin enforcer 执行权限判断。
type Engine struct {
	loader    Loader
	metrics   commonmetrics.ReloadMetrics
	userRoles UserRoleResolver

	mu              sync.RWMutex
	enforcer        *casbinlib.Enforcer
	appliedRevision int64
	targetRevision  int64
	initialized     bool
	reloadSucceeded bool
	lastErr         error
	flight          *reloadFlight
}

type reloadFlight struct {
	done    chan struct{}
	ctx     context.Context
	cancel  context.CancelFunc
	waiters int
	force   bool
}

// NewEngine 构造 Casbin Engine；调用方负责在启动边界显式执行 Initialize。
func NewEngine(loader Loader, metrics commonmetrics.ReloadMetrics, userRoles UserRoleResolver) *Engine {
	return &Engine{loader: loader, metrics: metrics, userRoles: userRoles}
}

// Enforce 基于已加载的内存 policy 判断用户是否允许访问指定路由模板和 HTTP 方法。
// 返回 false,nil 代表安全拒绝而不是系统错误，调用方应按无权限处理；只有上下文、角色解析或 Casbin 执行失败才返回 error。
func (e *Engine) Enforce(ctx context.Context, userID uuid.UUID, pathTemplate string, method string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	e.mu.RLock()
	ready := e.initialized && e.reloadSucceeded && e.lastErr == nil && e.appliedRevision >= e.targetRevision && e.enforcer != nil
	e.mu.RUnlock()
	if !ready {
		return false, nil
	}
	if e.userRoles == nil {
		return false, nil
	}
	roleIDs, err := e.userRoles.RolesForUser(ctx, userID)
	if err != nil {
		return false, err
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	enforcer := e.enforcer
	ready = e.initialized && e.reloadSucceeded && e.lastErr == nil && e.appliedRevision >= e.targetRevision
	if !ready || enforcer == nil {
		return false, nil
	}
	for _, roleID := range roleIDs {
		allowed, err := commoncasbin.Enforce(ctx, enforcer, commoncasbin.Request{Subject: roleSubject(roleID), Object: pathTemplate, Action: method})
		if err != nil || allowed {
			return allowed, err
		}
	}
	return false, nil
}

// ObserveTargetRevision 记录本实例已知的最高数据库 policy revision，但不改变 applied revision。
func (e *Engine) ObserveTargetRevision(targetRevision int64) {
	if targetRevision < 0 {
		return
	}
	e.mu.Lock()
	if targetRevision > e.targetRevision {
		e.targetRevision = targetRevision
	}
	e.mu.Unlock()
}

// ReloadToRevision 将 policy 投影推进到至少 targetRevision，并返回实际 applied revision。
func (e *Engine) ReloadToRevision(ctx context.Context, targetRevision int64) (int64, error) {
	return e.reloadToRevision(ctx, targetRevision, false)
}

// RefreshToRevision 强制从当前 PostgreSQL 快照刷新 policy，同时保持 target revision 门禁。
func (e *Engine) RefreshToRevision(ctx context.Context, targetRevision int64) (int64, error) {
	return e.reloadToRevision(ctx, targetRevision, true)
}

func (e *Engine) reloadToRevision(ctx context.Context, targetRevision int64, force bool) (int64, error) {
	if targetRevision < 0 {
		return e.failReload(fmt.Errorf("casbin policy target revision must not be negative: %d", targetRevision))
	}

	e.ObserveTargetRevision(targetRevision)
	e.mu.Lock()
	if err := ctx.Err(); err != nil {
		e.lastErr = err
		e.reloadSucceeded = false
		applied := e.appliedRevision
		e.metrics.ReloadFailed()
		e.metrics.SetLastStatus(false)
		e.mu.Unlock()
		return applied, err
	}
	if !force && e.projectionReadyForLocked(targetRevision) {
		applied := e.appliedRevision
		e.mu.Unlock()
		return applied, nil
	}
	flight := e.flight
	if flight == nil {
		flight = e.startFlightLocked(force)
	} else if force {
		flight.force = true
	}
	flight.waiters++
	e.mu.Unlock()

	select {
	case <-flight.done:
		e.mu.RLock()
		applied, err := e.appliedRevision, e.lastErr
		ready := e.projectionReadyForLocked(targetRevision)
		e.mu.RUnlock()
		if ready {
			return applied, nil
		}
		if err == nil {
			err = fmt.Errorf("%w: target=%d applied=%d", errPolicyRevisionNotReached, targetRevision, applied)
		}
		return applied, err
	case <-ctx.Done():
		e.leaveFlight(flight, ctx.Err())
		e.mu.RLock()
		applied := e.appliedRevision
		e.mu.RUnlock()
		return applied, ctx.Err()
	}
}

func (e *Engine) startFlightLocked(force bool) *reloadFlight {
	sharedCtx, cancel := context.WithCancel(context.Background())
	flight := &reloadFlight{done: make(chan struct{}), ctx: sharedCtx, cancel: cancel, force: force}
	e.flight = flight
	go e.runReloadFlight(flight)
	return flight
}

func (e *Engine) leaveFlight(flight *reloadFlight, waiterErr error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.flight != flight {
		return
	}
	flight.waiters--
	if flight.waiters == 0 {
		e.flight = nil
		e.lastErr = waiterErr
		e.reloadSucceeded = false
		e.metrics.ReloadFailed()
		e.metrics.SetLastStatus(false)
		flight.cancel()
	}
}

func (e *Engine) runReloadFlight(flight *reloadFlight) {
	defer func() {
		flight.cancel()
		close(flight.done)
	}()

	for {
		e.mu.Lock()
		if e.flight != flight {
			e.mu.Unlock()
			return
		}
		target := e.targetRevision
		force := flight.force
		flight.force = false
		e.mu.Unlock()

		policySet, enforcer, err := e.buildEnforcer(flight.ctx, target)
		if err != nil {
			e.recordFlightFailure(flight, err)
			return
		}
		if policySet.Revision < 0 {
			e.recordFlightFailure(flight, fmt.Errorf("casbin policy candidate revision must not be negative: %d", policySet.Revision))
			return
		}
		if policySet.Revision < target {
			e.recordFlightFailure(flight, fmt.Errorf("%w: target=%d candidate=%d", errPolicyRevisionNotReached, target, policySet.Revision))
			return
		}

		if e.applyCandidate(flight, policySet, enforcer, force) {
			return
		}
	}
}

func (e *Engine) applyCandidate(flight *reloadFlight, policySet PolicySet, enforcer *casbinlib.Enforcer, force bool) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.flight != flight {
		return true
	}
	if policySet.Revision < e.targetRevision {
		return false
	}
	// 构造候选期间到达的强制刷新必须再读一次数据库，不能用请求到达前的候选冒充已刷新快照。
	if flight != nil && flight.force {
		return false
	}
	if policySet.Revision > e.appliedRevision || !e.initialized || force {
		e.enforcer = enforcer
		e.appliedRevision = policySet.Revision
		e.initialized = true
	}
	e.lastErr = nil
	e.reloadSucceeded = true
	if e.flight == flight {
		e.flight = nil
	}
	e.metrics.ReloadSucceeded()
	e.metrics.SetLastStatus(true)
	return true
}

func (e *Engine) recordFlightFailure(flight *reloadFlight, err error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.flight != flight {
		return
	}
	e.lastErr = err
	e.reloadSucceeded = false
	if e.flight == flight {
		e.flight = nil
	}
	e.metrics.ReloadFailed()
	e.metrics.SetLastStatus(false)
}

func (e *Engine) failReload(err error) (int64, error) {
	e.recordReloadFailure(err)
	e.mu.RLock()
	applied := e.appliedRevision
	e.mu.RUnlock()
	return applied, err
}

func (e *Engine) recordReloadFailure(err error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.lastErr = err
	e.reloadSucceeded = false
	e.metrics.ReloadFailed()
	e.metrics.SetLastStatus(false)
}

func (e *Engine) projectionReadyForLocked(target int64) bool {
	return e.initialized && e.reloadSucceeded && e.lastErr == nil && e.appliedRevision >= target && e.appliedRevision >= e.targetRevision
}

// ProjectionStatus 返回不可独立写入的 policy 投影状态快照。
func (e *Engine) ProjectionStatus() permissionapplication.PolicyProjectionStatus {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return permissionapplication.PolicyProjectionStatus{
		Initialized:     e.initialized,
		ReloadSucceeded: e.reloadSucceeded,
		AppliedRevision: e.appliedRevision,
		TargetRevision:  e.targetRevision,
		LastError:       e.lastErr,
	}
}

// AppliedRevision 返回当前 enforcer 实际绑定的 policy revision。
func (e *Engine) AppliedRevision() int64 {
	return e.ProjectionStatus().AppliedRevision
}

// LastError 返回最近一次初始化或 reload 失败原因。
func (e *Engine) LastError() error {
	return e.ProjectionStatus().LastError
}

func (e *Engine) buildEnforcer(ctx context.Context, targetRevision int64) (PolicySet, *casbinlib.Enforcer, error) {
	if err := ctx.Err(); err != nil {
		return PolicySet{}, nil, err
	}
	policySet, err := e.loader.LoadPoliciesAtLeast(ctx, targetRevision)
	if err != nil {
		return PolicySet{}, nil, err
	}
	model, err := newModel()
	if err != nil {
		return PolicySet{}, nil, fmt.Errorf("load casbin model: %w", err)
	}
	enforcer, err := casbinlib.NewEnforcer(model)
	if err != nil {
		return PolicySet{}, nil, fmt.Errorf("create casbin enforcer: %w", err)
	}
	for _, rule := range policySet.PermissionRules {
		if _, err := enforcer.AddPolicy(roleSubject(rule.RoleID), rule.PathTemplate, rule.HTTPMethod); err != nil {
			return PolicySet{}, nil, fmt.Errorf("add casbin permission policy: %w", err)
		}
	}
	if err := ctx.Err(); err != nil {
		return PolicySet{}, nil, err
	}
	return policySet, enforcer, nil
}

// InvalidateUserRole 删除指定用户的角色授权缓存。
func (e *Engine) InvalidateUserRole(userID uuid.UUID) {
	if e.userRoles == nil {
		return
	}
	e.userRoles.InvalidateUserRole(userID)
}

// InvalidateAllUserRoles 清空本实例用户角色授权缓存。
func (e *Engine) InvalidateAllUserRoles() {
	if e.userRoles == nil {
		return
	}
	e.userRoles.InvalidateAllUserRoles()
}
