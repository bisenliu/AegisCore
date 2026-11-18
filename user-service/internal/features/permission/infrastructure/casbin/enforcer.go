package casbin

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"uuid"

	casbinlib "github.com/casbin/casbin/v3"

	commoncasbin "github.com/aegiscore/common/security/casbin"
	permissionapplication "github.com/aegiscore/user-service/internal/features/permission/application"
)

var errPolicyRevisionNotReached = errors.New("casbin policy target revision was not reached")

// Engine 在内存中维护权限规则判定引擎，并负责高并发下的接口权限快速裁决。
type Engine struct {
	loader    Loader
	metrics   ReloadMetrics
	userRoles UserRoleResolver

	lifecycleCtx     context.Context
	lifecycleCancel  context.CancelFunc
	lifecycleStarted bool
	lifecycleDone    bool
	mu               sync.RWMutex
	enforcer         *casbinlib.Enforcer
	appliedRevision  int64
	targetRevision   int64
	initialized      bool
	reloadSucceeded  bool
	lastErr          error
	flight           *reloadFlight
}

type reloadFlight struct {
	done    chan struct{}
	ctx     context.Context
	cancel  context.CancelFunc
	waiters int
	force   bool
}

// NewEngine 创建权限引擎对象（需在系统启动阶段显式调用 Initialize 初始化规则模型）。
func NewEngine(loader Loader, metrics ReloadMetrics, userRoles UserRoleResolver) *Engine {
	if metrics == nil {
		metrics = NopReloadMetrics()
	}
	return &Engine{loader: loader, metrics: metrics, userRoles: userRoles}
}

// Enforce 基于内存中的最新权限规则，判定用户是否有权访问指定的接口路径及对应操作。
// 若返回 false, nil 代表用户确实无权访问（属于正常的拒绝访问，不是系统故障）；仅在请求超时被取消、用户角色解析失败或引擎内部执行异常时才会返回非空 error。
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
	// 角色解析可能访问数据库或缓存，不能在此期间持有 enforcer 读锁；解析后会再次检查 readiness。
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

// reloadToRevision 将本地内存中的权限规则更新到指定的最新版本；若加载失败则继续沿用上一份可用的规则，防止权限检查全部瘫痪。
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
	if !e.lifecycleStarted {
		applied := e.appliedRevision
		err := errors.New("casbin engine lifecycle is not started")
		e.lastErr = err
		e.reloadSucceeded = false
		e.metrics.ReloadFailed()
		e.metrics.SetLastStatus(false)
		e.mu.Unlock()
		return applied, err
	}
	flight := e.flight
	if flight == nil {
		flight = e.startFlightLocked(force)
	} else if force {
		// 多个并发的重载请求合并为单次执行；若其中包含强制刷新标记，则确保在当前执行结束前至少再重新读取一次数据库的最新快照。
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

// startFlightLocked 检查当前是否已有正在进行的权限加载；如果有，则合并等待同一批次完成，避免多人并发操作时重复查询数据库。
func (e *Engine) startFlightLocked(force bool) *reloadFlight {
	sharedCtx, cancel := context.WithCancel(e.lifecycleCtx)
	flight := &reloadFlight{done: make(chan struct{}), ctx: sharedCtx, cancel: cancel, force: force}
	e.flight = flight
	go e.runReloadFlight(flight)
	return flight
}

// leaveFlight 结束当前合并的加载流程，由最后离开的协程完成状态清理。
func (e *Engine) leaveFlight(flight *reloadFlight, waiterErr error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.flight != flight {
		return
	}
	flight.waiters--
	if flight.waiters == 0 {
		// 所有调用方都已取消时终止共享 reload，避免后台查询失去任何请求生命周期约束。
		e.flight = nil
		e.lastErr = waiterErr
		e.reloadSucceeded = false
		e.metrics.ReloadFailed()
		e.metrics.SetLastStatus(false)
		flight.cancel()
	}
}

// runReloadFlight 真正执行从数据库读取权限规则的操作，并将加载好的结果共享给所有等待的请求。
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

// applyCandidate 确认加载完成的规则版本未被更新的操作覆盖后，正式替换当前内存中运行的权限规则。
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

// recordFlightFailure 记录合并加载失败，并让所有正在等待的调用方收到相同的错误信息。
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

// failReload 记录权限加载失败并返回错误，同时保留现有可用的旧权限规则，保障系统正常运转。
func (e *Engine) failReload(err error) (int64, error) {
	e.recordReloadFailure(err)
	e.mu.RLock()
	applied := e.appliedRevision
	e.mu.RUnlock()
	return applied, err
}

// recordReloadFailure 记录重载权限规则失败的监控日志。
func (e *Engine) recordReloadFailure(err error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.lastErr = err
	e.reloadSucceeded = false
	e.metrics.ReloadFailed()
	e.metrics.SetLastStatus(false)
}

// projectionReadyForLocked 检查本地内存中的权限版本是否已经达到或超过了目标版本号。
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

// buildEnforcer 根据从数据库读取到的权限规则清单，组装全新的权限判定器；若规则不完整则返回错误。
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
