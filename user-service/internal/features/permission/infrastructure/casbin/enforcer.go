package casbin

import (
	"context"
	"fmt"
	"sync"

	casbinlib "github.com/casbin/casbin/v3"
	"github.com/google/uuid"
	"go.uber.org/fx"

	commonmetrics "github.com/aegiscore/common/runtime/observability/metrics"
	commoncasbin "github.com/aegiscore/common/security/casbin"
)

// Engine 使用内存 Casbin enforcer 执行权限判断。
type Engine struct {
	loader    Loader
	metrics   commonmetrics.ReloadMetrics
	userRoles UserRoleResolver

	mu       sync.RWMutex
	enforcer *casbinlib.Enforcer
	lastErr  error
}

// Params 包含 Casbin Engine 所需的 Fx 输入。
type Params struct {
	fx.In

	Loader    Loader
	Metrics   commonmetrics.ReloadMetrics `optional:"true"`
	UserRoles UserRoleResolver
}

// NewEngine 构造 Casbin Engine；初始 policy 加载由 Fx lifecycle 执行。
func NewEngine(params Params) *Engine {
	metrics := params.Metrics
	if metrics == nil {
		metrics = commonmetrics.NopReloadMetrics()
	}
	return &Engine{loader: params.Loader, metrics: metrics, userRoles: params.UserRoles}
}

// RegisterInitialLoad 在 Fx 启动阶段执行初始 policy 加载，失败时保持 fail-closed。
// 初始 reload 失败不会阻断服务启动；Enforce 在 enforcer 或 userRoles 缺失时返回 deny，避免因授权组件未就绪而放行请求。
func RegisterInitialLoad(lc fx.Lifecycle, engine *Engine) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			_ = engine.Reload(ctx)
			return nil
		},
	})
}

// Enforce 基于已加载的内存 policy 判断用户是否允许访问指定路由模板和 HTTP 方法。
// 返回 false,nil 代表安全拒绝而不是系统错误，调用方应按无权限处理；只有上下文、角色解析或 Casbin 执行失败才返回 error。
func (e *Engine) Enforce(ctx context.Context, userID uuid.UUID, pathTemplate string, method string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	e.mu.RLock()
	enforcer := e.enforcer
	e.mu.RUnlock()
	if enforcer == nil {
		return false, nil
	}
	if e.userRoles == nil {
		return false, nil
	}
	roleIDs, err := e.userRoles.RolesForUser(ctx, userID)
	if err != nil {
		return false, err
	}
	for _, roleID := range roleIDs {
		allowed, err := commoncasbin.Enforce(ctx, enforcer, commoncasbin.Request{Subject: roleSubject(roleID), Object: pathTemplate, Action: method})
		if err != nil || allowed {
			return allowed, err
		}
	}
	return false, nil
}

// Reload 全量重建 policy，只有构造成功后才替换当前内存 enforcer。
func (e *Engine) Reload(ctx context.Context) error {
	enforcer, err := e.buildEnforcer(ctx)
	if err != nil {
		e.mu.Lock()
		e.lastErr = err
		e.mu.Unlock()
		e.metrics.ReloadFailed()
		e.metrics.SetLastStatus(false)
		return err
	}
	e.mu.Lock()
	e.enforcer = enforcer
	e.lastErr = nil
	e.mu.Unlock()
	e.metrics.ReloadSucceeded()
	e.metrics.SetLastStatus(true)
	return nil
}

// LastError 返回最近一次初始化或 reload 失败原因。
func (e *Engine) LastError() error {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.lastErr
}

func (e *Engine) buildEnforcer(ctx context.Context) (*casbinlib.Enforcer, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	policySet, err := e.loader.LoadPolicies(ctx)
	if err != nil {
		return nil, err
	}
	model, err := newModel()
	if err != nil {
		return nil, fmt.Errorf("load casbin model: %w", err)
	}
	enforcer, err := casbinlib.NewEnforcer(model)
	if err != nil {
		return nil, fmt.Errorf("create casbin enforcer: %w", err)
	}
	for _, rule := range policySet.PermissionRules {
		if _, err := enforcer.AddPolicy(roleSubject(rule.RoleID), rule.PathTemplate, rule.HTTPMethod); err != nil {
			return nil, fmt.Errorf("add casbin permission policy: %w", err)
		}
	}
	return enforcer, nil
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
