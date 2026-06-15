package casbin

import (
	"context"
	"fmt"
	"sync"

	casbinlib "github.com/casbin/casbin/v2"
	"github.com/google/uuid"
	"go.uber.org/fx"
)

// Engine 使用内存 Casbin enforcer 执行权限判断。
type Engine struct {
	loader Loader

	mu       sync.RWMutex
	enforcer *casbinlib.Enforcer
	lastErr  error
}

// Params 包含 Casbin Engine 所需的 Fx 输入。
type Params struct {
	fx.In

	Loader Loader
}

// NewEngine 构造 Casbin Engine，初始化失败时保持 fail-closed。
func NewEngine(params Params) *Engine {
	engine := &Engine{loader: params.Loader}
	if err := engine.Reload(context.Background()); err != nil {
		engine.mu.Lock()
		engine.lastErr = err
		engine.mu.Unlock()
	}
	return engine
}

// Enforce 基于已加载的内存 policy 判断用户是否允许访问指定路由模板和 HTTP 方法。
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
	return enforcer.Enforce(userSubject(userID), pathTemplate, method)
}

// Reload 全量重建 policy，只有构造成功后才替换当前内存 enforcer。
func (e *Engine) Reload(ctx context.Context) error {
	enforcer, err := e.buildEnforcer(ctx)
	if err != nil {
		e.mu.Lock()
		e.lastErr = err
		e.mu.Unlock()
		return err
	}
	e.mu.Lock()
	e.enforcer = enforcer
	e.lastErr = nil
	e.mu.Unlock()
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
	for _, group := range policySet.GroupingPolicies {
		if _, err := enforcer.AddGroupingPolicy(userSubject(group.UserID), roleSubject(group.RoleID)); err != nil {
			return nil, fmt.Errorf("add casbin grouping policy: %w", err)
		}
	}
	for _, rule := range policySet.PermissionRules {
		if _, err := enforcer.AddPolicy(roleSubject(rule.RoleID), rule.PathTemplate, rule.HTTPMethod); err != nil {
			return nil, fmt.Errorf("add casbin permission policy: %w", err)
		}
	}
	return enforcer, nil
}
